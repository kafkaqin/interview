### Design Document: Distributed Scheduling for Mixed On-Demand and Spot Instances in Kubernetes

#### Objective
The goal is to design a distributed scheduling strategy that maximizes the use of spot instances in a Kubernetes cluster while minimizing service interruptions. This strategy should work within a single cluster that uses both on-demand and spot instances, without modifying the existing Kubernetes scheduler.

#### Background
Spot instances are cost-effective but come with the risk of termination with short notice. To leverage their cost benefits while maintaining service reliability, we need a strategy that intelligently schedules workloads across both spot and on-demand instances.

#### Design Strategy

1. **Node Labeling**:
    - On-demand nodes are labeled with `node.kubernetes.io/capacity: on-demand`.
    - Spot nodes are labeled with `node.kubernetes.io/capacity: spot`.

2. **Workload Categorization**:
    - **Critical Workloads**: These require high availability and should primarily run on on-demand instances. They can be represented as Deployments or StatefulSets with a higher priority.
    - **Non-Critical Workloads**: These can tolerate interruptions and should be scheduled on spot instances whenever possible.

3. **Taint and Tolerations**:
    - Apply taints to spot nodes to prevent critical workloads from being scheduled on them unless they have specific tolerations.
    - Example Taint: `spot=true:NoSchedule`
    - Critical workloads should not have tolerations for this taint, ensuring they are scheduled on on-demand nodes.

4. **Affinity and Anti-Affinity Rules**:
    - Use node affinity rules to prefer scheduling non-critical workloads on spot instances.
    - Example Affinity Rule for Non-Critical Workloads:
      ```yaml
      affinity:
        nodeAffinity:
          preferredDuringSchedulingIgnoredDuringExecution:
          - weight: 1
            preference:
              matchExpressions:
              - key: node.kubernetes.io/capacity
                operator: In
                values:
                - spot
      ```

5. **Pod Disruption Budgets (PDBs)**:
    - Define PDBs for critical workloads to ensure a minimum number of replicas are always running, even during node terminations.

6. **Custom Controller**:
    - Implement a lightweight custom controller that monitors node terminations and reschedules non-critical workloads from spot to on-demand nodes if necessary.

#### Implementation Steps

1. **Label Nodes**:
    - Ensure all nodes are labeled correctly as either on-demand or spot.

2. **Define Taints and Tolerations**:
    - Apply taints to spot nodes.
    - Ensure critical workloads do not have tolerations for these taints.

3. **Set Affinity Rules**:
    - Define node affinity rules for non-critical workloads to prefer spot nodes.

4. **Configure Pod Disruption Budgets**:
    - Set up PDBs for critical workloads to maintain availability.

5. **Develop Custom Controller**:
    - Create a custom controller using a Kubernetes client library (e.g., client-go).
    - The controller should watch for node termination events and reschedule affected pods as needed.

#### Example Implementation

Below is a simplified example of how you might implement the affinity rules and taints in a Deployment manifest:

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: non-critical-app
spec:
  replicas: 3
  selector:
    matchLabels:
      app: non-critical-app
  template:
    metadata:
      labels:
        app: non-critical-app
    spec:
      affinity:
        nodeAffinity:
          preferredDuringSchedulingIgnoredDuringExecution:
          - weight: 1
            preference:
              matchExpressions:
              - key: node.kubernetes.io/capacity
                operator: In
                values:
                - spot
      tolerations:
      - key: "spot"
        operator: "Equal"
        value: "true"
        effect: "NoSchedule"
      containers:
      - name: app-container
        image: myapp:latest
```

#### Conclusion

This design leverages Kubernetes' native features like taints, tolerations, and affinity rules to implement a distributed scheduling strategy that maximizes the use of spot instances while ensuring critical workloads remain stable. The addition of a custom controller provides a safety net for handling spot instance terminations, ensuring workloads are rescheduled appropriately. This approach minimizes changes to the existing Kubernetes scheduler and requires minimal additional components.


===================
To continue with the implementation and further elaborate on the design, let's delve into the specifics of the custom controller and how it can be integrated into the Kubernetes cluster to handle spot instance terminations effectively.

### Custom Controller Design

#### Objective
The custom controller's primary role is to monitor the Kubernetes cluster for spot instance termination events and ensure that non-critical workloads are rescheduled onto available nodes, preferably on-demand nodes, to maintain service continuity.

#### Key Components

1. **Event Monitoring**:
   - The controller should watch for node status changes, particularly focusing on spot instances. This can be achieved by monitoring the `Node` resource for conditions indicating termination.

2. **Pod Rescheduling Logic**:
   - Upon detecting a termination event, the controller should identify all pods running on the affected spot node and attempt to reschedule them onto other nodes.
   - Prioritize rescheduling onto other spot nodes if available, or fallback to on-demand nodes if necessary.

3. **Graceful Handling**:
   - Ensure that any rescheduling is done gracefully, respecting Pod Disruption Budgets (PDBs) and maintaining application availability.

#### Implementation Steps

1. **Set Up the Development Environment**:
   - Use the Kubernetes client-go library to interact with the Kubernetes API.
   - Set up a Go project with the necessary dependencies.

2. **Implement the Controller Logic**:
   - Create a controller that watches for `Node` events.
   - Filter events to focus on spot nodes and detect termination signals.

3. **Reschedule Pods**:
   - Upon detecting a termination event, list all pods scheduled on the affected node.
   - Use the Kubernetes API to delete these pods, allowing the default scheduler to reschedule them according to the defined affinity and taint rules.

4. **Deploy the Controller**:
   - Package the controller as a containerized application.
   - Deploy it to the Kubernetes cluster, ensuring it has the necessary permissions to watch nodes and manage pods.

#### Example Code Snippet

Below is a simplified example of how the controller might be implemented in Go:

```go
package main

import (
    "context"
    "fmt"
    "log"
    "time"

    v1 "k8s.io/api/core/v1"
    metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
    "k8s.io/client-go/kubernetes"
    "k8s.io/client-go/tools/cache"
    "k8s.io/client-go/tools/clientcmd"
)

func main() {
    config, err := clientcmd.BuildConfigFromFlags("", "/path/to/kubeconfig")
    if err != nil {
        log.Fatalf("Error building kubeconfig: %s", err.Error())
    }

    clientset, err := kubernetes.NewForConfig(config)
    if err != nil {
        log.Fatalf("Error creating Kubernetes client: %s", err.Error())
    }

    nodeInformer := cache.NewSharedInformer(
        cache.NewListWatchFromClient(clientset.CoreV1().RESTClient(), "nodes", metav1.NamespaceAll, nil),
        &v1.Node{},
        0,
    )

    nodeInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
        UpdateFunc: func(oldObj, newObj interface{}) {
            oldNode := oldObj.(*v1.Node)
            newNode := newObj.(*v1.Node)

            if isSpotNode(newNode) && isTerminating(newNode) {
                fmt.Printf("Spot node %s is terminating\n", newNode.Name)
                handleNodeTermination(clientset, newNode)
            }
        },
    })

    stopCh := make(chan struct{})
    defer close(stopCh)
    go nodeInformer.Run(stopCh)

    select {}
}

func isSpotNode(node *v1.Node) bool {
    _, exists := node.Labels["node.kubernetes.io/capacity"]
    return exists && node.Labels["node.kubernetes.io/capacity"] == "spot"
}

func isTerminating(node *v1.Node) bool {
    for _, condition := range node.Status.Conditions {
        if condition.Type == v1.NodeReady && condition.Status == v1.ConditionFalse {
            return true
        }
    }
    return false
}

func handleNodeTermination(clientset *kubernetes.Clientset, node *v1.Node) {
    pods, err := clientset.CoreV1().Pods(metav1.NamespaceAll).List(context.TODO(), metav1.ListOptions{
        FieldSelector: fmt.Sprintf("spec.nodeName=%s", node.Name),
    })
    if err != nil {
        log.Printf("Error listing pods on node %s: %s", node.Name, err.Error())
        return
    }

    for _, pod := range pods.Items {
        err := clientset.CoreV1().Pods(pod.Namespace).Delete(context.TODO(), pod.Name, metav1.DeleteOptions{})
        if err != nil {
            log.Printf("Error deleting pod %s/%s: %s", pod.Namespace, pod.Name, err.Error())
        } else {
            log.Printf("Deleted pod %s/%s for rescheduling", pod.Namespace, pod.Name)
        }
    }
}
```

### Conclusion

This design and implementation provide a robust framework for leveraging spot instances in a Kubernetes cluster while minimizing the risk of service interruptions. By using Kubernetes' native features and a custom controller, we can achieve cost savings without compromising reliability. This approach ensures that workloads are dynamically and intelligently scheduled across available resources, adapting to changes in node availability.


## 为什么不使用scheduler 扩展或者plugin 

Using a scheduler extender is indeed a viable approach for implementing custom scheduling logic in Kubernetes, especially when you want to influence the scheduling decisions without replacing the default scheduler. Here's why you might consider using a scheduler extender for the scenario described:

### Advantages of Using a Scheduler Extender

1. **Seamless Integration**:
   - Scheduler extenders allow you to integrate custom scheduling logic with the existing Kubernetes scheduler. This means you can add your custom logic without having to replace or heavily modify the default scheduler.

2. **Fine-Grained Control**:
   - Extenders provide fine-grained control over the scheduling process. You can influence both the filtering and prioritization phases of scheduling, allowing you to implement complex scheduling policies.

3. **Dynamic Decision Making**:
   - With a scheduler extender, you can make dynamic scheduling decisions based on real-time cluster state, such as preferring spot instances for non-critical workloads while ensuring critical workloads are scheduled on on-demand instances.

4. **Minimal Cluster Changes**:
   - Using an extender requires minimal changes to the cluster setup. You don't need to deploy additional controllers or modify existing workloads, as the extender works alongside the default scheduler.

5. **Scalability**:
   - Scheduler extenders can scale with the cluster, as they are invoked only when scheduling decisions are made, reducing the overhead compared to continuously running controllers.

### Implementing a Scheduler Extender

To implement a scheduler extender, you would typically follow these steps:

1. **Develop the Extender Service**:
   - Create a service that implements the extender API. This service will handle HTTP requests from the Kubernetes scheduler for filtering and prioritizing nodes.

2. **Configure the Scheduler**:
   - Modify the scheduler configuration to include the extender. This involves specifying the extender's URL and the actions it should handle (e.g., filter, prioritize).

3. **Implement Filtering and Prioritization Logic**:
   - In the extender service, implement logic to filter out nodes that do not meet the scheduling criteria (e.g., spot nodes for critical workloads).
   - Implement prioritization logic to rank nodes based on custom criteria, such as preferring spot nodes for non-critical workloads.

4. **Deploy the Extender**:
   - Deploy the extender service in the Kubernetes cluster, ensuring it is accessible to the scheduler.

### Example Use Case

For the scenario of maximizing spot instance usage, the extender could:

- **Filter Phase**: Exclude spot nodes for critical workloads unless they have specific tolerations.
- **Prioritize Phase**: Rank spot nodes higher for non-critical workloads to maximize their usage.

### Conclusion

Using a scheduler extender is a powerful way to implement custom scheduling policies in Kubernetes. It allows you to enhance the default scheduling behavior without replacing the scheduler, providing a flexible and efficient solution for complex scheduling requirements. In the context of maximizing spot instance usage, an extender can dynamically adjust scheduling decisions based on workload criticality and node availability, ensuring cost efficiency and reliability.


#===========

Certainly! Let's explore both a Scheduler Extender and a Scheduler Plugin example in Kubernetes. These examples will illustrate how you can customize scheduling behavior to meet specific requirements, such as maximizing the use of spot instances.

### Scheduler Extender Example

A Scheduler Extender is an external service that the Kubernetes scheduler can call to influence scheduling decisions. It can be used to filter nodes and prioritize them based on custom logic.

#### Steps to Implement a Scheduler Extender

1. **Develop the Extender Service**:
   - Create a web service that implements the extender API. This service will handle HTTP requests for filtering and prioritizing nodes.

2. **Configure the Scheduler**:
   - Modify the scheduler configuration to include the extender. This involves specifying the extender's URL and the actions it should handle.

3. **Implement Filtering and Prioritization Logic**:
   - In the extender service, implement logic to filter and prioritize nodes.

#### Example Code for a Scheduler Extender

Here's a simple example using Python and Flask to create a scheduler extender:

```python
from flask import Flask, request, jsonify

app = Flask(__name__)

@app.route('/filter', methods=['POST'])
def filter_nodes():
    data = request.get_json()
    nodes = data['nodes']['items']
    filtered_nodes = [node for node in nodes if 'spot' in node['metadata']['labels']]
    return jsonify({'nodes': {'items': filtered_nodes}})

@app.route('/prioritize', methods=['POST'])
def prioritize_nodes():
    data = request.get_json()
    nodes = data['nodes']['items']
    scores = [{'name': node['metadata']['name'], 'score': 10} for node in nodes]
    return jsonify({'scores': scores})

if __name__ == '__main__':
    app.run(host='0.0.0.0', port=8000)
```

#### Scheduler Configuration

Modify the scheduler configuration to include the extender:

```yaml
apiVersion: kubescheduler.config.k8s.io/v1
kind: KubeSchedulerConfiguration
extenders:
  - urlPrefix: "http://<extender-service>:8000"
    filterVerb: "filter"
    prioritizeVerb: "prioritize"
    weight: 1
    nodeCacheCapable: false
```

### Scheduler Plugin Example

A Scheduler Plugin is a more integrated way to extend the Kubernetes scheduler, introduced with the scheduling framework. It allows you to write custom plugins that can participate in various scheduling phases.

#### Steps to Implement a Scheduler Plugin

1. **Develop the Plugin**:
   - Write a Go plugin that implements one or more scheduling phases (e.g., Filter, Score).

2. **Build and Deploy the Scheduler**:
   - Build a custom scheduler binary that includes your plugin.
   - Deploy this custom scheduler in your Kubernetes cluster.

#### Example Code for a Scheduler Plugin

Here's a simple example of a custom scheduler plugin in Go:

```go
package main

import (
    "context"
    "fmt"
    "k8s.io/kubernetes/pkg/scheduler/framework"
)

type SpotNodePlugin struct{}

func (p *SpotNodePlugin) Name() string {
    return "SpotNodePlugin"
}

func (p *SpotNodePlugin) Filter(ctx context.Context, state *framework.CycleState, pod *v1.Pod, nodeInfo *framework.NodeInfo) *framework.Status {
    if _, exists := nodeInfo.Node().Labels["spot"]; exists {
        return framework.NewStatus(framework.Success)
    }
    return framework.NewStatus(framework.Unschedulable, "Node is not a spot instance")
}

func (p *SpotNodePlugin) Score(ctx context.Context, state *framework.CycleState, pod *v1.Pod, nodeName string) (int64, *framework.Status) {
    return 10, framework.NewStatus(framework.Success)
}

func (p *SpotNodePlugin) ScoreExtensions() framework.ScoreExtensions {
    return nil
}

func NewSpotNodePlugin(_ *runtime.Unknown, _ framework.Handle) (framework.Plugin, error) {
    return &SpotNodePlugin{}, nil
}

func main() {
    fmt.Println("This is a custom scheduler plugin example.")
}
```

#### Register the Plugin

Modify the scheduler configuration to register your plugin:

```yaml
apiVersion: kubescheduler.config.k8s.io/v1
kind: KubeSchedulerConfiguration
profiles:
  - schedulerName: default-scheduler
    plugins:
      filter:
        enabled:
          - name: SpotNodePlugin
      score:
        enabled:
          - name: SpotNodePlugin
```

### Conclusion

Both Scheduler Extenders and Plugins offer powerful ways to customize scheduling behavior in Kubernetes. Extenders are easier to implement and deploy as separate services, while Plugins provide deeper integration with the scheduling process. The choice between them depends on your specific requirements and the level of integration you need.



=================
Implementing a Scheduler Extender in Go involves creating a web service that the Kubernetes scheduler can call to filter and prioritize nodes. Below is a step-by-step guide to creating a simple Scheduler Extender using Go and the `net/http` package.

### Step-by-Step Implementation

#### 1. Set Up Your Go Environment

Ensure you have Go installed and set up your workspace. You can create a new directory for your project:

```bash
mkdir scheduler-extender
cd scheduler-extender
```

#### 2. Create a Go Module

Initialize a new Go module:

```bash
go mod init scheduler-extender
```

#### 3. Implement the Scheduler Extender

Create a file named `main.go` and implement the HTTP server with endpoints for filtering and prioritizing nodes.

```go
package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
)

type ExtenderArgs struct {
	Nodes struct {
		Items []Node `json:"items"`
	} `json:"nodes"`
}

type Node struct {
	Metadata struct {
		Name   string            `json:"name"`
		Labels map[string]string `json:"labels"`
	} `json:"metadata"`
}

type ExtenderFilterResult struct {
	Nodes struct {
		Items []Node `json:"items"`
	} `json:"nodes"`
}

type ExtenderPrioritizeResult struct {
	Scores []HostPriority `json:"scores"`
}

type HostPriority struct {
	Host  string `json:"host"`
	Score int    `json:"score"`
}

func filterHandler(w http.ResponseWriter, r *http.Request) {
	var args ExtenderArgs
	if err := json.NewDecoder(r.Body).Decode(&args); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	var filteredNodes []Node
	for _, node := range args.Nodes.Items {
		if _, exists := node.Metadata.Labels["spot"]; exists {
			filteredNodes = append(filteredNodes, node)
		}
	}

	result := ExtenderFilterResult{}
	result.Nodes.Items = filteredNodes

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

func prioritizeHandler(w http.ResponseWriter, r *http.Request) {
	var args ExtenderArgs
	if err := json.NewDecoder(r.Body).Decode(&args); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	var scores []HostPriority
	for _, node := range args.Nodes.Items {
		score := 0
		if _, exists := node.Metadata.Labels["spot"]; exists {
			score = 10
		}
		scores = append(scores, HostPriority{Host: node.Metadata.Name, Score: score})
	}

	result := ExtenderPrioritizeResult{Scores: scores}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

func main() {
	http.HandleFunc("/filter", filterHandler)
	http.HandleFunc("/prioritize", prioritizeHandler)

	fmt.Println("Starting scheduler extender on port 8000...")
	log.Fatal(http.ListenAndServe(":8000", nil))
}
```

#### 4. Build and Run the Extender

Build the Go application:

```bash
go build -o scheduler-extender
```

Run the extender:

```bash
./scheduler-extender
```

#### 5. Configure the Kubernetes Scheduler

Modify the Kubernetes scheduler configuration to include the extender. This involves specifying the extender's URL and the actions it should handle.

```yaml
apiVersion: kubescheduler.config.k8s.io/v1
kind: KubeSchedulerConfiguration
extenders:
  - urlPrefix: "http://<extender-service>:8000"
    filterVerb: "filter"
    prioritizeVerb: "prioritize"
    weight: 1
    nodeCacheCapable: false
```

Replace `<extender-service>` with the appropriate service address where your extender is running.

### Conclusion

This Go-based Scheduler Extender provides a simple example of how to filter and prioritize nodes based on custom logic. In this case, it filters nodes with the "spot" label and assigns them a higher score. You can expand this logic to suit more complex scheduling requirements. Deploy the extender as a service in your Kubernetes cluster and configure the scheduler to use it for enhanced scheduling decisions.

## ==== 


To implement a scheduling strategy where workloads have a preference for spot nodes but can fall back to on-demand nodes when spot nodes are unavailable, you can use a combination of Kubernetes features such as node affinity, taints and tolerations, and a custom scheduler extender or plugin. Here's a detailed approach to achieve this:

### Step-by-Step Implementation

#### 1. Node Affinity Configuration

Node affinity allows you to specify rules about which nodes your pods can be scheduled on. You can set a preferred affinity for spot nodes and a required affinity for on-demand nodes as a fallback.

**Pod Spec with Node Affinity:**

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: my-app
spec:
  replicas: 3
  selector:
    matchLabels:
      app: my-app
  template:
    metadata:
      labels:
        app: my-app
    spec:
      affinity:
        nodeAffinity:
          preferredDuringSchedulingIgnoredDuringExecution:
            - weight: 100
              preference:
                matchExpressions:
                  - key: node.kubernetes.io/capacity
                    operator: In
                    values:
                      - spot
          requiredDuringSchedulingIgnoredDuringExecution:
            nodeSelectorTerms:
              - matchExpressions:
                  - key: node.kubernetes.io/capacity
                    operator: In
                    values:
                      - on-demand
      containers:
        - name: my-container
          image: my-image:latest
```

In this configuration:
- **Preferred Affinity**: The pod prefers to be scheduled on nodes labeled as `spot`.
- **Required Affinity**: If no spot nodes are available, the pod can be scheduled on nodes labeled as `on-demand`.

#### 2. Taints and Tolerations

To ensure that critical workloads do not end up on spot nodes, you can use taints and tolerations. Spot nodes can be tainted to prevent critical workloads from being scheduled on them unless they have the appropriate tolerations.

**Taint Spot Nodes:**

```bash
kubectl taint nodes <spot-node-name> spot=true:NoSchedule
```

**Pod Spec with Tolerations:**

```yaml
tolerations:
  - key: "spot"
    operator: "Equal"
    value: "true"
    effect: "NoSchedule"
```

This toleration allows non-critical workloads to be scheduled on spot nodes.

#### 3. Scheduler Extender or Plugin

To automate the fallback mechanism when spot nodes are unavailable, you can implement a scheduler extender or plugin. This component can dynamically adjust scheduling decisions based on node availability.

**Scheduler Extender Logic:**

- **Filter Phase**: Exclude spot nodes for critical workloads unless they have specific tolerations.
- **Prioritize Phase**: Rank spot nodes higher for non-critical workloads to maximize their usage.

**Example Extender Logic in Go:**

```go
func prioritizeHandler(w http.ResponseWriter, r *http.Request) {
    var args ExtenderArgs
    if err := json.NewDecoder(r.Body).Decode(&args); err != nil {
        http.Error(w, err.Error(), http.StatusBadRequest)
        return
    }

    var scores []HostPriority
    for _, node := range args.Nodes.Items {
        score := 0
        if _, exists := node.Metadata.Labels["spot"]; exists {
            score = 10
        } else if _, exists := node.Metadata.Labels["on-demand"]; exists {
            score = 5
        }
        scores = append(scores, HostPriority{Host: node.Metadata.Name, Score: score})
    }

    result := ExtenderPrioritizeResult{Scores: scores}

    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(result)
}
```

### Conclusion

This approach leverages Kubernetes' native features to implement a scheduling strategy that prefers spot nodes but can fall back to on-demand nodes when necessary. By using node affinity, taints and tolerations, and potentially a custom scheduler extender, you can ensure that workloads are scheduled efficiently and cost-effectively, with minimal service disruption. This setup allows for dynamic adaptation to changes in node availability, ensuring that workloads remain running even if spot nodes are terminated.




###
To implement a custom controller that manages the scheduling of workloads with a preference for spot nodes and a fallback to on-demand nodes, we can use the Kubernetes client-go library. This custom controller will monitor the cluster for changes in node availability and reschedule pods as necessary.

### Step-by-Step Implementation

#### 1. Set Up Your Go Environment

Ensure you have Go installed and set up your workspace. Create a new directory for your project:

```bash
mkdir custom-controller
cd custom-controller
```

#### 2. Create a Go Module

Initialize a new Go module:

```bash
go mod init custom-controller
```

#### 3. Implement the Custom Controller

Create a file named `main.go` and implement the custom controller logic. The controller will watch for node events and reschedule pods when spot nodes become unavailable.

```go
package main

import (
	"context"
	"fmt"
	"log"
	"time"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/retry"
)

func main() {
	config, err := clientcmd.BuildConfigFromFlags("", "/path/to/kubeconfig")
	if err != nil {
		log.Fatalf("Error building kubeconfig: %s", err.Error())
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		log.Fatalf("Error creating Kubernetes client: %s", err.Error())
	}

	nodeInformer := cache.NewSharedInformer(
		cache.NewListWatchFromClient(clientset.CoreV1().RESTClient(), "nodes", metav1.NamespaceAll, nil),
		&v1.Node{},
		0,
	)

	nodeInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		UpdateFunc: func(oldObj, newObj interface{}) {
			oldNode := oldObj.(*v1.Node)
			newNode := newObj.(*v1.Node)

			if isSpotNode(oldNode) && !isNodeReady(newNode) {
				fmt.Printf("Spot node %s is not ready\n", newNode.Name)
				handleNodeUnavailability(clientset, newNode)
			}
		},
	})

	stopCh := make(chan struct{})
	defer close(stopCh)
	go nodeInformer.Run(stopCh)

	select {}
}

func isSpotNode(node *v1.Node) bool {
	_, exists := node.Labels["node.kubernetes.io/capacity"]
	return exists && node.Labels["node.kubernetes.io/capacity"] == "spot"
}

func isNodeReady(node *v1.Node) bool {
	for _, condition := range node.Status.Conditions {
		if condition.Type == v1.NodeReady && condition.Status == v1.ConditionTrue {
			return true
		}
	}
	return false
}

func handleNodeUnavailability(clientset *kubernetes.Clientset, node *v1.Node) {
	pods, err := clientset.CoreV1().Pods(metav1.NamespaceAll).List(context.TODO(), metav1.ListOptions{
		FieldSelector: fmt.Sprintf("spec.nodeName=%s", node.Name),
	})
	if err != nil {
		log.Printf("Error listing pods on node %s: %s", node.Name, err.Error())
		return
	}

	for _, pod := range pods.Items {
		err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
			pod.Spec.NodeName = ""
			_, updateErr := clientset.CoreV1().Pods(pod.Namespace).Update(context.TODO(), &pod, metav1.UpdateOptions{})
			return updateErr
		})
		if err != nil {
			log.Printf("Error rescheduling pod %s/%s: %s", pod.Namespace, pod.Name, err.Error())
		} else {
			log.Printf("Rescheduled pod %s/%s", pod.Namespace, pod.Name)
		}
	}
}
```

### Explanation

- **Node Monitoring**: The controller uses a shared informer to watch for updates to nodes. It checks if a spot node becomes unavailable (not ready).
- **Pod Rescheduling**: When a spot node is detected as unavailable, the controller lists all pods on that node and clears their `NodeName` field, allowing the default scheduler to reschedule them.
- **Fallback to On-Demand Nodes**: The default scheduler will use the node affinity rules defined in the pod spec to reschedule pods onto on-demand nodes if no spot nodes are available.

### Conclusion

This custom controller provides a mechanism to handle the unavailability of spot nodes by rescheduling affected pods onto available nodes, preferably on-demand nodes. This approach ensures that workloads remain running even if spot nodes are terminated, leveraging Kubernetes' native scheduling capabilities to achieve the desired behavior.

```
root@k8s-node:~# kubectl get nodes -owide 
NAME           STATUS   ROLES           AGE    VERSION    INTERNAL-IP     EXTERNAL-IP   OS-IMAGE             KERNEL-VERSION      CONTAINER-RUNTIME
k8s-node       Ready    control-plane   20d    v1.28.14   192.168.1.102   <none>        Ubuntu 20.04.6 LTS   5.4.0-196-generic   containerd://1.7.12
k8s-worker     Ready    <none>          2d1h   v1.28.14   192.168.1.103   <none>        Ubuntu 20.04.6 LTS   5.4.0-196-generic   containerd://1.7.12
k8s-worker-2   Ready    <none>          41h    v1.28.14   192.168.1.105   <none>        Ubuntu 20.04.6 LTS   5.4.0-196-generic   containerd://1.7.12

```