## kube-proxy中的IPVS总结

Kubernetes v1.8.0-beta.0后在kube-proxy中引入了ipvs mode。笔者目前使用***v1.15.3***版本，默认使用ipvs mode。

### 1. LVS和IPVS

说到IPVS（IP Virtual Server），不得不说说LVS。LVS（Linux Virtual Server），Linux虚拟服务器, 是一个由章文嵩博士发起的自由软件项目，现在已经是 Linux（linux 2.6+）标准内核的一部分。LVS用于负载均衡，转发效率很高，能够处理百万并发的连接请求。IPVS是LVS的核心模块，构建于Netfilter之上，作为Linux内核的一部分提供四层（传输层）的负载均衡。

工作在第四层的协议是TCP/UDP协议，也就是说四层负载均衡，是主要作用于TCP协议报文之上，基于IP和端口来判断需要重定向的报文，并修改报文中目标IP地址以进行重定向的负载均衡方式。OSI七层模型如下：

![OSI七层模型](/home/yangym/yangym/projects/gavin.io/KubernetesInAction/images/src/kube-proxy/OSI模型1.jpeg)



### 2. IPVS工作模式

IPVS主要有3中工作模式，各有利弊，先简述下它们各自的工作原理。在此之前，先熟悉几个名词：

- DS：Director Server，指的是前端负载均衡器。

- RS：Real Server，指的是后端工作的服务器。

- VIP：Virtual IP，用户请求的目标IP地址。

- DIP：Director Server IP，前端负载均衡器的IP地址。

- RIP：Real Server IP，后端服务器的IP地址。

- CIP：Client IP，访问客户端的IP地址。


#### 2.1 三种工作模式原理介绍

**NAT模式（Network Address Translation）**

NAT模式下，请求包和响应包都需要经过LB处理。当客户端的请求到达虚拟服务后，LB会对请求包做目的地址转换（DNAT），将请求包的目的IP改写为RS的IP。当收到RS的响应后，LB会对响应包做源地址转换（SNAT），将响应包的源IP改写为LB的IP。

**DR模式（Direct Routing）**

DR模式下，客户端的请求包到达负载均衡器的虚拟服务IP端口后，负载均衡器不会改写请求包的IP和端口，但是会改写请求包的MAC地址（源MAC地址是DIP所在接口的MAC，目标MAC是RS的RIP接口所在的MAC，IP首部不会发生变化），然后将数据包转发。

此时数据包将会发至真实服务器，真实服务器发现请求报文的MAC地址是自己的MAC地址，接收此报文，拆了MAC首部，发现目标地址是VIP后，向lo:0转发此报文（每个RS主机上都应有VIP，并且RIP配置在物理接口上，VIP配置在内置接口lo:0上），最终到达用户空间的进程。，然后向外发出。

真实服务器用户空间的进程构建响应报文，将响应报文通过lo:0接口传送给物理网卡处理请求后，响应包直接回给客户端，不再经过负载均衡器。

**FULLNAT模式**

FULLNAT下，客户端感知不到RS，RS也感知不到客户端，它们都只能看到LB。LB会对请求包和响应包都做SNAT和DNAT。



#### 2.2 三种工作模式利弊分析

**转发效率：**

DR>NAT>FULLNAT

**组网要求：**

NAT：LB和RS须位于同一个子网，并且客户端不能和LB/RS位于同一子网

DR：LB和RS须位于同一个子网

FULLNAT：没有要求

**端口映射**

NAT：支持

DR：不支持

FULLNAT：不支持



### 3. IPVS支持的调度算法

IPVS常用的负载均衡调度算法如下：

- 轮询（Round Robin）
- 加权轮询（Weighted Round Robin）
- 源地址哈希（SourceIP Hash）
- 目标地址哈希（Destination Hash）
- 最小连接数（Least Connections）
- 加权最小连接数（Weighted Least Connections）
- 最小期望延迟（ Shortest Expection Delay）



### 4. IPVS命令行操作

ipvsadm，ipvs在用户空间的命令行工具，命令行参数及使用方式如下：

```shell
root@localhost:~# ipvsadm --help
ipvsadm v1.28 2015/02/09 (compiled with popt and IPVS v1.2.1)
Usage:
  ipvsadm -A|E virtual-service [-s scheduler] [-p [timeout]] [-M netmask] [--pe persistence_engine] [-b sched-flags]
  ipvsadm -D virtual-service
  ipvsadm -C
  ipvsadm -R
  ipvsadm -S [-n]
  ipvsadm -a|e virtual-service -r server-address [options]
  ipvsadm -d virtual-service -r server-address
  ipvsadm -L|l [virtual-service] [options]
  ipvsadm -Z [virtual-service]
  ipvsadm --set tcp tcpfin udp
  ipvsadm --start-daemon state [--mcast-interface interface] [--syncid sid]
  ipvsadm --stop-daemon state
  ipvsadm -h

Commands:
Either long or short options are allowed.
  --add-service     -A        add virtual service with options
  --edit-service    -E        edit virtual service with options
  --delete-service  -D        delete virtual service
  --clear           -C        clear the whole table
  --restore         -R        restore rules from stdin
  --save            -S        save rules to stdout
  --add-server      -a        add real server with options
  --edit-server     -e        edit real server with options
  --delete-server   -d        delete real server
  --list            -L|-l     list the table
  --zero            -Z        zero counters in a service or all services
  --set tcp tcpfin udp        set connection timeout values
  --start-daemon              start connection sync daemon
  --stop-daemon               stop connection sync daemon
  --help            -h        display this help message

virtual-service:
  --tcp-service|-t  service-address   service-address is host[:port]
  --udp-service|-u  service-address   service-address is host[:port]
  --sctp-service    service-address   service-address is host[:port]
  --fwmark-service|-f fwmark          fwmark is an integer greater than zero

Options:
  --ipv6         -6                   fwmark entry uses IPv6
  --scheduler    -s scheduler         one of rr|wrr|lc|wlc|lblc|lblcr|dh|sh|sed|nq,
                                      the default scheduler is wlc.
  --pe            engine              alternate persistence engine may be sip,
                                      not set by default.
  --persistent   -p [timeout]         persistent service
  --netmask      -M netmask           persistent granularity mask
  --real-server  -r server-address    server-address is host (and port)
  --gatewaying   -g                   gatewaying (direct routing) (default)
  --ipip         -i                   ipip encapsulation (tunneling)
  --masquerading -m                   masquerading (NAT)
  --weight       -w weight            capacity of real server
  --u-threshold  -x uthreshold        upper threshold of connections
  --l-threshold  -y lthreshold        lower threshold of connections
  --mcast-interface interface         multicast interface for connection sync
  --syncid sid                        syncid for connection sync (default=255)
  --connection   -c                   output of current IPVS connections
  --timeout                           output of timeout (tcp tcpfin udp)
  --daemon                            output of daemon information
  --stats                             output of statistics information
  --rate                              output of rate information
  --exact                             expand numbers (display exact values)
  --thresholds                        output of thresholds information
  --persistent-conn                   output of persistent connection info
  --nosort                            disable sorting output of service/server entries
  --sort                              does nothing, for backwards compatibility
  --ops          -o                   one-packet scheduling
  --numeric      -n                   numeric output of addresses and ports
  --sched-flags  -b flags             scheduler flags (comma-separated)

```



### 5. Kubernetes和IPVS

Kube-proxy早期一直使用iptables作为其后端的实现机制，但随着kubernetes可支持的规模的不断扩大（5000+），作为为防火墙而设计且基于内核规则列表的iptables显然会出现性能瓶颈。

举例来说，在一个5000节点的集群中全部使用NodePort类型的的service，如果我们创建2000个services，每个service对应10个pods，这将使每个节点上生成至少20000条记录，从而导致内核相当繁忙。

相对而言，使用基于IPVS的负载均衡就要好很多了。IPVS是专门为负载均衡而设计的，并使用了更高效的数据结构（hash tables），从而在底层几乎具有无限的可扩展性。



#### 5.1 开启IPVS

1. 各节点加载ip_vs内核模块，模块列表如下：

```shell
ip_vs
ip_vs_rr
ip_vs_wrr
ip_vs_sh
nf_conntrack_ipv4
```

> 如果这些内核模块不加载，当kube-proxy会退回到iptables模式

2. 各节点安装软件包：ipset，ipvsadm

3. 配置`--proxy-mode=ipvs`开启IPVS，Kubernetes 1.15.3的kube-proxy已经默认开启IPVS



#### 5.2 IPVS转发模式选择

kubernetes要求能够进行端口映射，因此只能选择NAT模式。



#### 5.3 负载均衡算法选择

通过参数`--ipvs-scheduler`配置负载均衡算法，默认为轮询（Round Robin），具体参数值如下：

- rr:：round robin
- lc：least connection
- dh：destination hashing
- sh：source hashing
- sed：shortest expected delay
- nq：never queue



#### 5.4 IPVS创建virtual server步骤

1. 在节点上创建dummy 网卡，默认为kube-ipvs0

```shell
10: kube-ipvs0: <BROADCAST,NOARP> mtu 1500 qdisc noop state DOWN group default 
    link/ether 7e:2f:8c:4c:b4:b9 brd ff:ff:ff:ff:ff:ff
    inet 10.10.72.208/32 brd 10.10.72.208 scope global kube-ipvs0
       valid_lft forever preferred_lft forever
```

2. 将Service IP绑定到kube-ipvs0上

```shell
10: kube-ipvs0: <BROADCAST,NOARP> mtu 1500 qdisc noop state DOWN group default 
    link/ether 7e:2f:8c:4c:b4:b9 brd ff:ff:ff:ff:ff:ff
    inet 10.10.72.208/32 brd 10.10.72.208 scope global kube-ipvs0
       valid_lft forever preferred_lft forever
```

3. 为每个Service IP单独创建virtual servers

```shell
TCP  10.10.72.208:443 rr
  -> 10.244.136.55:443            Masq    1      0          0         
  -> 10.244.137.76:443            Masq    1      0          0         
  -> 10.244.180.11:443            Masq    1      0          0        
```



#### 5.4 IPVS常用调优方法

基于ubuntu 18.04环境

1. 调整ulimit

   > 修改文件：/etc/security/limits.conf

```shell
*               soft    nofile          655350
*               hard    nofile          655350
*               soft    nproc           655350
*               hard    nproc           655350
root            soft    nofile          655350
root            hard    nofile          655350
root            soft    nproc           655350
root            hard    nproc           655350
```

2. 调整sysctl.conf

   > 修改文件：/etc/sysctl.conf

```shell
net.ipv4.ip_forward = 0
net.ipv4.conf.default.rp_filter = 1
net.ipv4.conf.default.accept_source_route = 0
kernel.sysrq = 0
kernel.core_uses_pid = 1
net.ipv4.tcp_syncookies = 1
kernel.msgmnb = 65536
kernel.msgmax = 65536
kernel.shmmax = 68719476736
kernel.shmall = 4294967296
net.ipv4.tcp_max_tw_buckets = 6000
net.ipv4.tcp_sack = 1
net.ipv4.tcp_window_scaling = 1
net.ipv4.tcp_rmem = 4096 87380 4194304
net.ipv4.tcp_wmem = 4096 16384 4194304
net.core.wmem_default = 8388608
net.core.rmem_default = 8388608
net.core.rmem_max = 16777216
net.core.wmem_max = 16777216
net.core.netdev_max_backlog = 262144
net.core.somaxconn = 262144
net.ipv4.tcp_max_orphans = 3276800
net.ipv4.tcp_max_syn_backlog = 262144
net.ipv4.tcp_timestamps = 0
net.ipv4.tcp_synack_retries = 1
net.ipv4.tcp_syn_retries = 1
net.ipv4.tcp_tw_recycle = 1
net.ipv4.tcp_tw_reuse = 1
net.ipv4.tcp_mem = 94500000 915000000 927000000
net.ipv4.tcp_fin_timeout = 1
net.ipv4.tcp_keepalive_time = 30
net.ipv4.ip_local_port_range = 1024 65000
```

3. 调整system.conf

   > 修改文件：/etc/systemd/system.conf

```shell
DefaultLimitNOFILE=infinity
DefaultLimitNPROC=infinity
DefaultTasksMax=infinity
```

4. 调整ip_vs option（默认为4096）

   > 修改文件：/etc/modprobe.conf

```shell
options ip_vs conn_tab_bits=20
```

5. 重启机器



#### 5.5 通过Cluster IP访问Pod的流量转发工作原理

kubernetes里的ipvs使用NAT模式，但ipvs不能实现如包过滤、源地址转换、目标地址转换、masquared转换等能力，这些还需要iptables来处理。



**iptables**

iptables是Linux防火墙的管理工具，位于/sbin/iptables，真正实现防火墙功能的是netfilter，它是Linux内核中实现包过滤的内部结构。

在说明通过Cluster IP访问Pod时的流量转发之前，先熟悉下iptables的数据包传输过程：

1. 数据包进入网卡时，它首先进入PREROUTING链，内核根据数据包目的IP判断是否需要转送出去;
2. 如果数据包就是进入本机的，它会到达INPUT链。数据包到了INPUT链后，任何进程都会收到它。本机上运行的程序也可以发送数据包，这些数据包会经过OUTPUT链，然后到达POSTROUTING链输出；
3. 如果数据包是要转发出去的，且内核允许转发，数据包经过FORWARD链，然后到达POSTROUTING链输出；

其中，规则链如下：

1. INPUT：进来的数据包应用此规则链中的策略
2. OUTPUT：外出的数据包应用此规则链中的策略
3. FORWARD：转发数据包时应用此规则链中的策略
4. PREROUTING：对数据包作路由选择前应用此规则链中的策略
5. POSTROUTING：对数据包作路由选择后应用此规则链中的策略

其中，规则链中的策略，即规则表中的规则，规则表如下：

1. filter表，作用于INPUT、FORWARD、OUTPUT，用于过滤数据包 
2. Nat表，作用于PREROUTING、POSTROUTING、OUTPUT，用于网络地址转换
3. Mangle表，作用于PREROUTING、POSTROUTING、INPUT、OUTPUT、FORWARD，用于修改数据包的服务类型
4. Raw表，作用于OUTPUT、PREROUTING，用于决定数据包是否被状态跟踪机制处理



**通过Cluster IP访问Pod时的流量转发工作过程**

1. 外部流量匹配

```shell
root@localhost:~# iptables -S -tnat | grep OUTPUT
-P OUTPUT ACCEPT
-A OUTPUT -m comment --comment "kubernetes service portals" -j KUBE-SERVICES
```

2. 将流量引入到KUBE-SERVICES规则中处理

```shell
root@localhost:~# iptables -S -tnat | grep KUBE-SERVICES
-N KUBE-SERVICES
-A PREROUTING -m comment --comment "kubernetes service portals" -j KUBE-SERVICES
-A OUTPUT -m comment --comment "kubernetes service portals" -j KUBE-SERVICES
-A KUBE-SERVICES -m comment --comment "Kubernetes service cluster ip + port for masquerade purpose" -m set --match-set KUBE-CLUSTER-IP src,dst -j KUBE-MARK-MASQ
-A KUBE-SERVICES -m addrtype --dst-type LOCAL -j KUBE-NODE-PORT
-A KUBE-SERVICES -m set --match-set KUBE-CLUSTER-IP dst,dst -j ACCEPT
```

```shell
root@localhost:~# ipset list KUBE-CLUSTER-IP
Name: KUBE-CLUSTER-IP
Type: hash:ip,port
Revision: 5
Header: family inet hashsize 1024 maxelem 65536
Size in memory: 7552
References: 2
Members:
10.10.79.163,tcp:8888
10.10.155.215,tcp:8080
10.10.103.103,tcp:3306
10.10.151.229,tcp:8000
10.10.67.231,tcp:9200
```

3. 将KUBE-SERVICES规则中的流量进行打标签处理

```shell
root@localhost:~# iptables -S -tnat | grep KUBE-MARK-MASQ
-N KUBE-MARK-MASQ
-A KUBE-LOAD-BALANCER -j KUBE-MARK-MASQ
-A KUBE-MARK-MASQ -j MARK --set-xmark 0x4000/0x4000
-A KUBE-NODE-PORT -p tcp -m comment --comment "Kubernetes nodeport TCP port for masquerade purpose" -m set --match-set KUBE-NODE-PORT-TCP dst -j KUBE-MARK-MASQ
-A KUBE-SERVICES -m comment --comment "Kubernetes service cluster ip + port for masquerade purpose" -m set --match-set KUBE-CLUSTER-IP src,dst -j KUBE-MARK-MASQ
```

4. 将所有外部流量的源地址转换成该Cluster IP

```shell
root@localhost:~# iptables -S -tnat | grep POSTROUTING
-P POSTROUTING ACCEPT
-N KUBE-POSTROUTING
-A POSTROUTING -m comment --comment "kubernetes postrouting rules" -j KUBE-POSTROUTING
-A KUBE-POSTROUTING -m comment --comment "kubernetes service traffic requiring SNAT" -m mark --mark 0x4000/0x4000 -j MASQUERADE
-A KUBE-POSTROUTING -m comment --comment "Kubernetes endpoints dst ip:port, source ip for solving hairpin purpose" -m set --match-set KUBE-LOOP-BACK dst,dst,src -j MASQUERADE
```

5. 通过ipvs将流量转发到后端pod上

```shell
TCP  10.10.72.208:443 rr
  -> 10.244.136.55:443            Masq    1      0          0         
  -> 10.244.137.76:443            Masq    1      0          0         
  -> 10.244.180.11:443            Masq    1      0          0     
```



### 参考资料

1. IPVS从入门到精通kube-proxy实现原理：https://mp.weixin.qq.com/s?__biz=MzA5OTAyNzQ2OA==&mid=2649703848&idx=1&sn=863b60fdd8f33dec7c3154f3539df8ca&chksm=88937acbbfe4f3dd6d199af8cb77ee2058291a9e83de8af37719d8bb1014a92f1f78ffe3438e&mpshare=1&scene=1&srcid=&sharer_sharetime=1575940520892&sharer_shareid=0723e5d1a7c981d460238a9c9d97e0d8#rd
2. 浅析kube-proxy中的IPVS模式：https://cloud.tencent.com/developer/article/1477638
3. Linux负载均衡--LVS（IPVS）：https://www.jianshu.com/p/36880b085265
4. k8s实践3:ipvs结合iptables使用过程分析：https://blog.51cto.com/goome/2369150
5. k8s集群中ipvs负载详解：https://www.jianshu.com/p/89f126b241db?utm_campaign=maleskine&utm_content=note&utm_medium=seo_notes&utm_source=recommendation