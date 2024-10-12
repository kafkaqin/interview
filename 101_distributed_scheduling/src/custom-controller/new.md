当然可以！通过扩展 Kubernetes 的调度器配置，我们可以实现自定义调度策略而不需要编写新的调度器插件代码。Kubernetes 提供了灵活的插件系统，允许我们在现有的调度框架中添加自定义逻辑。以下是通过扩展配置实现自定义调度策略的详细步骤。

实现自定义调度策略的详细步骤
1. 创建自定义调度器配置
   我们将使用 Kubernetes 的 KubeSchedulerConfiguration 来定义自定义调度策略。具体来说，我们将使用 Policy 插件来优先选择带有 node.kubernetes.io/capacity: spot 标签的节点。

步骤 1.1: 创建自定义调度器配置文件
创建一个配置文件 custom-scheduler-config.yaml，定义自定义调度器的配置。

yaml
深色版本
apiVersion: kubescheduler.config.k8s.io/v1
kind: KubeSchedulerConfiguration
profiles:
- schedulerName: custom-scheduler
  pluginConfig:
    - name: NodeLabelPriority
      enabled: true
      weight: 1
      pluginNameToWeight:
      NodeLabelPriority: 10
      pluginConfig:
    - name: NodePreferAvoidPods
      enabled: false
      在这个配置文件中，我们定义了一个名为 NodeLabelPriority 的插件，并为其指定了权重。我们还禁用了默认的 NodePreferAvoidPods 插件。

2. 创建自定义调度器插件配置
   接下来，我们需要创建一个插件配置文件，用于定义 NodeLabelPriority 插件的行为。

步骤 2.1: 创建插件配置文件
创建一个配置文件 node-label-priority-config.yaml，定义 NodeLabelPriority 插件的配置。

yaml
深色版本
apiVersion: kubescheduler.config.k8s.io/v1
kind: Policy
extenders:
- urlPrefix: ""
  bindAddress: ""
  nodeLabelPriority:
  labels:
    - key: node.kubernetes.io/capacity
      value: spot
      weight: 10
      在这个配置文件中，我们定义了 nodeLabelPriority 配置，指定了标签 node.kubernetes.io/capacity: spot 的权重为 10。

3. 配置 Kubernetes 使用自定义调度器
   我们需要将自定义调度器配置应用到 Kubernetes 集群中。

步骤 3.1: 创建 ConfigMap
创建一个 ConfigMap 来存储自定义调度器配置文件。

yaml
深色版本
apiVersion: v1
kind: ConfigMap
metadata:
name: custom-scheduler-config
data:
custom-scheduler-config.yaml: |
apiVersion: kubescheduler.config.k8s.io/v1
kind: KubeSchedulerConfiguration
profiles:
- schedulerName: custom-scheduler
pluginConfig:
- name: NodeLabelPriority
enabled: true
weight: 1
pluginNameToWeight:
NodeLabelPriority: 10
pluginConfig:
- name: NodePreferAvoidPods
enabled: false
node-label-priority-config.yaml: |
apiVersion: kubescheduler.config.k8s.io/v1
kind: Policy
extenders:
- urlPrefix: ""
bindAddress: ""
nodeLabelPriority:
labels:
- key: node.kubernetes.io/capacity
value: spot
weight: 10
应用 ConfigMap。

sh
深色版本
kubectl apply -f custom-scheduler-configmap.yaml
步骤 3.2: 创建自定义调度器 Deployment
创建一个 Deployment 来运行自定义调度器。

yaml
深色版本
apiVersion: apps/v1
kind: Deployment
metadata:
name: custom-scheduler
spec:
replicas: 1
selector:
matchLabels:
app: custom-scheduler
template:
metadata:
labels:
app: custom-scheduler
spec:
containers:
- name: custom-scheduler
image: k8s.gcr.io/kube-scheduler:v1.22.0
command:
- kube-scheduler
args:
- --config=/etc/kubernetes/custom-scheduler-config.yaml
volumeMounts:
- name: config-volume
mountPath: /etc/kubernetes
volumes:
- name: config-volume
configMap:
name: custom-scheduler-config
应用 Deployment。

sh
深色版本
kubectl apply -f custom-scheduler-deployment.yaml
4. 使用自定义调度器
   在工作负载中指定使用自定义调度器。

步骤 4.1: 更新 Deployment
在 Deployment 中指定使用自定义调度器。

yaml
深色版本
apiVersion: apps/v1
kind: Deployment
metadata:
name: example-deployment
spec:
replicas: 3
selector:
matchLabels:
app: example-app
template:
metadata:
labels:
app: example-app
spec:
schedulerName: custom-scheduler
affinity:
podAntiAffinity:
requiredDuringSchedulingIgnoredDuringExecution:
- labelSelector:
matchExpressions:
- key: app
operator: In
values:
- example-app
topologyKey: kubernetes.io/hostname
tolerations:
- key: "node.kubernetes.io/capacity"
operator: "Equal"
value: "spot"
effect: "NoSchedule"
containers:
- name: example-container
image: example-image:latest
ports:
- containerPort: 8080
应用更新后的 Deployment。

sh
深色版本
kubectl apply -f example-deployment.yaml
5. 测试和验证
   测试自定义调度器是否按预期工作。

步骤 5.1: 部署工作负载
应用更新后的 Deployment。

sh
深色版本
kubectl apply -f example-deployment.yaml
步骤 5.2: 检查调度结果
检查 Pod 是否被调度到 spot 节点上。

sh
深色版本
kubectl get pods -o wide
总结
通过以上步骤，我们实现了自定义调度策略，优先选择带有 node.kubernetes.io/capacity: spot 标签的节点进行调度，同时确保高可用性。这种方法利用了 Kubernetes 的扩展配置功能，避免了编写新的调度器插件代码，简化了实现过程。这不仅降低了成本，还提高了系统的可靠性和稳定性。