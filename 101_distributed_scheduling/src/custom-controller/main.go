package main

import (
	"encoding/json"
	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog"
	schedulerapi "k8s.io/kubernetes/pkg/scheduler/apis/extender/v1"
	"kubernetes/staging/src/k8s.io/client-go/rest"
	"kubernetes/staging/src/k8s.io/client-go/tools/clientcmd"
	"net/http"
)

const (
	OnDemandNodeLabel = "node.kubernetes.io/capacity=on-demand"
	SpotNodeLabel     = "node.kubernetes.io/capacity=spot"
)

type ScheduleExtender struct {
	clientset *kubernetes.Clientset
}

func getWorkloadReplicas(pod *v1.Pod) int32 {
	for _, ownerRef := range pod.OwnerReferences {
		if ownerRef.Kind == "ReplicaSet" {
			if ownerRef.Controller != nil && *ownerRef.Controller {
				// Pod is controlled by a Deployment
				deployment, err := clientset.AppsV1().Deployments(pod.Namespace).Get(ownerRef.Name, metav1.GetOptions{})
				if err != nil {
					klog.Errorf("Failed to get Deployment %s/%s: %v", pod.Namespace, ownerRef.Name, err)
					return 1
				}
				return *deployment.Spec.Replicas
			}
		} else if ownerRef.Kind == "StatefulSet" {
			// Pod is controlled by a StatefulSet
			statefulSet, err := clientset.AppsV1().StatefulSets(pod.Namespace).Get(ownerRef.Name, metav1.GetOptions{})
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
	pods, err := clientset.CoreV1().Pods(pod.Namespace).List(metav1.ListOptions{
		FieldSelector: "spec.nodeName=" + node.Name,
		LabelSelector: workloadSelector.String(),
	})
	if err != nil {
		klog.Errorf("Failed to get Pods on node %s: %v", node.Name, err)
		return 0
	}
	return len(pods.Items)
}

func (s *ScheduleExtender) Filter(args schedulerapi.ExtenderArgs) *schedulerapi.ExtenderFilterResult {
	pod := args.Pod
	nodes := args.Nodes.Items

	klog.V(3).Infof("Extender Filter called for pod: %s/%s", pod.Namespace, pod.Name)

	var filteredNodes []*v1.Node
	workloadReplicas := getWorkloadReplicas(s.clientset, pod)

	for _, node := range nodes {
		if workloadReplicas == 1 {
			if isOnDemandNode(&node) {
				filteredNodes = append(filteredNodes, &node)
			}
		} else {
			filteredNodes = append(filteredNodes, &node)
		}
	}

	return &schedulerapi.ExtenderFilterResult{
		Nodes: &v1.NodeList{Items: filteredNodes},
	}
}

func (s *ScheduleExtender) Prioritize(args schedulerapi.ExtenderArgs) (*schedulerapi.HostPriorityList, error) {
	pod := args.Pod
	nodes := args.Nodes.Items

	klog.V(3).Infof("Extender Prioritize called for pod: %s/%s", pod.Namespace, pod.Name)

	workloadReplicas := getWorkloadReplicas(s.clientset, pod)

	var priorityList schedulerapi.HostPriorityList
	for _, node := range nodes {
		score := 0
		if isOnDemandNode(&node) {
			score += 10
		}
		if workloadReplicas > 1 {
			score -= getWorkloadPodsOnNode(s.clientset, pod, &node)
		}
		priorityList = append(priorityList, schedulerapi.HostPriority{Host: node.Name, Score: score})
	}

	return &priorityList, nil
}

func main() {
	config, err := rest.InClusterConfig()
	if err != nil {
		config, err = clientcmd.BuildConfigFromFlags("", "")
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
		var args schedulerapi.ExtenderArgs
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
		var args schedulerapi.ExtenderArgs
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
