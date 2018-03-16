package main

import (
	"bufio"
	//"errors"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/docker/docker/pkg/namesgenerator"
	appsv1beta1 "k8s.io/api/apps/v1beta1"
	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/remotecommand"
	"k8s.io/client-go/util/homedir"
	"k8s.io/client-go/util/retry"
	// Uncomment the following line to load the gcp plugin (only required to authenticate against GKE clusters).
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
)

type Writer struct {
	Str []string
}

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

func main() {
	//Obtain cluster authentication information from users home directory, or fall back to user input.
	var kubeconfig *string
	if home := homedir.HomeDir(); home != "" {
		kubeconfig = flag.String("kubeconfig", filepath.Join(home, ".kube", "config"), "(optional) absolute path to the kubeconfig file")
	} else {
		kubeconfig = flag.String("kubeconfig", "", "absolute path to the kubeconfig file")
	}
	flag.Parse()

	config, err := clientcmd.BuildConfigFromFlags("", *kubeconfig)
	if err != nil {
		panic(err)
	}
	//Create authenticated clientset
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		panic(err)
	}
	//Create a unique namespace
	namespaceClient := clientset.CoreV1().Namespaces()
	newNamespace := strings.Replace(namesgenerator.GetRandomName(0), "_", "-", -1) + "-wr"
	//Retry if namespace taken
	retryErr := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		_, nsErr := namespaceClient.Create(&apiv1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: newNamespace,
			},
		})
		if nsErr != nil {
			fmt.Printf("Failed to create new namespace, %s. Trying again. Error: %v", newNamespace, err)
			newNamespace = strings.Replace(namesgenerator.GetRandomName(0), "_", "-", -1) + "-wr"
		}
		return nsErr
	})
	if retryErr != nil {
		panic(fmt.Errorf("Creatioin of namespace failed: %v", retryErr))
	}

	//Create clientset for deployments that is authenticated against the given cluster. Use default namsespace.
	deploymentsClient := clientset.AppsV1beta1().Deployments(newNamespace)

	//Create new wr deployment
	deployment := &appsv1beta1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name: "wr-manager",
		},
		Spec: appsv1beta1.DeploymentSpec{
			Replicas: int32Ptr(1),
			Template: apiv1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Name: "wr-manager",
					Labels: map[string]string{
						"app": "wr-manager",
					},
				},
				Spec: apiv1.PodSpec{
					Volumes: []apiv1.Volume{
						{
							Name: "wr-temp",
							VolumeSource: apiv1.VolumeSource{
								EmptyDir: &apiv1.EmptyDirVolumeSource{},
							},
						},
					},
					Containers: []apiv1.Container{
						{
							Name:  "wr-manager",
							Image: "ubuntu:17.10",
							Ports: []apiv1.ContainerPort{
								{
									Name:          "wr-manager",
									Protocol:      apiv1.ProtocolTCP,
									ContainerPort: 1021,
								},
								{
									Name:          "wr-web",
									Protocol:      apiv1.ProtocolTCP,
									ContainerPort: 1022,
								},
							},
							Command: []string{
								"tail",
							},
							Args: []string{
								"-f",
								"/dev/null",
							},
							VolumeMounts: []apiv1.VolumeMount{
								{
									Name:      "wr-temp",
									MountPath: "/wr-tmp",
								},
							},
						},
					},
					InitContainers: []apiv1.Container{
						{
							Name:      "init-container",
							Image:     "ubuntu:17.10",
							Command:   []string{"/bin/dd", "of=/wr-tmp/wr", "bs=1024"},
							Stdin:     true,
							StdinOnce: true,
							VolumeMounts: []apiv1.VolumeMount{
								{
									Name:      "wr-temp",
									MountPath: "/wr-tmp",
								},
							},
						},
					},
					Hostname: "wr-manager",
				},
			},
		},
	}

	// Create Deployment
	fmt.Println("Creating deployment...")
	result, err := deploymentsClient.Create(deployment)
	if err != nil {
		panic(err)
	}
	fmt.Printf("Created deployment %q in namespace %v.\n", result.GetObjectMeta().GetName(), newNamespace)

	//Copy WR to pod, selecting by label.
	//Wait for the pod to be created, then return it
	var podList *apiv1.PodList
	getPodErr := wait.ExponentialBackoff(retry.DefaultRetry, func() (done bool, err error) {
		var getErr error
		podList, getErr = clientset.CoreV1().Pods(newNamespace).List(metav1.ListOptions{
			LabelSelector: "app=wr-manager",
		})
		switch {
		case getErr != nil:
			panic(fmt.Errorf("Failed to list pods in namespace %v \n", newNamespace))
		case len(podList.Items) == 0:
			return false, nil
		case len(podList.Items) > 0:
			return true, nil
		default:
			return false, err
		}
	})
	if getPodErr != nil {
		panic(fmt.Errorf("Failed to list pods, error: %v\n", getPodErr))

	}

	//Get the current working directory.
	dir, err := os.Getwd()
	if err != nil {
		panic(err)
	}

	//Read binary
	reader, writer := io.Pipe()
	//Read file from disk
	dat, err := ioutil.ReadFile(dir + "/wr")
	if err != nil {
		panic(fmt.Errorf("Failed to read binary: %v", err))
	}
	go func() { //avoid deadlock
		defer writer.Close()
		writer.Write(dat) //Write data to pipe
	}()

	//Copy the wr binary to the running pod
	fmt.Println("Sleeping for 15s") // wait for container to be running
	time.Sleep(15 * time.Second)
	fmt.Println("Woken up")
	pod := podList.Items[0]
	fmt.Printf("Container for pod is %v\n", pod.Spec.InitContainers[0].Name)
	fmt.Println(pod.Spec.InitContainers)
	fmt.Printf("Pod has name %v, in namespace %v\n", pod.ObjectMeta.Name, pod.ObjectMeta.Namespace)
	//command := []string{"/bin/dd", "of=/wr-tmp/wr", "bs=1024"} //Open a bash terminal on the pod

	//Make a request to the APIServer for an 'exec'.
	//Open Stdin, Stdout and Stderr for use by the client
	execRequest := clientset.CoreV1().RESTClient().Post().
		Resource("pods").
		Name(pod.ObjectMeta.Name).
		Namespace(pod.ObjectMeta.Namespace).
		SubResource("attach")
	execRequest.VersionedParams(&apiv1.PodExecOptions{
		Container: pod.Spec.InitContainers[0].Name,
		//Command:   command,
		Stdin:  true,
		Stdout: false,
		Stderr: true,
		TTY:    false,
	}, scheme.ParameterCodec)

	//Create an executor to send commands / recieve output.
	//SPDY Allows multiplexed bidirectional streams to and from  the pod
	exec, err := remotecommand.NewSPDYExecutor(config, "POST", execRequest.URL())
	if err != nil {
		panic(fmt.Errorf("Error creating SPDYExecutor: %v", err))
	}
	fmt.Println("Created SPDYExecutor")

	//stdIn := newStringReader([]string{input})
	stdIn := reader
	stdOut := new(Writer)
	stdErr := new(Writer)

	//Execute the command, with Std(in,out,err) pointing to the
	//above readers and writers
	fmt.Println("Executing remotecommand")
	err = exec.Stream(remotecommand.StreamOptions{
		Stdin: stdIn,
		//Stdout: stdOut,
		Stderr: stdErr,
		Tty:    false,
	})
	if err != nil {
		//fmt.Printf("Stdin: %v\n", stdIn)
		fmt.Printf("StdOut: %v\n", stdOut)
		fmt.Printf("StdErr: %v\n", stdErr)
		panic(fmt.Errorf("Error executing remote command: %v", err))
	}
}

func prompt() {
	fmt.Printf("-> Press Return key to continue.")
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		break
	}
	if err := scanner.Err(); err != nil {
		panic(err)
	}
	fmt.Println()
}

func int32Ptr(i int32) *int32 { return &i }
