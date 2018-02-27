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
        "k8s.io/client-go/util/retry"
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
        //Get latest version of the pod
        retryErr := retry.RetryOnConflict(retry.DefaultRetry, func() error {
                //should avoid conflicts
                //exponential backoff
                result, getErr := clientset.CoreV1().Pods(namespace).Get(pod, metav1.GetOptions{})
                //Panic if getting pod fails
                if getErr != nil {
                        panic(fmt.Errorf("Failed to get latest version of deployment: %v", getErr))
                }
                //set labels for pod
                result.ObjectMeta.SetLabels(map[string]string{
                        "app": "jupyter",
                })
                //update the pod
                _, updateErr := clientset.CoreV1().Pods(namespace).Update(pod)
                return updateErr
        })
        if retryErr != nil {
                panic(fmt.Errorf("Update failed: %v", retryErr))
        }
        fmt.Println("Updated Labels on pod")
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
