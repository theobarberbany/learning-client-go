package main

import (
	"fmt"
	"github.com/VertebrateResequencing/wr/kubernetes/client"
	"github.com/VertebrateResequencing/wr/kubernetes/deployment"
	"os"
)

func main() {
	// Authenticate the client lib with cluster
	var err error
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
	fmt.Println("Authenticated and Initialised!")
	fmt.Println("====================")
	fmt.Printf("\n\n")
	// Create a ConfigMap
	_, err = c.Client.NewConfigMap(&client.ConfigMapOpts{
		Name: "test",
		Data: map[string]string{"test.sh": "echo \"testing 1234\""},
	})
	if err != nil {
		panic(err)
	}

	// Set up the parameters for the deployment
	// AttachCmdOpts gets populated by controller when pod is created.
	fmt.Println("Populating opts")
	dir, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	c.Opts = &deployment.DeployOpts{
		ContainerImage: "ubuntu:latest",
		TempMountPath:  "/wr-tmp",
		Files: []client.FilePair{
			{dir + "/wr-linux", "/wr-tmp/"},
		},
		BinaryPath:      "/wr-tmp/wr-linux",
		BinaryArgs:      []string{"manager", "start"},
		ConfigMapName:   "test",
		ConfigMountPath: "/scripts",
		RequiredPorts:   []int{1120, 1121},
	}

	stopCh := make(chan struct{})
	defer close(stopCh)
	fmt.Printf("\n\n")
	fmt.Println("====================")
	fmt.Printf("\n\n")
	fmt.Println("Controller started :)")

	c.Run(stopCh)
}
