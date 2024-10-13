package main

import (
	"context"
	"encoding/json"
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
	NodeLabel         = "node.kubernetes.io/capacity"
	DefaultReplicaSet = 1
)

type ScheduleExtender struct {
	clientset *kubernetes.Clientset
}

func (s *ScheduleExtender) Filter(args extenderv1.ExtenderArgs) *extenderv1.ExtenderFilterResult {
	klog.InfoS("begin schedule filter", "pod", args.Pod.Name, "uuid", args.Pod.UID, "namespaces", args.Pod.Namespace)
	pod := args.Pod
	nodes := args.Nodes.Items

	klog.V(3).Infof("Extender Filter called for pod: %s/%s", pod.Namespace, pod.Name)

	var filteredNodes []v1.Node
	workloadReplicas := getWorkloadReplicas(s.clientset, pod)

	for _, node := range nodes {
		if isNodeReady(&node) {
			if workloadReplicas == DefaultReplicaSet {
				if isOnDemandNode(&node) {
					filteredNodes = append(filteredNodes, node)
				}
			} else {
				if isSpotNode(&node) {
					filteredNodes = append(filteredNodes, node)
				}
			}
		}
	}
	klog.InfoS("begin schedule filter", "filteredNodes", filteredNodes, "uuid", args.Pod.UID, "namespaces", args.Pod.Namespace)
	return &extenderv1.ExtenderFilterResult{
		Nodes: &v1.NodeList{Items: filteredNodes},
	}
}

func (s *ScheduleExtender) Prioritize(args extenderv1.ExtenderArgs) (*extenderv1.HostPriorityList, error) {
	klog.InfoS("begin schedule prioritize", "pod", args.Pod.Name, "uuid", args.Pod.UID, "namespaces", args.Pod.Namespace)
	pod := args.Pod
	nodes := args.Nodes.Items

	klog.V(3).Infof("Extender Prioritize called for pod: %s/%s", pod.Namespace, pod.Name)

	workloadReplicas := getWorkloadReplicas(s.clientset, pod)

	var priorityList extenderv1.HostPriorityList
	for _, node := range nodes {
		score := 0
		//Filter on demand node will get high score
		if isOnDemandNode(&node) {
			score += 10
		}
		if workloadReplicas > DefaultReplicaSet {
			score -= getWorkloadPodsOnNode(s.clientset, pod, &node)
		}
		priorityList = append(priorityList, extenderv1.HostPriority{Host: node.Name, Score: int64(score)})
	}
	klog.InfoS("begin schedule Prioritize", "priorityList", priorityList, "uuid", args.Pod.UID, "namespaces", args.Pod.Namespace)
	return &priorityList, nil
}

func getWorkloadReplicas(clientset *kubernetes.Clientset, pod *v1.Pod) int32 {
	for _, ownerRef := range pod.OwnerReferences {
		if ownerRef.Kind == "ReplicaSet" {
			if ownerRef.Controller != nil && *ownerRef.Controller {
				replicaSet, err := clientset.AppsV1().ReplicaSets(pod.Namespace).Get(context.Background(), ownerRef.Name, metav1.GetOptions{})
				if err != nil {
					klog.Errorf("Failed to get ReplicaSet %s/%s: %v", pod.Namespace, ownerRef.Name, err)
					return DefaultReplicaSet
				}
				if replicaSet.OwnerReferences != nil && len(replicaSet.OwnerReferences) > 0 {
					owner := replicaSet.OwnerReferences[0]
					if owner.Kind == "Deployment" {
						deployment, err := clientset.AppsV1().Deployments(pod.Namespace).Get(context.Background(), owner.Name, metav1.GetOptions{})
						if err != nil {
							klog.Errorf("Failed to get Deployment %s/%s: %v", pod.Namespace, owner.Name, err)
							return DefaultReplicaSet
						}
						return *deployment.Spec.Replicas
					}
				}
			}
		} else if ownerRef.Kind == "StatefulSet" {
			statefulSet, err := clientset.AppsV1().StatefulSets(pod.Namespace).Get(context.Background(), ownerRef.Name, metav1.GetOptions{})
			if err != nil {
				klog.Errorf("Failed to get StatefulSet %s/%s: %v", pod.Namespace, ownerRef.Name, err)
				return DefaultReplicaSet
			}
			return *statefulSet.Spec.Replicas
		}
	}
	return DefaultReplicaSet
}

func isOnDemandNode(node *v1.Node) bool {
	_, exists := node.Labels[NodeLabel]
	return exists && node.Labels[NodeLabel] == "on-demand"
}

func isSpotNode(node *v1.Node) bool {
	_, exists := node.Labels[NodeLabel]
	return exists && node.Labels[NodeLabel] == "spot"
}

func isNodeReady(node *v1.Node) bool {
	for _, condition := range node.Status.Conditions {
		if condition.Type == v1.NodeReady && condition.Status == v1.ConditionTrue {
			return true
		}
	}
	return false
}

func isTerminating(node *v1.Node) bool {
	for _, condition := range node.Status.Conditions {
		if condition.Type == v1.NodeReady && condition.Status == v1.ConditionFalse {
			return true
		}
	}
	return false
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
		config, err = clientcmd.BuildConfigFromFlags("", "/etc/kubernetes/scheduler.conf")
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
		klog.Infoln("Into Filter Route outer func")
		var args extenderv1.ExtenderArgs
		if err := json.NewDecoder(r.Body).Decode(&args); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		extenderFilterResult := extender.Filter(args)
		if resultBody, err := json.Marshal(extenderFilterResult); err != nil {
			klog.Errorf("Failed to marshal extenderFilterResult: %+v, %+v",
				err, extenderFilterResult)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(err.Error()))
		} else {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write(resultBody)
		}
	})

	http.HandleFunc("/prioritize", func(w http.ResponseWriter, r *http.Request) {
		klog.Infoln("Into Prioritize Route outer func")
		var args extenderv1.ExtenderArgs
		if err := json.NewDecoder(r.Body).Decode(&args); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		extenderPrioritizeResult, err := extender.Prioritize(args)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		if resultBody, err := json.Marshal(extenderPrioritizeResult); err != nil {
			klog.Errorf("Failed to marshal extenderPrioritizeResult: %+v, %+v",
				err, extenderPrioritizeResult)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(err.Error()))
		} else {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write(resultBody)
		}
	})
	http.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	klog.Info("Extender server started on port 8888")
	if err := http.ListenAndServe(":8888", nil); err != nil {
		klog.Fatalf("Failed to start extender server: %v", err)
	}
}
