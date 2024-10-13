
### Design Document: Distributed Scheduling with Scheduler Extender

#### Objective
The goal is to design a distributed scheduling strategy that maximizes the use of spot instances without causing service interruptions. This involves using a scheduler extender to make informed decisions about where to place workloads, considering the volatility of spot instances.

#### Key Considerations
1. **Node Classification**:
    - **Spot Nodes**: Labeled with `node.kubernetes.io/capacity: spot`.
    - **On-Demand Nodes**: Labeled with `node.kubernetes.io/capacity: on-demand`.

2. **Workload Characteristics**:
    - **Single Replica Workloads**: More sensitive to interruptions; prefer on-demand nodes.
    - **Multiple Replica Workloads**: Can tolerate some level of interruption; prefer spot nodes to reduce costs.

3. **Scheduling Strategy**:
    - Prioritize scheduling multiple replica workloads on spot nodes.
    - Schedule single replica workloads on on-demand nodes to ensure stability.
    - Implement a fallback mechanism to reschedule workloads from spot to on-demand nodes if spot nodes are terminated.

4. **Extender Implementation**:
    - The extender will intercept scheduling decisions and apply custom logic to prioritize node selection based on the above strategy.
    - It will communicate with the Kubernetes API to get node and pod information and make scheduling decisions accordingly.

#### Implementation

Below is a basic implementation of a Kubernetes scheduler extender using Go. This implementation assumes you have a basic understanding of Kubernetes and Go programming.

[Please refer to the file for the code implementation.](./main.go)

### Deployment

1. **Build and Deploy the Extender**:
    - Compile the Go program and create a Docker image(Note that Docker needs to be installed.).
    ```
    git clone https://github.com/kafkaqin/interview.git;
    cd interview/101_distributed_scheduling/src/custom-controller
    docker build -t spot-scheduler-extender:v1 .
    ```
    - Deploy the extender as a service in your Kubernetes cluster.
    ```shell
    kubectl apply -f kube-scheduler/deployment.yaml
    ```

2. **Configure the Scheduler**:
    - Modify the scheduler configuration to include the extender by updating the `scheduler-config.yaml` to point to the extender service.

3. **Testing**:
    3.1 ***Cluster environment information***
    - The cluster was deployed using kubeadm. The detailed cluster information is as follows:***
    ```
      root@k8s-node:~# kubectl get nodes -owide 
      NAME           STATUS   ROLES           AGE    VERSION    INTERNAL-IP     EXTERNAL-IP   OS-IMAGE             KERNEL-VERSION      CONTAINER-RUNTIME
      k8s-node       Ready    control-plane   20d    v1.28.14   192.168.1.102   <none>        Ubuntu 20.04.6 LTS   5.4.0-196-generic   containerd://1.7.12
      k8s-worker     Ready    <none>          2d1h   v1.28.14   192.168.1.103   <none>        Ubuntu 20.04.6 LTS   5.4.0-196-generic   containerd://1.7.12
      k8s-worker-2   Ready    <none>          41h    v1.28.14   192.168.1.105   <none>        Ubuntu 20.04.6 LTS   5.4.0-196-generic   containerd://1.7.12
    ```
    3.2 ***Modify the kube-scheduler configuration file***
     In a Kubernetes cluster deployed using kubeadm, the default kube-scheduler runs as a static Pod. Its manifest file is typically located in the /etc/kubernetes/manifests directory on the control plane node. To configure kube-scheduler to use a custom scheduler extender, follow these steps:
    - 3.2.1. Create a new scheduler configuration file on the control plane node, such as /etc/kubernetes/kube-scheduler-config.yaml. The content of the file is as follows:
    ```yaml
  apiVersion: kubescheduler.config.k8s.io/v1
  kind: KubeSchedulerConfiguration
  extenders:
    - urlPrefix: "http://spot-scheduler-extender.kube-system.svc.cluster.local"
      filterVerb: "filter"
      prioritizeVerb: "prioritize"
      weight: 1
      enableHTTPS: false
      nodeCacheCapable: false
      ignorable: true
  clientConnection:
    kubeconfig: /etc/kubernetes/scheduler.conf
   ```
  Make sure this file is saved at /etc/kubernetes/kube-scheduler-config.yaml.
   - 3.2.2. Modify the kube-scheduler Static Pod Configuration:
     Edit the `/etc/kubernetes/manifests/kube-scheduler.yaml` file, adding or modifying the `--config` parameter to point to the new scheduler configuration file.

   ```yaml
  apiVersion: v1
  kind: Pod
  metadata:
    creationTimestamp: null
    labels:
      component: kube-scheduler
      tier: control-plane
    name: kube-scheduler
    namespace: kube-system
  spec:
    containers:
      - command:
          - kube-scheduler
          - --authentication-kubeconfig=/etc/kubernetes/scheduler.conf
          - --authorization-kubeconfig=/etc/kubernetes/scheduler.conf
          - --bind-address=127.0.0.1
          - --kubeconfig=/etc/kubernetes/scheduler.conf
          - --leader-elect=true
          - --config=/etc/kubernetes/kube-scheduler-config.yaml
        image: registry.cn-hangzhou.aliyuncs.com/google_containers/kube-scheduler:v1.28.0
        imagePullPolicy: IfNotPresent
        livenessProbe:
          failureThreshold: 8
          httpGet:
            host: 127.0.0.1
            path: /healthz
            port: 10259
            scheme: HTTPS
          initialDelaySeconds: 10
          periodSeconds: 10
          timeoutSeconds: 15
        name: kube-scheduler
        resources:
          requests:
            cpu: 100m
        startupProbe:
          failureThreshold: 24
          httpGet:
            host: 127.0.0.1
            path: /healthz
            port: 10259
            scheme: HTTPS
          initialDelaySeconds: 10
          periodSeconds: 10
          timeoutSeconds: 15
        volumeMounts:
          - mountPath: /etc/kubernetes/scheduler.conf
            name: kubeconfig
            readOnly: true
          - mountPath: /etc/kubernetes/kube-scheduler-config.yaml
            name: kube-scheduler-config-volume
            readOnly: true
    hostNetwork: true
    priority: 2000001000
    priorityClassName: system-node-critical
    securityContext:
      seccompProfile:
        type: RuntimeDefault
    volumes:
      - hostPath:
          path: /etc/kubernetes/scheduler.conf
          type: FileOrCreate
        name: kubeconfig
      - name: kube-scheduler-config-volume
        hostPath:
          path: /etc/kubernetes/kube-scheduler-config.yaml
          type: FileOrCreate
  status: {}
   ```
 - 3.2.1. Save and Exit:
   - Save the changes to the `kube-scheduler.yaml` file and exit the editor.

 - 3.2.4. Automatically Restart kube-scheduler:
   - Since kube-scheduler runs as a static Pod, kubelet will automatically detect the changes in the configuration file and restart kube-scheduler.

  - 3.2.1.5. Verify Configuration:
  - Use `kubectl get pods -n kube-system` to check if kube-scheduler is running properly.
  - Check the logs of kube-scheduler to ensure it has correctly loaded the new configuration and is interacting with the extender.

```bash
kubectl logs <kube-scheduler-pod-name> -n kube-system
```


    - Deploy workloads with labels indicating single or multiple replicas.
    - Observe the scheduling decisions to ensure they align with the strategy.
