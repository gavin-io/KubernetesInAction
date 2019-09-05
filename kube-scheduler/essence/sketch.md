## kube-scheduler

### 参数配置

options

options 的默认配置

根据数据类型，从schema中选择核实的默认配置器，通过默认配置器Default配置默认值

k8s.io/kubernetes/pkg/scheduler/apis/config/v1alpha1.RegisterDefaults.func1

SetDefaults_KubeSchedulerConfiguration



Options  （returns default scheduler app options）

给使用用户提供的可配置的参数，包括从configfile读取配置项的文件路径

Config 

生成并启动kube-scheduler所需的所有配置项，包括configfile读取的配置项

1. 如果参数--tls-cert-file和--tls-private-key-file未指定，MaybeDefaultWithSelfSignedCerts（）生成自签名证书到--cert-dir指定目录下（CertDirectory/PairName.crt and CertDirectory/PairName.key），如果--cert-dir未指定，生成自签名证书到内存中。
2. ApplyTo()将option{}映射到scheduler的config{}



CompletedConfig

*option* -> o.Config() -> *config* -> c.Complete() -> *cc*->factory.Configurator->factory.Config->sched



Start all informers

go cc.PodInformer.Informer().Run(stopCh)

```go
type podInformer struct {
	informer cache.SharedIndexInformer
}

func (i *podInformer) Informer() cache.SharedIndexInformer {
	return i.informer
}

func (i *podInformer) Lister() corelisters.PodLister {
	return corelisters.NewPodLister(i.informer.GetIndexer())
}
```

```go
// NewPodInformer creates a shared index informer that returns only non-terminal pods.
func NewPodInformer(client clientset.Interface, resyncPeriod time.Duration) coreinformers.PodInformer {
	selector := fields.ParseSelectorOrDie(
		"status.phase!=" + string(v1.PodSucceeded) +
			",status.phase!=" + string(v1.PodFailed))
	lw := cache.NewListWatchFromClient(client.CoreV1().RESTClient(), string(v1.ResourcePods), metav1.NamespaceAll, selector)
	return &podInformer{
		informer: cache.NewSharedIndexInformer(lw, &v1.Pod{}, resyncPeriod, cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc}),
	}
}
```

```go
c.PodInformer = factory.NewPodInformer(client, 0)
```

```go
c, err := opts.Config()
```

cc.InformerFactory.Start(stopCh)

```go
// NewSharedInformerFactory constructs a new instance of sharedInformerFactory for all namespaces.
func NewSharedInformerFactory(client kubernetes.Interface, defaultResync time.Duration) SharedInformerFactory {
	return NewSharedInformerFactoryWithOptions(client, defaultResync)
}
```

```go
c.InformerFactory = informers.NewSharedInformerFactory(client, 0)
```

cc.InformerFactory.WaitForCacheSync(stopCh)

### 事件机制

> kube-scheduler options.Config()为例

```go
import  (
"k8s.io/client-go/tools/events"
)

```

### List-Watch机制

> 异步消息系统

4个原则：

可靠性、实时性、顺序性、高性能

> 参考：
>
> 理解 K8S 的设计精髓之 list-watch： http://wsfdl.com/kubernetes/2019/01/10/list_watch_in_k8s.html

> 参考：
>
> Kubernetes Informer 详解： https://www.kubernetes.org.cn/2693.html



### 运行调度

sched.Run()

> pkg/scheduler/scheduler.go

```go
func (sched *Scheduler) scheduleOne() {
    //取出待调度pod
    pod := sched.NextPod()
    
    //选择可调度节点
    scheduleResult, err := sched.schedule(pod, pluginContext)
    
    //无可调度节点执行抢占逻辑
    sched.preempt(fwk, pod, fitError)
    
    //假性绑定
    allBound, err := sched.assumeVolumes(assumedPod, scheduleResult.SuggestedHost)
    
    
}
```

#### 选择可调度节点

实现Algorithm接口

> pkg/scheduler/scheduler.go func New() {...}

predicatekeys

```
"MaxGCEPDVolumeCount":<k8s.io/kubernetes/vendor/k8s.io/apimachinery/pkg/util/sets.Empty>
"MaxCSIVolumeCountPred":<k8s.io/kubernetes/vendor/k8s.io/apimachinery/pkg/util/sets.Empty>
"NoDiskConflict":<k8s.io/kubernetes/vendor/k8s.io/apimachinery/pkg/util/sets.Empty>
"GeneralPredicates":<k8s.io/kubernetes/vendor/k8s.io/apimachinery/pkg/util/sets.Empty>
"MaxEBSVolumeCount":<k8s.io/kubernetes/vendor/k8s.io/apimachinery/pkg/util/sets.Empty>
"MaxAzureDiskVolumeCount":<k8s.io/kubernetes/vendor/k8s.io/apimachinery/pkg/util/sets.Empty>
"MatchInterPodAffinity":<k8s.io/kubernetes/vendor/k8s.io/apimachinery/pkg/util/sets.Empty>
"CheckNodeUnschedulable":<k8s.io/kubernetes/vendor/k8s.io/apimachinery/pkg/util/sets.Empty>
"PodToleratesNodeTaints":<k8s.io/kubernetes/vendor/k8s.io/apimachinery/pkg/util/sets.Empty>
"CheckVolumeBinding":<k8s.io/kubernetes/vendor/k8s.io/apimachinery/pkg/util/sets.Empty>
"NoVolumeZoneConflict":<k8s.io/kubernetes/vendor/k8s.io/apimachinery/pkg/util/sets.Empty>
```

prioritykeys

```
"SelectorSpreadPriority":<k8s.io/kubernetes/vendor/k8s.io/apimachinery/pkg/util/sets.Empty>
"InterPodAffinityPriority":<k8s.io/kubernetes/vendor/k8s.io/apimachinery/pkg/util/sets.Empty>
"LeastRequestedPriority":<k8s.io/kubernetes/vendor/k8s.io/apimachinery/pkg/util/sets.Empty>
"BalancedResourceAllocation":<k8s.io/kubernetes/vendor/k8s.io/apimachinery/pkg/util/sets.Empty>
"NodePreferAvoidPodsPriority":<k8s.io/kubernetes/vendor/k8s.io/apimachinery/pkg/util/sets.Empty>
"NodeAffinityPriority":<k8s.io/kubernetes/vendor/k8s.io/apimachinery/pkg/util/sets.Empty>
"TaintTolerationPriority":<k8s.io/kubernetes/vendor/k8s.io/apimachinery/pkg/util/sets.Empty>
"ImageLocalityPriority":<k8s.io/kubernetes/vendor/k8s.io/apimachinery/pkg/util/sets.Empty>
```



执行调度逻辑

> pkg/scheduler/core/generic_scheduler.go

```
func NewGenericScheduler
```

```go
type genericScheduler struct {

    cache                    internalcache.Cache

    schedulingQueue          internalqueue.SchedulingQueue

    predicates               map[string]predicates.FitPredicate

    priorityMetaProducer     priorities.PriorityMetadataProducer

    predicateMetaProducer    predicates.PredicateMetadataProducer

    prioritizers             []priorities.PriorityConfig

    framework                framework.Framework

    extenders                []algorithm.SchedulerExtender

    alwaysCheckAllPredicates bool

    nodeInfoSnapshot         *schedulernodeinfo.Snapshot

    volumeBinder             *volumebinder.VolumeBinder

    pvcLister                corelisters.PersistentVolumeClaimLister

    pdbLister                algorithm.PDBLister

    disablePreemption        bool

    percentageOfNodesToScore int32

    enableNonPreempting      bool

}

```

```go
// Schedule tries to schedule the given pod to one of the nodes in the node list.
// If it succeeds, it will return the name of the node.
// If it fails, it will return a FitError error with reasons.
func (g *genericScheduler) Schedule(pod *v1.Pod, pluginContext *framework.PluginContext) (result ScheduleResult, err error) {
    findNodesThatFit
    if len(filteredNodes) == 1 {
        return ScheduleResult{
			SuggestedHost:  filteredNodes[0].Name,
			EvaluatedNodes: 1 + len(failedPredicateMap),
			FeasibleNodes:  1,
		}, nil
    }
    PrioritizeNodes
}
```

sched.Framework

> pkg/scheduler/factory/factory.go

```go
func NewConfigFactory(args *ConfigFactoryArgs) *Configurator {
    // framework has a set of plugins and the context used for running them.
    framework, err := framework.NewFramework(args.Registry, args.Plugins, args.PluginConfig)
    c := &Configurator{
        framework:   framework,
    }
}
```

预选

```go
// Filters the nodes to find the ones that fit based on the given predicate functions
// Each node is passed through the predicate functions to determine if it is a fit
func (g *genericScheduler) findNodesThatFit {
    //获取numNodesToFind
    //获取meta?
    podFitsOnNode
    g.framework.RunFilterPlugins
}
```

优选

```go
// PrioritizeNodes prioritizes the nodes by running the individual priority functions in parallel.
// Each priority function is expected to set a score of 0-10
// 0 is the lowest priority score (least preferred node) and 10 is the highest
// Each priority function can also have its own weight
// The node scores returned by the priority function are multiplied by the weights to get weighted scores
// All scores are finally combined (added) to get the total weighted scores of all nodes
func PrioritizeNodes(
	pod *v1.Pod,
	nodeNameToInfo map[string]*schedulernodeinfo.NodeInfo,
	meta interface{},
	priorityConfigs []priorities.PriorityConfig,
	nodes []*v1.Node,
	extenders []algorithm.SchedulerExtender,
	framework framework.Framework,
    pluginContext *framework.PluginContext) (schedulerapi.HostPriorityList, error) {
    
    for i := range priorityConfigs {
		if priorityConfigs[i].Function != nil {
			wg.Add(1)
			go func(index int) {
				defer wg.Done()
				var err error
				results[index], err = priorityConfigs[index].Function(pod, nodeNameToInfo, nodes)
				if err != nil {
					appendError(err)
				}
			}(i)
		} else {
			results[i] = make(schedulerapi.HostPriorityList, len(nodes))
		}
	}
    
}
```

```go
// HostPriority represents the priority of scheduling to a particular host, higher priority is better.
type HostPriority struct {
	// Name of the host
	Host string
	// Score associated with the host
	Score int
}

// HostPriorityList declares a []HostPriority type.
type HostPriorityList []HostPriority

results := make([]schedulerapi.HostPriorityList, len(priorityConfigs), len(priorityConfigs))
```

priorityConfigs[index].Function, 例如：

"InterPodAffinityPriority"：func (ipa *InterPodAffinity) CalculateInterPodAffinityPriority(pod *v1.Pod, nodeNameToInfo map[string]*schedulernodeinfo.NodeInfo, nodes []*v1.Node) (schedulerapi.HostPriorityList, error) 

> pkg/scheduler/algorithm/priorities/interpod_affinity.go

```go
func (ipa *InterPodAffinity) CalculateInterPodAffinityPriority(pod v1.Pod, nodeNameToInfo map[string]schedulernodeinfo.NodeInfo, nodes []*v1.Node) (schedulerapi.HostPriorityList, error) {
    
}
```

将来全部采用Map-Reduce模式统计node-score

```go
	workqueue.ParallelizeUntil(context.TODO(), 16, len(nodes), func(index int) {
		nodeInfo := nodeNameToInfo[nodes[index].Name]
		for i := range priorityConfigs {
			if priorityConfigs[i].Function != nil {
				continue
			}

			var err error
			results[i][index], err = priorityConfigs[i].Map(pod, meta, nodeInfo)
			if err != nil {
				appendError(err)
				results[i][index].Host = nodes[index].Name
			}
		}
	})

	for i := range priorityConfigs {
		if priorityConfigs[i].Reduce == nil {
			continue
		}
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			if err := priorityConfigs[index].Reduce(pod, meta, nodeNameToInfo, results[index]); err != nil {
				appendError(err)
			}
			if klog.V(10) {
				for _, hostPriority := range results[index] {
					klog.Infof("%v -> %v: %v, Score: (%d)", util.GetPodFullName(pod), hostPriority.Host, priorityConfigs[index].Name, hostPriority.Score)
				}
			}
		}(i)
	}
```

priorityConfigs[i].Map(pod, meta, nodeInfo)，例如：

> pkg/scheduler/algorithm/priorities/resource_allocation.go

```go
// PriorityMap priorities nodes according to the resource allocations on the node.
// It will use `scorer` function to calculate the score.
func (r *ResourceAllocationPriority) PriorityMap(
	pod *v1.Pod,
	meta interface{},
	nodeInfo *schedulernodeinfo.NodeInfo) (schedulerapi.HostPriority, error)
```

> pkg/scheduler/algorithm/priorities/reduce.go，[0, maxPriority]标准化

```go
// NormalizeReduce generates a PriorityReduceFunction that can normalize the result
// scores to [0, maxPriority]. If reverse is set to true, it reverses the scores by
// subtracting it from maxPriority.
func NormalizeReduce(maxPriority int, reverse bool) PriorityReduceFunction {
    
}
```



> pkg/scheduler/framework/v1alpha1/framework.go

```go
// RunScorePlugins runs the set of configured scoring plugins. It returns a list that
// stores for each scoring plugin name the corresponding NodeScoreList(s).
// It also returns *Status, which is set to non-success if any of the plugins returns
// a non-success status.
func (f *framework) RunScorePlugins(pc *PluginContext, pod *v1.Pod, nodes []*v1.Node) (PluginToNodeScores, *Status) {
    
}
```

统计分值
```go
// Summarize all scores.
	result := make(schedulerapi.HostPriorityList, 0, len(nodes))

	for i := range nodes {
		result = append(result, schedulerapi.HostPriority{Host: nodes[i].Name, Score: 0})
		for j := range priorityConfigs {
			result[i].Score += results[j][i].Score * priorityConfigs[j].Weight
		}

		for j := range scoresMap {
			result[i].Score += scoresMap[j][i].Score
		}
	}
```

优选打分-数据结构
|      | node1 | node2 | ...  | nodeN |  |
| ---- | ----- | ---- | ----- | ----- | ---- |
| priority1 | Map（打分） |      |       |  | Reduece（标准化） |
| priority2 | Map（打分） | | |  | Reduece（标准化） |
| ... | Map（打分） | | |  | Reduece（标准化） |
| priority | Map（打分） | | |  | Reduece（标准化） |
| Total | 加和统计 | | |  |  |

选择最高分值

> pkg/scheduler/core/generic_scheduler.go

```go
// selectHost takes a prioritized list of nodes and then picks one
// in a reservoir sampling manner from the nodes that had the highest score.
func (g *genericScheduler) selectHost(priorityList schedulerapi.HostPriorityList) (string, error) {}
```



#### 抢占逻辑（当无可调度节点时）

> pkg/scheduler/scheduler.go

```go
// preempt tries to create room for a pod that has failed to schedule, by preempting lower priority pods if possible.
// If it succeeds, it adds the name of the node where preemption has happened to the pod spec.
// It returns the node name and an error if any.
func (sched *Scheduler) preempt(fwk framework.Framework, preemptor *v1.Pod, scheduleErr error) (string, error) {
    
}
```

> pkg/scheduler/factory/factory.go

``` go
func (p *podPreemptor) GetUpdatedPod(pod *v1.Pod) (*v1.Pod, error) {
	return p.Client.CoreV1().Pods(pod.Namespace).Get(pod.Name, metav1.GetOptions{})
}
```


















## 附:

>  kube-scheduler的--help的所有参数

/ # kube-scheduler --help
The Kubernetes scheduler is a policy-rich, topology-aware,
workload-specific function that significantly impacts availability, performance,
and capacity. The scheduler needs to take into account individual and collective
resource requirements, quality of service requirements, hardware/software/policy
constraints, affinity and anti-affinity specifications, data locality, inter-workload
interference, deadlines, and so on. Workload-specific requirements will be exposed
through the API as necessary.

Usage:
  kube-scheduler [flags]

Misc flags:

      --config string                                                                                                                                          
                The path to the configuration file. Flags override values in this file.
      --master string                                                                                                                                          
                The address of the Kubernetes API server (overrides any value in kubeconfig)
      --write-config-to string                                                                                                                                 
                If set, write the configuration values to this file and exit.

Secure serving flags:

      --bind-address ip                                                                                                                                        
                The IP address on which to listen for the --secure-port port. The associated interface(s) must be reachable by the rest of the cluster, and by
                CLI/web clients. If blank, all interfaces will be used (0.0.0.0 for all IPv4 interfaces and :: for all IPv6 interfaces). (default 0.0.0.0)
      --cert-dir string                                                                                                                                        
                The directory where the TLS certs are located. If --tls-cert-file and --tls-private-key-file are provided, this flag will be ignored.
      --http2-max-streams-per-connection int                                                                                                                   
                The limit that the server gives to clients for the maximum number of streams in an HTTP/2 connection. Zero means to use golang's default.
      --secure-port int                                                                                                                                        
                The port on which to serve HTTPS with authentication and authorization.If 0, don't serve HTTPS at all. (default 10259)
      --tls-cert-file string                                                                                                                                   
                File containing the default x509 Certificate for HTTPS. (CA cert, if any, concatenated after server cert). If HTTPS serving is enabled, and
                --tls-cert-file and --tls-private-key-file are not provided, a self-signed certificate and key are generated for the public address and saved to
                the directory specified by --cert-dir.
      --tls-cipher-suites strings                                                                                                                              
                Comma-separated list of cipher suites for the server. If omitted, the default Go cipher suites will be use.  Possible values:
                TLS_ECDHE_ECDSA_WITH_AES_128_CBC_SHA,TLS_ECDHE_ECDSA_WITH_AES_128_CBC_SHA256,TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,TLS_ECDHE_ECDSA_WITH_AES_256_CBC_SHA,TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305,TLS_ECDHE_ECDSA_WITH_RC4_128_SHA,TLS_ECDHE_RSA_WITH_3DES_EDE_CBC_SHA,TLS_ECDHE_RSA_WITH_AES_128_CBC_SHA,TLS_ECDHE_RSA_WITH_AES_128_CBC_SHA256,TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,TLS_ECDHE_RSA_WITH_AES_256_CBC_SHA,TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305,TLS_ECDHE_RSA_WITH_RC4_128_SHA,TLS_RSA_WITH_3DES_EDE_CBC_SHA,TLS_RSA_WITH_AES_128_CBC_SHA,TLS_RSA_WITH_AES_128_CBC_SHA256,TLS_RSA_WITH_AES_128_GCM_SHA256,TLS_RSA_WITH_AES_256_CBC_SHA,TLS_RSA_WITH_AES_256_GCM_SHA384,TLS_RSA_WITH_RC4_128_SHA
      --tls-min-version string                                                                                                                                 
                Minimum TLS version supported. Possible values: VersionTLS10, VersionTLS11, VersionTLS12
      --tls-private-key-file string                                                                                                                            
                File containing the default x509 private key matching --tls-cert-file.
      --tls-sni-cert-key namedCertKey                                                                                                                          
                A pair of x509 certificate and private key file paths, optionally suffixed with a list of domain patterns which are fully qualified domain names,
                possibly with prefixed wildcard segments. If no domain patterns are provided, the names of the certificate are extracted. Non-wildcard matches
                trump over wildcard matches, explicit domain patterns trump over extracted names. For multiple key/certificate pairs, use the --tls-sni-cert-key
                multiple times. Examples: "example.crt,example.key" or "foo.crt,foo.key:*.foo.com,foo.com". (default [])

Insecure serving flags:

      --address string                                                                                                                                         
                DEPRECATED: the IP address on which to listen for the --port port (set to 0.0.0.0 for all IPv4 interfaces and :: for all IPv6 interfaces). See
                --bind-address instead. (default "0.0.0.0")
      --port int                                                                                                                                               
                DEPRECATED: the port on which to serve HTTP insecurely without authentication and authorization. If 0, don't serve HTTPS at all. See --secure-port
                instead. (default 10251)

Authentication flags:

      --authentication-kubeconfig string                                                                                                                       
                kubeconfig file pointing at the 'core' kubernetes server with enough rights to create tokenaccessreviews.authentication.k8s.io. This is optional.
                If empty, all token requests are considered to be anonymous and no client CA is looked up in the cluster.
      --authentication-skip-lookup                                                                                                                             
                If false, the authentication-kubeconfig will be used to lookup missing authentication configuration from the cluster.
      --authentication-token-webhook-cache-ttl duration                                                                                                        
                The duration to cache responses from the webhook token authenticator. (default 10s)
      --authentication-tolerate-lookup-failure                                                                                                                 
                If true, failures to look up missing authentication configuration from the cluster are not considered fatal. Note that this can result in
                authentication that treats all requests as anonymous. (default true)
      --client-ca-file string                                                                                                                                  
                If set, any request presenting a client certificate signed by one of the authorities in the client-ca-file is authenticated with an identity
                corresponding to the CommonName of the client certificate.
      --requestheader-allowed-names strings                                                                                                                    
                List of client certificate common names to allow to provide usernames in headers specified by --requestheader-username-headers. If empty, any
                client certificate validated by the authorities in --requestheader-client-ca-file is allowed.
      --requestheader-client-ca-file string                                                                                                                    
                Root certificate bundle to use to verify client certificates on incoming requests before trusting usernames in headers specified by
                --requestheader-username-headers. WARNING: generally do not depend on authorization being already done for incoming requests.
      --requestheader-extra-headers-prefix strings                                                                                                             
                List of request header prefixes to inspect. X-Remote-Extra- is suggested. (default [x-remote-extra-])
      --requestheader-group-headers strings                                                                                                                    
                List of request headers to inspect for groups. X-Remote-Group is suggested. (default [x-remote-group])
      --requestheader-username-headers strings                                                                                                                 
                List of request headers to inspect for usernames. X-Remote-User is common. (default [x-remote-user])

Authorization flags:

      --authorization-always-allow-paths strings                                                                                                               
                A list of HTTP paths to skip during authorization, i.e. these are authorized without contacting the 'core' kubernetes server. (default [/healthz])
      --authorization-kubeconfig string                                                                                                                        
                kubeconfig file pointing at the 'core' kubernetes server with enough rights to create subjectaccessreviews.authorization.k8s.io. This is optional.
                If empty, all requests not skipped by authorization are forbidden.
      --authorization-webhook-cache-authorized-ttl duration                                                                                                    
                The duration to cache 'authorized' responses from the webhook authorizer. (default 10s)
      --authorization-webhook-cache-unauthorized-ttl duration                                                                                                  
                The duration to cache 'unauthorized' responses from the webhook authorizer. (default 10s)

Deprecated flags:

      --algorithm-provider string                                                                                                                              
                DEPRECATED: the scheduling algorithm provider to use, one of: ClusterAutoscalerProvider | DefaultProvider
      --contention-profiling                                                                                                                                   
                DEPRECATED: enable lock contention profiling, if profiling is enabled
      --kube-api-burst int32                                                                                                                                   
                DEPRECATED: burst to use while talking with kubernetes apiserver (default 100)
      --kube-api-content-type string                                                                                                                           
                DEPRECATED: content type of requests sent to apiserver. (default "application/vnd.kubernetes.protobuf")
      --kube-api-qps float32                                                                                                                                   
                DEPRECATED: QPS to use while talking with kubernetes apiserver (default 50)
      --kubeconfig string                                                                                                                                      
                DEPRECATED: path to kubeconfig file with authorization and master location information.
      --lock-object-name string                                                                                                                                
                DEPRECATED: define the name of the lock object. (default "kube-scheduler")
      --lock-object-namespace string                                                                                                                           
                DEPRECATED: define the namespace of the lock object. (default "kube-system")
      --policy-config-file string                                                                                                                              
                DEPRECATED: file with scheduler policy configuration. This file is used if policy ConfigMap is not provided or --use-legacy-policy-config=true
      --policy-configmap string                                                                                                                                
                DEPRECATED: name of the ConfigMap object that contains scheduler's policy configuration. It must exist in the system namespace before scheduler
                initialization if --use-legacy-policy-config=false. The config must be provided as the value of an element in 'Data' map with the key='policy.cfg'
      --policy-configmap-namespace string                                                                                                                      
                DEPRECATED: the namespace where policy ConfigMap is located. The kube-system namespace will be used if this is not provided or is empty. (default
                "kube-system")
      --profiling                                                                                                                                              
                DEPRECATED: enable profiling via web interface host:port/debug/pprof/
      --scheduler-name string                                                                                                                                  
                DEPRECATED: name of the scheduler, used to select which pods will be processed by this scheduler, based on pod's "spec.schedulerName". (default
                "default-scheduler")
      --use-legacy-policy-config                                                                                                                               
                DEPRECATED: when set to true, scheduler will ignore policy ConfigMap and uses policy config file

Leader election flags:

      --leader-elect                                                                                                                                           
                Start a leader election client and gain leadership before executing the main loop. Enable this when running replicated components for high
                availability. (default true)
      --leader-elect-lease-duration duration                                                                                                                   
                The duration that non-leader candidates will wait after observing a leadership renewal until attempting to acquire leadership of a led but
                unrenewed leader slot. This is effectively the maximum duration that a leader can be stopped before it is replaced by another candidate. This is
                only applicable if leader election is enabled. (default 15s)
      --leader-elect-renew-deadline duration                                                                                                                   
                The interval between attempts by the acting master to renew a leadership slot before it stops leading. This must be less than or equal to the
                lease duration. This is only applicable if leader election is enabled. (default 10s)
      --leader-elect-resource-lock endpoints                                                                                                                   
                The type of resource object that is used for locking during leader election. Supported options are endpoints (default) and `configmaps`. (default
                "endpoints")
      --leader-elect-retry-period duration                                                                                                                     
                The duration the clients should wait between attempting acquisition and renewal of a leadership. This is only applicable if leader election is
                enabled. (default 2s)

Feature gate flags:

      --feature-gates mapStringBool                                                                                                                            
                A set of key=value pairs that describe feature gates for alpha/experimental features. Options are:
                APIListChunking=true|false (BETA - default=true)
                APIResponseCompression=true|false (ALPHA - default=false)
                AllAlpha=true|false (ALPHA - default=false)
                AppArmor=true|false (BETA - default=true)
                AttachVolumeLimit=true|false (BETA - default=true)
                BalanceAttachedNodeVolumes=true|false (ALPHA - default=false)
                BlockVolume=true|false (BETA - default=true)
                BoundServiceAccountTokenVolume=true|false (ALPHA - default=false)
                CPUManager=true|false (BETA - default=true)
                CRIContainerLogRotation=true|false (BETA - default=true)
                CSIBlockVolume=true|false (BETA - default=true)
                CSIDriverRegistry=true|false (BETA - default=true)
                CSIInlineVolume=true|false (ALPHA - default=false)
                CSIMigration=true|false (ALPHA - default=false)
                CSIMigrationAWS=true|false (ALPHA - default=false)
                CSIMigrationGCE=true|false (ALPHA - default=false)
                CSIMigrationOpenStack=true|false (ALPHA - default=false)
                CSINodeInfo=true|false (BETA - default=true)
                CustomCPUCFSQuotaPeriod=true|false (ALPHA - default=false)
                CustomResourcePublishOpenAPI=true|false (ALPHA - default=false)
                CustomResourceSubresources=true|false (BETA - default=true)
                CustomResourceValidation=true|false (BETA - default=true)
                CustomResourceWebhookConversion=true|false (ALPHA - default=false)
                DebugContainers=true|false (ALPHA - default=false)
                DevicePlugins=true|false (BETA - default=true)
                DryRun=true|false (BETA - default=true)
                DynamicAuditing=true|false (ALPHA - default=false)
                DynamicKubeletConfig=true|false (BETA - default=true)
                ExpandCSIVolumes=true|false (ALPHA - default=false)
                ExpandInUsePersistentVolumes=true|false (ALPHA - default=false)
                ExpandPersistentVolumes=true|false (BETA - default=true)
                ExperimentalCriticalPodAnnotation=true|false (ALPHA - default=false)
                ExperimentalHostUserNamespaceDefaulting=true|false (BETA - default=false)
                HyperVContainer=true|false (ALPHA - default=false)
                KubeletPodResources=true|false (ALPHA - default=false)
                LocalStorageCapacityIsolation=true|false (BETA - default=true)
                MountContainers=true|false (ALPHA - default=false)
                NodeLease=true|false (BETA - default=true)
                PodShareProcessNamespace=true|false (BETA - default=true)
                ProcMountType=true|false (ALPHA - default=false)
                QOSReserved=true|false (ALPHA - default=false)
                ResourceLimitsPriorityFunction=true|false (ALPHA - default=false)
                ResourceQuotaScopeSelectors=true|false (BETA - default=true)
                RotateKubeletClientCertificate=true|false (BETA - default=true)
                RotateKubeletServerCertificate=true|false (BETA - default=true)
                RunAsGroup=true|false (BETA - default=true)
                RuntimeClass=true|false (BETA - default=true)
                SCTPSupport=true|false (ALPHA - default=false)
                ScheduleDaemonSetPods=true|false (BETA - default=true)
                ServerSideApply=true|false (ALPHA - default=false)
                ServiceNodeExclusion=true|false (ALPHA - default=false)
                StorageVersionHash=true|false (ALPHA - default=false)
                StreamingProxyRedirects=true|false (BETA - default=true)
                SupportNodePidsLimit=true|false (ALPHA - default=false)
                SupportPodPidsLimit=true|false (BETA - default=true)
                Sysctls=true|false (BETA - default=true)
                TTLAfterFinished=true|false (ALPHA - default=false)
                TaintBasedEvictions=true|false (BETA - default=true)
                TaintNodesByCondition=true|false (BETA - default=true)
                TokenRequest=true|false (BETA - default=true)
                TokenRequestProjection=true|false (BETA - default=true)
                ValidateProxyRedirects=true|false (BETA - default=true)
                VolumeSnapshotDataSource=true|false (ALPHA - default=false)
                VolumeSubpathEnvExpansion=true|false (ALPHA - default=false)
                WinDSR=true|false (ALPHA - default=false)
                WinOverlay=true|false (ALPHA - default=false)
                WindowsGMSA=true|false (ALPHA - default=false)

Global flags:

      --alsologtostderr                                                                                                                                        
                log to standard error as well as files
  -h, --help                                                                                                                                                   
​                help for kube-scheduler
​      --log-backtrace-at traceLocation                                                                                                                         
​                when logging hits line file:N, emit a stack trace (default :0)
​      --log-dir string                                                                                                                                         
​                If non-empty, write log files in this directory
​      --log-file string                                                                                                                                        
​                If non-empty, use this log file
​      --log-flush-frequency duration                                                                                                                           
​                Maximum number of seconds between log flushes (default 5s)
​      --logtostderr                                                                                                                                            
​                log to standard error instead of files (default true)
​      --skip-headers                                                                                                                                           
​                If true, avoid header prefixes in the log messages
​      --stderrthreshold severity                                                                                                                               
​                logs at or above this threshold go to stderr (default 2)
  -v, --v Level                                                                                                                                                
​                number for the log level verbosity
​      --version version[=true]                                                                                                                                 
​                Print version information and quit
​      --vmodule moduleSpec                                                                                                                                     
​                comma-separated list of pattern=N settings for file-filtered logging



> kubeconfig file 的配置项

type KubeSchedulerConfiguration struct {

​    metav1.TypeMeta

​    // SchedulerName is name of the scheduler, used to select which pods

​    // will be processed by this scheduler, based on pod's "spec.SchedulerName".

​    SchedulerName string

​    // AlgorithmSource specifies the scheduler algorithm source.

​    AlgorithmSource SchedulerAlgorithmSource

​    // RequiredDuringScheduling affinity is not symmetric, but there is an implicit PreferredDuringScheduling affinity rule

​    // corresponding to every RequiredDuringScheduling affinity rule.

​    // HardPodAffinitySymmetricWeight represents the weight of implicit PreferredDuringScheduling affinity rule, in the range 0-100.

​    HardPodAffinitySymmetricWeight int32

​    // LeaderElection defines the configuration of leader election client.

​    LeaderElection KubeSchedulerLeaderElectionConfiguration

​    // ClientConnection specifies the kubeconfig file and client connection

​    // settings for the proxy server to use when communicating with the apiserver.

​    ClientConnection componentbaseconfig.ClientConnectionConfiguration

​    // HealthzBindAddress is the IP address and port for the health check server to serve on,

​    // defaulting to 0.0.0.0:10251

​    HealthzBindAddress string

​    // MetricsBindAddress is the IP address and port for the metrics server to

​    // serve on, defaulting to 0.0.0.0:10251.

​    MetricsBindAddress string

​    // DebuggingConfiguration holds configuration for Debugging related features

​    // TODO: We might wanna make this a substruct like Debugging componentbaseconfig.DebuggingConfiguration

​    componentbaseconfig.DebuggingConfiguration

​    // DisablePreemption disables the pod preemption feature.

​    DisablePreemption bool

​    // PercentageOfNodeToScore is the percentage of all nodes that once found feasible

​    // for running a pod, the scheduler stops its search for more feasible nodes in

​    // the cluster. This helps improve scheduler's performance. Scheduler always tries to find

​    // at least "minFeasibleNodesToFind" feasible nodes no matter what the value of this flag is.

​    // Example: if the cluster size is 500 nodes and the value of this flag is 30,

​    // then scheduler stops finding further feasible nodes once it finds 150 feasible ones.

​    // When the value is 0, default percentage (5%--50% based on the size of the cluster) of the

​    // nodes will be scored.

​    PercentageOfNodesToScore int32

​    // Duration to wait for a binding operation to complete before timing out

​    // Value must be non-negative integer. The value zero indicates no waiting.

​    // If this value is nil, the default value will be used.

​    BindTimeoutSeconds *int64

​    // Plugins specify the set of plugins that should be enabled or disabled. Enabled plugins are the

​    // ones that should be enabled in addition to the default plugins. Disabled plugins are any of the

​    // default plugins that should be disabled.

​    // When no enabled or disabled plugin is specified for an extension point, default plugins for

​    // that extension point will be used if there is any.

​    Plugins *Plugins

​    // PluginConfig is an optional set of custom plugin arguments for each plugin.

​    // Omitting config args for a plugin is equivalent to using the default config for that plugin.

​    PluginConfig []PluginConfig

}



















参考资料：

1. Kubernetes scheduler学习笔记: https://mp.weixin.qq.com/s?__biz=MzA5OTAyNzQ2OA==&mid=2649702449&idx=1&sn=8e6446948700becbe1cb23abbfecb82e&chksm=88937f52bfe4f6440016e6146af6214e201f8f8205ceda6364187fe9365b98826445bed6a3f0&scene=0&xtrack=1#rd
2. kubelet 源码分析： 事件处理：https://cizixs.com/2017/06/22/kubelet-source-code-analysis-part4-event/
3. 胡伟煌的gitbook：https://www.huweihuang.com/kubernetes-notes/code-analysis/kube-scheduler/scheduleOne.html
4. 胡伟煌的博客：https://www.huweihuang.com/