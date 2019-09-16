# Etcd源码分析（一）Raft协议

本文是Etcd源码分析系列文章的第一篇，将重点从理论上分析Etcd的一致性算法 - Raft。



## 1. 分布式基础理论

当前，微服务架构日益普遍，虽然大多数微服务本身是无状态的，但是它们通常会操作一些含有数据的分布式系统，如数据库、缓存等。在本小节中，将先简单回顾一下分布式理论的一些知识点。



### 1.1 分布式基础

**分布式系统**

分布式系统由多个不同的服务节点组成，节点与节点之间通过消息进行通信和协调。根据消息机制的不同，分布式系统的运行模型可以分为异步模型系统 和同步模型系统。



**分布式系统的一致性**

在一个分布式系统中，保证集群中所有节点中的数据完全相同，并且能够对某个提案达成一致，是分布式系统正常工作的核心。

但由于引入了多个节点，分布式系统中常会出现各种各样非常复杂的情况，包括节点宕机、通信受到干扰/阻断、节点间运行速度存在差异等等，即当多个节点通过异步通讯方式组成网络集群时，这种异步网络默认是不可靠的，那么，如何保证这些不可靠节点的状态最终能达成相同一致的状态，此即*分布式系统的一致性问题*，而解决此类问题的算法即为*共识算法*。



### 1.2 分布式理论

**ACID原则**

- 原子性（Atomicity）：每次操作是原子的
- 一致性（Consistency）：数据库的状态是一致的
- 隔离性（Isolation）：各种操作彼此之间互相不影响
- 持久性（Durability）：状态的改变是持久的

> ACID是传统数据库常用的设计理念，追求强一致性。



**BASE理论**

- 基本可用（Basically Available）：指分布式系统在出现故障的时候，允许损失部分可用性
- 软状态（Soft State）：允许系统存在中间状态，而该中间状态不会影响系统整体可用性
- 最终一致性（Eventual Consistency）：系统中的所有数据副本经过一定时间后，最终能够达到一致的状态



**CAP理论**

一个分布式系统最多只能同时满足一致性（Consistency）、可用性（Availability）和分区容忍性（Partition tolerance）这三项中的两项，其中：

- 一致性（Consistency）：强一致性，所有节点在同一时间的数据完全一致
- 可用性（Availability）：分布式系统可以在正常响应的时间内提供相应的服务
- 分区容忍性（Partition tolerance）：在遇到某节点或网络分区故障的时候，仍然能够对外提供满足一致性和可用性的服务

> CAP原理最早是2000年由Eric Brewer在ACM组织的一个研讨会上提出猜想，后来Lynch等人进行了证明，该原理被认为是分布式系统领域的重要原理之一。



**FLP不可能原理**

在网络可靠，但允许节点失效（即便只有一个）的最小化异步模型系统中，不存在一个可以解决一致性问题的确定性共识算法。

这个定理告诉我们，不要浪费时间去为异步分布式系统设计在任意场景上都能够实现共识的算法，异步系统完全没有办法保证能在有限时间内达成一致。

> 该原理见于由Fischer、Lynch和Patterson三位科学家于1985年发表的论文《Impossibility of Distributed Consensus with One Faulty Process》，该定理被认为是分布式系统中重要的原理之一。



### 1.3 共识算法

**拜占庭将军问题**

拜占庭将军问题是 Leslie Lamport 在《 The Byzantine Generals Problem》 论文中提出的分布式领域的容错问题，它是分布式领域中最复杂、最严格的容错模型。

在该模型下，系统不会对集群中的节点做任何的限制，它们可以向其他节点发送随机数据、错误数据，也可以选择不响应其他节点的请求，这些无法预测的行为使得容错这一问题变得非常复杂。



**解决非拜占庭将军容错的一致性问题**

拜占庭将军问题是对分布式系统容错的最高要求，但在日常工作中使用的大多数分布式系统中不会面对所有这些复杂的问题，我们遇到更多的还是节点故障、宕机或者不响应等情况，这就大大简化了系统对容错的要求，解决此类问题的常见算法：

- paxos

- raft
- zab

> Leslie Lamport 提出的 Paxos 可以在没有恶意节点的前提下保证系统中节点的一致性，也是第一个被证明完备的共识算法



**解决拜占庭将军容错的一致性问题**

解决此类问题的常见算法：

- pow
- pos
- dpos



### 1.4 复制状态机

复制状态机（replicated state machine）通常是基于复制日志实现的，每一个节点存储一个包含一系列指令的日志，并且按照日志的顺序进行执行。

由于每一个日志都按照相同的顺序包含相同的指令，所以每一个节点都执行相同的指令序列后，都会产生相同的状态，即，如果各节点的初始状态一致，每个节点都执行相同的操作序列，那么他们最后能得到一个一致的状态。

而保证复制日志相同就是共识算法的工作了。



## 2. Raft算法

Raft ，是一种用来管理日志复制的一致性算法。

> 论文原文：[ In search of an Understandable Consensus Algorithm (Extended Version)](https://ramcloud.atlassian.net/wiki/download/attachments/6586375/raft.pdf)

> 论文翻译：[Raft 一致性算法论文译文](https://www.infoq.cn/article/raft-paper)。



Raft算法在论文中有详细的描述，建议阅读，本小节只针对算法中的关键点作部分说明。



### 2.1 Paxos与Raft

早在 1990 年，Leslie Lamport向 ACM Transactions on Computer Systems (TOCS) 提交了关于 Paxos 算法的论文*The Part-Time Parliament*。但Paxos 很难理解，而且，Paxos 需要经过复杂的修改才能应用于实际中。实际上，目前工程实践中所有声称基于Paxos实现的系统都非真正的Paxos系统。

Raft是一种在可理解性上更容易的一种一致性算法。可理解性是作者非常强调的一点，引用作者在论文中的结语：

> 算法的设计通常会把正确性，效率或者简洁作为主要的目标。尽管这些都是很有意义的目标，但是我们相信，可理解性也是一样的重要。在开发者把算法应用到实际的系统中之前，这些目标没有一个会被实现，这些都会必然的偏离发表时的形式。除非开发人员对这个算法有着很深的理解并且有着直观的感觉，否则将会对他们而言很难在实现的时候保持原有期望的特性。



### 2.2 Raft的可理解性设计

为使得大多数人能够很容易理解，Raft在设计上采用了一下两种方式：

1. 问题分解：将问题分解成为若干个可解决的、可被理解的小问题。在 Raft 中，把问题分解成为了领导选取（leader election）、日志复制（log replication）、安全（safety）和成员变化（membership changes）。
2. 状态空间简化：减少需要考虑的状态的数量。在 Raft 中，使用随机化选举超时来简化了领导选取算法，随机化方法使得不确定性增加，但是它减少了状态空间。



### 2.3 保证Raft算法一致性的原则

**选举安全原则（Election Safety）：**

一个任期（term）内最多允许有一个领导人被选上



**领导人只增加原则（Leader Append-Only）：**

领导人永远不会覆盖或者删除自己的日志，它只会增加条目



**日志匹配原则（Log Matching Property）:**

如果在不同日志中的两个条目有着相同的索引和任期号，则它们所存储的命令是相同的

如果在不同日志中的两个条目有着相同的索引和任期号，则它们之间的所有条目都是完全一样的。



**领导人完全原则（Leader Completeness)：**

如果一个日志条目在一个给定任期内被提交，那么这个条目一定会出现在所有任期号更大的领导人中



**状态机安全原则（State Machine Safety）：**

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
9. 分布式系统中的FLP不可能原理、CAP理论与BASE理论：https://zhuanlan.zhihu.com/p/35608244