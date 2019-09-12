# Etcd源码分析（一）Raft协议





分布式应用，共识：

- 分布式应用共识问题的完整描述

- 现存解决方案，优劣比较



什么是分布式系统一致性，为什么需要分布式一致性

分布式一致性面临的问题：

- 节点失效

- 通信受到干扰/阻断

- 运行速度差异
- 等等



CAP定理、FLP不可能定理



非拜占庭将军容错问题

paxos、raft、zab



拜占庭将军容错问题

POW、POS 和 DPOS

Ripple协议，Tendermint协议，Stellar



思路：

> why

问题背景

其他解决方案、优劣

> what

该方案是什么、优劣、适用/不适用场景

> how

该方案如何工作

实验评估



raft适用场景

常用数据库的分布式数据一致性算法



replicated state machine（复制状态机）：

如果各节点的初始状态一致，每个节点都执行相同的操作序列，那么他们最后能得到一个一致的状态

同步操作序列log，为何不直接同步state？



可理解性：

1. 问题分解：领导选取（leader election）、日志复制（log replication）、安全（safety）和成员变化（membership changes）
2. 状态空间简化：减少需要考虑的状态的数量



日志复制：

- appendEntries

一致性检查

NextIndex



Raft一致性算法（5大原则）

1. 选举安全原则（Election Safety）：

一个任期（term）内最多允许有一个领导人被选上



*收到了来自集群中大多数服务器的投票才会赢得选举*

*在一个任期内，一台服务器最多能给一个候选人投票*

*先到先服务原则（first-come-first-served）*

*阻止没有包含全部日志条目的服务器赢得选举*



2. 领导人只增加原则（Leader Append-Only）：

领导人永远不会覆盖或者删除自己的日志，它只会增加条目



*阻止没有包含全部日志条目的服务器赢得选举*



3. 日志匹配原则（Log Matching Property）:

如果在不同日志中的两个条目有着相同的索引和任期号，则它们所存储的命令是相同的

如果在不同日志中的两个条目有着相同的索引和任期号，则它们之间的所有条目都是完全一样的。



4. 领导人完全原则（Leader Completeness)：

如果一个日志条目在一个给定任期内被提交，那么这个条目一定会出现在所有任期号更大的领导人中



5. 状态机安全原则（State Machine Safety）：

如果一台服务器将给定索引上的日志条目应用到了它自己的状态机上，则所有其他服务器不会在该索引位置应用不同的条目









































参考资料：

1. Raft 一致性算法论文译文：https://www.infoq.cn/article/raft-paper
2. 共识算法：Raft：https://www.jianshu.com/p/8e4bbe7e276c
3. CoreOS 实战：剖析 etcd：https://www.infoq.cn/article/coreos-analyse-etcd/
4. 微信 PaxosStore：深入浅出 Paxos 算法协议：https://www.infoq.cn/article/wechat-paxosstore-paxos-algorithm-protocol?utm_source=related_read&utm_medium=article
5. 谈谈分布式系统中的复制：http://www.voidcn.com/article/p-sfcgwsjt-zg.html
6. Bully算法：http://www.distorage.com/%E5%88%86%E5%B8%83%E5%BC%8F%E7%B3%BB%E7%BB%9F%E6%8A%80%E6%9C%AF%E7%B3%BB%E5%88%97-%E9%80%89%E4%B8%BB%E7%AE%97%E6%B3%95/
7. 分布式一致性与共识算法：https://draveness.me/consensus
8. membership：https://zhuanlan.zhihu.com/p/29678067