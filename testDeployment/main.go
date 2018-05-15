package main

import (
	"flag"
	"github.com/VertebrateResequencing/wr/kubernetes/client"
	"github.com/VertebrateResequencing/wr/kubernetes/deployment"
	"github.com/sevlyar/go-daemon"
	"log"
	"os"
	"syscall"
)

var (
	signal = flag.String("s", "", `send signal to the port forwarding daemon
		quit -- graceful shutdown 
		stop -- fast shutdown`)
)

var err error
var stopCh chan struct{}

func StartController(stopCh chan struct{}) {
	// Always authenticate the client lib with cluster
	c := deployment.Controller{
		Client: &client.Kubernetesp{},
	}
	c.Clientset, c.Restconfig, err = c.Client.Authenticate() // Authenticate and populate Kubernetesp with clientset and restconfig.
	if err != nil {
		panic(err)
	}
	err = c.Client.Initialize(c.Clientset) // Populate the rest of Kubernetesp
	if err != nil {
		panic(err)
	}
	log.Println("Authenticated and Initialised!")
	log.Println("====================")

	// Create a ConfigMap
	err = c.Client.CreateInitScriptConfigMap("test", "/Users/tb15/go/src/projects/learning_client_go/testDeployment/test.sh")
	if err != nil {
		panic(err)
	}
	// Set up the parameters for the deployment
	// AttachCmdOpts gets populated by controller when pod is created.
	dir, err := os.Getwd()
	log.Println(dir)
	if err != nil {
		panic(err)
	}
	c.Opts = &deployment.DeployOpts{
		ContainerImage: "ubuntu:latest",
		TempMountPath:  "/wr-tmp",
		Files: []client.FilePair{
			{"/Users/tb15/go/src/projects/learning_client_go/testDeployment/wr-linux", "/wr-tmp/"},
		},
		BinaryPath:      "/scripts/test.sh",
		BinaryArgs:      []string{"/wr-tmp/wr-linux", "manager", "start", "-f"},
		ConfigMapName:   "test",
		ConfigMountPath: "/scripts",
		RequiredPorts:   []int{1120, 1121},
	}

	defer close(stopCh)
	log.Printf("\n\n")
	log.Println("====================")
	log.Printf("\n\n")
	log.Println("Controller started :)")

	c.Run(stopCh)

	return
}

func main() {
	flag.Parse()
	daemon.AddCommand(daemon.StringFlag(signal, "quit"), syscall.SIGQUIT, termHandler)
	daemon.AddCommand(daemon.StringFlag(signal, "stop"), syscall.SIGTERM, termHandler)

	cntxt := daemon.Context{
		PidFileName: "pfwpid",
		PidFilePerm: 0644,
		LogFileName: "pfwlog",
		LogFilePerm: 0640,
		WorkDir:     "/",
		Umask:       027,
		Args:        []string{""},
	}

	// Daemon currently running
	if len(daemon.ActiveFlags()) > 0 {
		d, err := cntxt.Search()
		if err != nil {
			log.Fatalln("Unable to send signal to daemon: ", err)
		}
		daemon.SendCommands(d)
		return
	}

	// Check if forward flag is set
	d, err := cntxt.Reborn()
	if err != nil {
		log.Fatalln(err)
	}
	if d != nil {
		return
	}
	defer cntxt.Release()

	log.Println("======================")
	log.Println("daemon started")
	stopCh = make(chan struct{})
	StartController(stopCh)

}

func termHandler(sig os.Signal) error {
	log.Println("terminating portforward....")
	close(stopCh)
	return daemon.ErrStop
}
