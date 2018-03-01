// Modified, using code from 
//https://github.com/kubernetes/client-go/tree/master/examples/out-of-cluster-client-configuration
//as a base, and attempting to recreate running 'kubectl create -f' on 
//https://raw.githubusercontent.com/kubernetes-incubator/client-python/master/examples/notebooks/docker/jupyter.yml
package main

import (
        "fmt"
        //"time"
	"flag"
	"os"
	"path/filepath"
        "k8s.io/api/core/v1"
        metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
        intstr "k8s.io/apimachinery/pkg/util/intstr"
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
        pod_name := "jupyter"
        //Create a notebook pod.
        _, err = clientset.CoreV1().Pods(namespace).Create(&v1.Pod{
                ObjectMeta: metav1.ObjectMeta{
                        Name: pod_name,
                },
                Spec: v1.PodSpec{
                        Volumes: []v1.Volume{
                                {
                                        Name: "notebook-volume",
                                        VolumeSource: v1.VolumeSource{
                                                GitRepo: &v1.GitRepoVolumeSource{
                                                        Repository: "https://github.com/kubernetes-client/python.git",
                                                },
                                        },
                                },
                        },
                        Containers: []v1.Container {
                                {
                                        Name: "jupyter",
                                        Image: "skippbox/jupyter:0.0.3",
                                        VolumeMounts: []v1.VolumeMount{
                                                {
                                                        Name: "notebook-volume",
                                                        MountPath: "/root",
                                                },
                                        },
                                        Ports: []v1.ContainerPort{
                                                {
                                                        Name: "http",
                                                        Protocol: v1.ProtocolTCP,
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
                result, getErr := clientset.CoreV1().Pods(namespace).Get(pod_name, metav1.GetOptions{})
                //Panic if getting pod fails
                if getErr != nil {
                        panic(fmt.Errorf("Failed to get latest version of pod: %v", getErr))
                }
                //set labels for pod
                result.ObjectMeta.SetLabels(map[string]string{
                        "app": "jupyter",
                })
                //update the pod, and return the updated pod or error
                _, updateErr := clientset.CoreV1().Pods(namespace).Update(result)
                return updateErr
        })
        if retryErr != nil {
                panic(fmt.Errorf("Update failed: %v", retryErr))
        }
        pod, getErr := clientset.CoreV1().Pods(namespace).Get(pod_name, metav1.GetOptions{})
        //Panic if getting pod fails
        if getErr != nil {
                panic(fmt.Errorf("Failed to get latest version of pod: %v", getErr))
        }
        fmt.Println("Updated Labels on pod")
        //add a service to expose the pod
        svc, err := clientset.CoreV1().Services(namespace).Create(&v1.Service{
                ObjectMeta: metav1.ObjectMeta{
                        Name: "jupyter-svc",
                        Labels: map[string]string{
                                "app": "jupyter",
                        },
                },
                Spec: v1.ServiceSpec{
                        Type: v1.ServiceTypeLoadBalancer,
                        Selector: pod.Labels,
                        Ports: []v1.ServicePort{
                                        {
                                        Port: 80,
                                        Name: "http",
                                        TargetPort: intstr.IntOrString{
                                                IntVal: 8888,
                                        },
                                },
                        },
                },
        })
        if err != nil {
                fmt.Println(err)
                panic(err)
        }
        fmt.Println(svc.Spec.Ports[0].NodePort)
        fmt.Println("Hopefully everything's done now")
}

func homeDir() string {
	if h := os.Getenv("HOME"); h != "" {
		return h
	}
	return os.Getenv("USERPROFILE") // windows
}
