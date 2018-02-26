// Modified, using code from 
//https://github.com/kubernetes/client-go/tree/master/examples/out-of-cluster-client-configuration
//as a base
package main

import (
        "fmt"
        //"time"
	"flag"
	"os"
	"path/filepath"

        "k8s.io/api/core/v1"
        metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	// Uncomment the following line to load the gcp plugin (only required to authenticate against GKE clusters).
	// _ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
)

func main() {
	var kubeconfig *string
	if home := homeDir(); home != "" {
		kubeconfig = flag.String("kubeconfig", filepath.Join(home, ".kube", "config"), "(optional) absolute path to the kubeconfig file")
	} else {
		kubeconfig = flag.String("kubeconfig", "", "absolute path to the kubeconfig file")
	}
	flag.Parse()

	// use the current context in kubeconfig
	config, err := clientcmd.BuildConfigFromFlags("", *kubeconfig)
	if err != nil {
		panic(err.Error())
	}

	// create the clientset
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		panic(err.Error())
	}
        namespace := "default"
        //Create a notebook pod.
        pod, err := clientset.CoreV1().Pods(namespace).Create(&v1.Pod{
                ObjectMeta: metav1.ObjectMeta{
                        Name: "jupyter",
                },
                Spec: v1.PodSpec{
                        Containers: []v1.Container {
                                {
                                        Name: "jupyter",
                                        Image: "skippbox/jupyter:0.0.3",
                                        Ports: []v1.ContainerPort{
                                                v1.ContainerPort{
                                                        ContainerPort :8888,
                                                },
                                        },
                                },
                        },
                },
        })
        if err != nil {
                panic(err.Error())
        }
        //set labels for pod
        pod.ObjectMeta.SetLabels(map[string]string{
                "app": "jupyter",
        })
        //what I want to do:
        //pod, err := clientset.CoreV1().Pods(namespace).Update(pod)
        //if err != nil {
                //panic(err.Error())
        //}
        //update the pod
        //for debug
        pod_u, err := clientset.CoreV1().Pods(namespace).Update(pod)
        if err != nil {
                fmt.Println(pod.Status)
                fmt.Println(pod_u.Status)
                panic(err.Error())
        }
        //add a service to expose the pod
        //svc, err := clientset.CoreV1().Services(namespace).Create(&v1.Service{
                //ObjectMeta: metav1.ObjectMeta{
                        //Name: "jupyter-svc",
                //},
                //Spec: v1.ServiceSpec{
                        //Type: v1.ServiceTypeNodePort,
                        //Selector: pod.Labels,
                        //Ports: []v1.ServicePort{
                                //v1.ServicePort{
                                        //Port: 8888,
                                //},
                        //},
                //},
        //})
        //if err != nil {
                //fmt.Println(err)
                //panic(err)
        //}
        //fmt.Println(svc.Spec.Ports[0].NodePort)
        //fmt.Println("Hopefully everything's done now")
}

func homeDir() string {
	if h := os.Getenv("HOME"); h != "" {
		return h
	}
	return os.Getenv("USERPROFILE") // windows
}
