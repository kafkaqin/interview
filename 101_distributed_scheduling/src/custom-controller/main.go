package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	klog "k8s.io/klog/v2"
	extenderv1 "k8s.io/kube-scheduler/extender/v1"
)

const (
	OnDemandNodeLabel = "node.kubernetes.io/capacity=on-demand"
	SpotNodeLabel     = "node.kubernetes.io/capacity=spot"
)

type ScheduleExtender struct {
	clientset *kubernetes.Clientset
}

func (s *ScheduleExtender) Filter(args extenderv1.ExtenderArgs) *extenderv1.ExtenderFilterResult {
	pod := args.Pod
	nodes := args.Nodes.Items

	klog.V(3).Infof("Extender Filter called for pod: %s/%s", pod.Namespace, pod.Name)

	var filteredNodes []v1.Node
	workloadReplicas := getWorkloadReplicas(s.clientset, pod)

	for _, node := range nodes {
		if workloadReplicas == 1 {
			if isOnDemandNode(&node) {
				filteredNodes = append(filteredNodes, node)
			}
		} else {
			filteredNodes = append(filteredNodes, node)
		}
	}

	return &extenderv1.ExtenderFilterResult{
		Nodes: &v1.NodeList{Items: filteredNodes},
	}
}

func (s *ScheduleExtender) Prioritize(args extenderv1.ExtenderArgs) (*extenderv1.HostPriorityList, error) {
	pod := args.Pod
	nodes := args.Nodes.Items

	klog.V(3).Infof("Extender Prioritize called for pod: %s/%s", pod.Namespace, pod.Name)

	workloadReplicas := getWorkloadReplicas(s.clientset, pod)

	var priorityList extenderv1.HostPriorityList
	for _, node := range nodes {
		score := 0
		if isOnDemandNode(&node) {
			score += 10
		}
		if workloadReplicas > 1 {
			score -= getWorkloadPodsOnNode(s.clientset, pod, &node)
		}
		priorityList = append(priorityList, extenderv1.HostPriority{Host: node.Name, Score: int64(score)})
	}

	return &priorityList, nil
}

func getWorkloadReplicas(clientset *kubernetes.Clientset, pod *v1.Pod) int32 {
	for _, ownerRef := range pod.OwnerReferences {
		if ownerRef.Kind == "ReplicaSet" {
			if ownerRef.Controller != nil && *ownerRef.Controller {
				replicaSet, err := clientset.AppsV1().ReplicaSets(pod.Namespace).Get(context.Background(), ownerRef.Name, metav1.GetOptions{})
				if err != nil {
					klog.Errorf("Failed to get ReplicaSet %s/%s: %v", pod.Namespace, ownerRef.Name, err)
					return 1
				}
				if replicaSet.OwnerReferences != nil && len(replicaSet.OwnerReferences) > 0 {
					owner := replicaSet.OwnerReferences[0]
					if owner.Kind == "Deployment" {
						deployment, err := clientset.AppsV1().Deployments(pod.Namespace).Get(context.Background(), owner.Name, metav1.GetOptions{})
						if err != nil {
							klog.Errorf("Failed to get Deployment %s/%s: %v", pod.Namespace, owner.Name, err)
							return 1
						}
						return *deployment.Spec.Replicas
					}
				}
			}
		} else if ownerRef.Kind == "StatefulSet" {
			statefulSet, err := clientset.AppsV1().StatefulSets(pod.Namespace).Get(context.Background(), ownerRef.Name, metav1.GetOptions{})
			if err != nil {
				klog.Errorf("Failed to get StatefulSet %s/%s: %v", pod.Namespace, ownerRef.Name, err)
				return 1
			}
			return *statefulSet.Spec.Replicas
		}
	}
	return 1
}

func isOnDemandNode(node *v1.Node) bool {
	_, ok := node.Labels[OnDemandNodeLabel]
	return ok
}

func getWorkloadPodsOnNode(clientset *kubernetes.Clientset, pod *v1.Pod, node *v1.Node) int {
	workloadSelector := labels.SelectorFromSet(pod.Labels)
	pods, err := clientset.CoreV1().Pods(pod.Namespace).List(context.Background(), metav1.ListOptions{
		FieldSelector: "spec.nodeName=" + node.Name,
		LabelSelector: workloadSelector.String(),
	})
	if err != nil {
		klog.Errorf("Failed to get Pods on node %s: %v", node.Name, err)
		return 0
	}
	return len(pods.Items)
}

func main() {
	config, err := rest.InClusterConfig()
	if err != nil {
		config, err = clientcmd.BuildConfigFromFlags("", "D:\\kubernetes\\kube-scheduler\\kube-schduler.conf")
		if err != nil {
			klog.Fatalf("Failed to get kubeconfig: %v", err)
		}
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		klog.Fatalf("Failed to create clientset: %v", err)
	}

	extender := &ScheduleExtender{
		clientset: clientset,
	}

	http.HandleFunc("/filter", func(w http.ResponseWriter, r *http.Request) {
		fmt.Println("=================filter:")
		var args extenderv1.ExtenderArgs
		if err := json.NewDecoder(r.Body).Decode(&args); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		result := extender.Filter(args)
		if err := json.NewEncoder(w).Encode(result); err != nil {
			klog.Errorf("Failed to encode filter result: %v", err)
		}
	})

	http.HandleFunc("/prioritize", func(w http.ResponseWriter, r *http.Request) {
		fmt.Println("=================prioritize:")
		var args extenderv1.ExtenderArgs
		if err := json.NewDecoder(r.Body).Decode(&args); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		result, err := extender.Prioritize(args)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		if err := json.NewEncoder(w).Encode(result); err != nil {
			klog.Errorf("Failed to encode prioritize result: %v", err)
		}
	})

	klog.Info("Extender server started on port 8888")
	if err := http.ListenAndServe(":8888", nil); err != nil {
		klog.Fatalf("Failed to start extender server: %v", err)
	}
}

//apiVersion: kubescheduler.config.k8s.io/v1
//kind: KubeSchedulerConfiguration
//clientConnection:
//kubeconfig: "/etc/kubernetes/scheduler.conf"
//extenders:
//- urlPrefix: "http://your-extender-service:8080"
//filterVerb: "filter"
//prioritizeVerb: "prioritize"
//weight: 1
//enableHTTPS: false
//nodeCacheCapable: false
//managedResources:
//- name: "example.com/custom-resource"
//ignoredByScheduler: true
//ignorable: true
