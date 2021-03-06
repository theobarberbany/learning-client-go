//Example usage of remotecommand library.
//Found here: https://github.com/appscode/searchlight/blob/22632646424bdd34c98bdaec87553fd182a85945/plugins/check_pod_exec/lib.go#L62
package check_pod_exec

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/remotecommand"
	utilexec "k8s.io/client-go/util/exec"
)

type Writer struct {
	Str []string
}

//Construct a string from an array of bytes.
//Store in struct Writer
func (w *Writer) Write(p []byte) (n int, err error) {
	str := string(p)
	if len(str) > 0 {
		w.Str = append(w.Str, str)
	}
	return len(str), nil
}

func newStringReader(ss []string) io.Reader {
	formattedString := strings.Join(ss, "\n")
	reader := strings.NewReader(formattedString)
	return reader
}

//Create clientset
func CheckKubeExec(req *Request) (icinga.State, interface{}) {
	config, err := clientcmd.BuildConfigFromFlags(req.masterURL, req.kubeconfigPath)
	if err != nil {
		return icinga.UNKNOWN, err
	}
	kubeClient := kubernetes.NewForConfigOrDie(config)

	pod, err := kubeClient.CoreV1().Pods(req.Namespace).Get(req.Pod, metav1.GetOptions{})
	if err != nil {
		return icinga.UNKNOWN, err
	}

	if req.Container != "" {
		notFound := true
		for _, container := range pod.Spec.Containers {
			if container.Name == req.Container {
				notFound = false
				break
			}
		}
		if notFound {
			return icinga.UNKNOWN, fmt.Sprintf(`Container "%v" not found`, req.Container)
		}
	}

	//Create a POST to the API asking for stdin to a specific container
	execRequest := kubeClient.CoreV1().RESTClient().Post().
		Resource("pods").
		Name(req.Pod).
		Namespace(req.Namespace).
		SubResource("exec").
		Param("container", req.Container).
		Param("command", req.Command).
		Param("stdin", "true").
		Param("stdout", "false").
		Param("stderr", "false").
		Param("tty", "false")
	//Use remotecommand to create a SPDY executor with the clientset(config) and
	//URL returned from the execRequest. SPDYExecutor provides bidirectional multiplexed
	//streams to / from pod
	exec, err := remotecommand.NewSPDYExecutor(config, "POST", execRequest.URL())
	if err != nil {
		return icinga.UNKNOWN, err
	}

	//Set stdIn to the reader function above, and out and err to the outputs
	//of the writers
	stdIn := newStringReader([]string{"-c", req.Arg})
	stdOut := new(Writer)
	stdErr := new(Writer)
	//Use remotecommand to execute the command built above.
	err = exec.Stream(remotecommand.StreamOptions{
		Stdin:  stdIn,
		Stdout: stdOut,
		Stderr: stdErr,
		Tty:    false, //Don't request a terminal
	})

	var exitCode int
	if err == nil {
		exitCode = 0
	} else {
		if exitErr, ok := err.(utilexec.ExitError); ok && exitErr.Exited() {
			exitCode = exitErr.ExitStatus()
		} else {
			return icinga.UNKNOWN, "Failed to find exit code."
		}
	}

	output := fmt.Sprintf("Exit Code: %v", exitCode)
	if exitCode != 0 {
		exitCode = 2
	}

	return icinga.State(exitCode), output
}

//Request struct, used above
type Request struct {
	masterURL      string
	kubeconfigPath string

	Pod       string
	Container string
	Namespace string
	Command   string
	Arg       string
}

func NewCmd() *cobra.Command {
	var req Request
	var icingaHost string
	c := &cobra.Command{
		Use:     "check_pod_exec",
		Short:   "Check exit code of exec command on Kubernetes container",
		Example: "",

		Run: func(cmd *cobra.Command, args []string) {
			flags.EnsureRequiredFlags(cmd, "host", "arg")

			host, err := icinga.ParseHost(icingaHost)
			if err != nil {
				fmt.Fprintln(os.Stdout, icinga.WARNING, "Invalid icinga host.name")
				os.Exit(3)
			}
			if host.Type != icinga.TypePod {
				fmt.Fprintln(os.Stdout, icinga.WARNING, "Invalid icinga host type")
				os.Exit(3)
			}
			req.Namespace = host.AlertNamespace
			req.Pod = host.ObjectName
			icinga.Output(CheckKubeExec(&req))
		},
	}

	c.Flags().StringVar(&req.masterURL, "master", req.masterURL, "The address of the Kubernetes API server (overrides any value in kubeconfig)")
	c.Flags().StringVar(&req.kubeconfigPath, "kubeconfig", req.kubeconfigPath, "Path to kubeconfig file with authorization information (the master location is set by the master flag).")
	c.Flags().StringVarP(&icingaHost, "host", "H", "", "Icinga host name")
	c.Flags().StringVarP(&req.Container, "container", "C", "", "Container name in specified pod")
	c.Flags().StringVarP(&req.Command, "cmd", "c", "/bin/sh", "Exec command. [Default: /bin/sh]")
	c.Flags().StringVarP(&req.Arg, "argv", "a", "", "Arguments for exec command. [Format: 'arg; arg; arg']")
	return c
}
