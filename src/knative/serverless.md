

### Kubernetes CRD

自定义资源

### Serving

Revision（修订版本）、Configuration （配置）、Route（路由）、 Service（服务）

Knative Serving 维护某一时刻的快照，提供自动化伸缩功能（既支持扩容，也支持缩容直至为零)，以及处理必要的路由和网络编排。

### Buid

- Build

> 驱动构建过程的自定义 Kubernetes 资源。在定义构建时，您需要定义如何获取源代码以及如何创建容器镜像来运行代码。

- Build Template

> 封装可重复构建步骤集合以及允许对构建进行参数化的模板。

- Service Account

> 允许对私有资源（如 Git 仓库或容器镜像库）进行身份验证。

### Eventing

Source（源）、Channel（通道）和 Subscription（订阅）

Source 

Source是事件的来源，它是我们定义事件在何处生成以及如何将事件传递给关注对象的方式

Channel

Channel通道处理缓冲和持久性，即使该服务已被关闭时也确保将事件传递到其预期的服务。



### Istio





























































参考资料：

1. https://www.servicemesher.com/getting-started-with-knative/knative-overview.html
2. https://www.servicemesher.com/blog/knative-serving-autoscaler-single-tenancy-deep-dive/
3. 容器云未来：Kubernetes、Istio 和 Knative：https://zhuanlan.zhihu.com/p/67872233
4. Knative：重新定义 serverless：https://www.servicemesher.com/blog/knative-redefine-serverless/
5. Knative 基本功能深入剖析：Knative Eventing 之 Sequence 介绍：https://www.infoq.cn/article/Fvgmyr1As0bqXTpypG9i