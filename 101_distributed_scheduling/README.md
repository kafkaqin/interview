# Distributed Scheduling

Please write a design document about distributed scheduling and implementation of it.

Evaluation aspects:
- Learning ability
- Depth of thinking on single-point issues
- Hands-on ability

## Background

In all cloud providers, like AWS, Google, and others, there are many spot instances. They are quite cheap (10% of the on-demand instances' price), but after you buy them, they could be terminated with only two minutes' notice in advance (in most scenarios, we don't set PDB, and we should perform the graceful drain).

So, I want you to design a strategy to maximize the use of spot instances without causing service interruptions, instead of relying solely on on-demand instances, to cut costs, by using distributed scheduling in a single cluster (on-demand/spot mixed or other methods for one workload). This is important because all spot instances being terminated at the same time could cause interruptions for different kinds of workloads (single replica workload, multiple replica workload).

Also, I don't want to change the scheduler already used in the K8s cluster and want to ensure the minimal components necessary in the cluster.

Notes:
> 1. On demand nodes has label: node.kubernetes.io/capacity: on-demand.
> 2. Spot node has label: node.kubernetes.io/capacity: spot.
> 3. Workloads represented as Deployments and StatefulSets.
> 4. on-demand/spot instance represented as K8s nodes in the cluster.
> 5. Only focus on scheduling control; the graceful drain after receiving the terminal notification is handled by other components.

## Output

- A rough design document (English)
- An easy implementation (After talking to the interviewer)

Kube-scheduler extenders do support a `prioritize` API. This API allows the extender to influence the scoring phase of the scheduling process, where nodes are ranked based on their suitability for running a given pod.

### Scheduler Extender APIs

1. **Filter**: This API is used to filter out nodes that are not suitable for scheduling a pod. The extender can return a subset of nodes that it considers feasible.

2. **Prioritize**: This API is used to assign scores to nodes. The scheduler uses these scores to rank nodes. The scores are typically in the range of 0 to 100, where a higher score indicates a more preferred node.

3. **Bind**: This API allows the extender to take over the binding process, which is the final step of assigning a pod to a node.

4. **Preemption**: This API is used to handle preemption logic, allowing the extender to suggest which pods should be preempted to make room for a new pod.

### Example of Prioritize API

Here is a simple example of how you might implement the `prioritize` API in a scheduler extender:

```go
package main

import (
	"encoding/json"
	"log"
	"net/http"

	v1 "k8s.io/api/core/v1"
	schedulerapi "k8s.io/kube-scheduler/extender/v1"
)

func main() {
	http.HandleFunc("/prioritize", prioritizeHandler)
	log.Fatal(http.ListenAndServe(":8080", nil))
}

func prioritizeHandler(w http.ResponseWriter, r *http.Request) {
	var args schedulerapi.ExtenderArgs
	if err := json.NewDecoder(r.Body).Decode(&args); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	nodeScores := prioritizeNodes(args.Pod, args.Nodes.Items)

	response := schedulerapi.HostPriorityList(nodeScores)

	if err := json.NewEncoder(w).Encode(response); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func prioritizeNodes(pod *v1.Pod, nodes []v1.Node) []schedulerapi.HostPriority {
	var hostPriorityList []schedulerapi.HostPriority

	for _, node := range nodes {
		score := calculateNodeScore(pod, &node)
		hostPriorityList = append(hostPriorityList, schedulerapi.HostPriority{
			Host:  node.Name,
			Score: score,
		})
	}

	return hostPriorityList
}

func calculateNodeScore(pod *v1.Pod, node *v1.Node) int64 {
	// Example scoring logic: Prefer on-demand nodes for single replica workloads
	if isSingleReplicaWorkload(pod) && node.Labels["node.kubernetes.io/capacity"] == "on-demand" {
		return 100
	}

	// Prefer spot nodes for multiple replica workloads
	if isMultipleReplicaWorkload(pod) && node.Labels["node.kubernetes.io/capacity"] == "spot" {
		return 80
	}

	// Default score
	return 50
}

func isSingleReplicaWorkload(pod *v1.Pod) bool {
	// Example logic: Check if the pod belongs to a single replica deployment
	return pod.Labels["workload-type"] == "single-replica"
}

func isMultipleReplicaWorkload(pod *v1.Pod) bool {
	// Example logic: Check if the pod belongs to a multiple replica deployment
	return pod.Labels["workload-type"] == "multiple-replica"
}
```

### Deployment Steps

1. **Build and Deploy**: Compile the Go program and create a Docker image. Deploy the extender as a service in your Kubernetes cluster.

2. **Configure Scheduler**: Update the scheduler configuration to include the extender by modifying the `scheduler-config.yaml` to point to the extender service.

3. **Test**: Deploy workloads with appropriate labels and observe the scheduling decisions to ensure they align with the prioritization strategy.

This example demonstrates how to implement a basic prioritize API in a scheduler extender, allowing you to influence node selection based on custom logic.



In Kubernetes scheduling, the process of assigning a pod to a node involves several steps, and when using a scheduler extender, the process is slightly extended to include additional custom logic. Here is the typical execution order of the scheduling process with an extender:

1. **Pre-filtering (Optional)**:
    - This step involves any pre-filtering logic that might be applied before the main filtering phase. This is not part of the extender API but can be part of custom logic within the scheduler.

2. **Filtering**:
    - The default scheduler filters out nodes that do not meet the basic requirements of the pod (e.g., resource requests, node selectors, taints, and tolerations).
    - The extender's `filter` API is called, allowing it to further filter the list of nodes based on custom criteria. The extender can return a subset of nodes that it considers feasible for the pod.

3. **Prioritization**:
    - The default scheduler scores the remaining nodes based on various built-in priorities (e.g., least requested resources, balanced resource allocation).
    - The extender's `prioritize` API is called, allowing it to assign custom scores to the nodes. These scores are combined with the default scheduler's scores to rank the nodes.

4. **Scoring**:
    - The scheduler combines scores from its own prioritization logic and the extender's prioritize API to determine the most suitable node for the pod.

5. **Preemption (Optional)**:
    - If no suitable nodes are found, the scheduler may consider preempting lower-priority pods to make room for the new pod.
    - The extender's `preemption` API can be used to influence which pods should be preempted.

6. **Binding**:
    - The scheduler selects the highest-ranked node and binds the pod to it.
    - If the extender implements the `bind` API, it can take over the binding process, allowing for custom binding logic.

7. **Post-binding (Optional)**:
    - Any post-binding operations can be performed, though this is not part of the extender API.

### Summary

The execution order with a scheduler extender is as follows:

1. **Filtering**: Default scheduler filtering → Extender `filter` API
2. **Prioritization**: Default scheduler prioritization → Extender `prioritize` API
3. **Scoring**: Combine scores from default scheduler and extender
4. **Preemption**: (if needed) Default preemption logic → Extender `preemption` API
5. **Binding**: Default binding logic → Extender `bind` API (if implemented)

This sequence allows the extender to influence both the filtering and prioritization phases, providing a mechanism to implement custom scheduling logic while still leveraging the default scheduler's capabilities.