# Etcd源码分析（二）Etcd

本文是Etcd源码分析系列文章的第二篇，将重点梳理Etcd的源码实现。由于已经有较多文章在这方面做过分析，所以本文只做部分引导，建议同步阅读参考资料中的优秀文章以扩充认识。



> 以下代码分析基于 ***etcd v3.4.0*** 版本



Eetcd源码的主函数入口位于`go.etcd.io/etcd/main.go`，代码目录结构如下：

```shell
├── auth
├── client
├── clientv3
├── embed
├── etcdctl
├── etcdmain
|   ├── config.go
|   ├── config_test.go
|   ├── doc.go
|   ├── etcd.go
|   ├── gateway.go
|   ├── grpc_proxy.go
|   ├── help.go
|   ├── main.go // etcd主服务入口
|   └── util.go
├── etcdserver
├── functional
├── hack
├── integration
├── lease
├── main.go // 主函数入口
├── mvcc
├── pkg
├── Procfile
├── Procfile.learner
├── Procfile.v2
├── proxy
├── raft
├── security
├── tools
├── vendor
├── version
└── wal
```



### Etcd启动基本流程

主函数

> 代码路径：main.go

```go
package main

import "go.etcd.io/etcd/etcdmain"

func main() {
	etcdmain.Main() // 主函数
}
```

```go
func Main() {
    // 检查支持的CPU架构
	checkSupportArch()
    ...

	// etcd
	startEtcdOrProxyV2()
}
```

启动etcdserver

> 代码路径：etcdmain/etcd.go

```go
// startEtcd runs StartEtcd in addition to hooks needed for standalone etcd.
func startEtcd(cfg *embed.Config) (<-chan struct{}, <-chan error, error) {
	// 启动etcdserver
	e, err := embed.StartEtcd(cfg) //启动etcd
	if err != nil {
		return nil, nil, err
	}
	osutil.RegisterInterruptHandler(e.Close)
	select {
	case <-e.Server.ReadyNotify(): // wait for e.Server to join the cluster
	case <-e.Server.StopNotify(): // publish aborted from 'ErrStopped'
	}
	return e.Server.StopNotify(), e.Err(), nil
}
```

```go
// StartEtcd launches the etcd server and HTTP handlers for client/server communication.
// The returned Etcd.Server is not guaranteed to have joined the cluster. Wait
// on the Etcd.Server.ReadyNotify() channel to know when it completes and is ready for use.
func StartEtcd(inCfg *Config) (e *Etcd, err error) {
	// 校验config
	if err = inCfg.Validate(); err != nil {
		return nil, err
	}

	serving := false

	// 构建etcd数据结构体
	// Peers   []*peerListener
	// Clients []net.Listener
	// sctxs            map[string]*serveCtx
	// metricsListeners []net.Listener
	// Server *etcdserver.EtcdServer
	// cfg   Config
	// stopc chan struct{}
	// errc  chan error
	// closeOnce sync.Once
	e = &Etcd{cfg: *inCfg, stopc: make(chan struct{})}
	cfg := &e.cfg

	defer func() {
		if e == nil || err == nil {
			return
		}
		if !serving {
			// errored before starting gRPC server for serveCtx.serversC
			for _, sctx := range e.sctxs {
				close(sctx.serversC)
			}
		}
		e.Close()
		e = nil
	}()

	// 配置要监听的集群中所有其他成员的的URLs
	if e.cfg.logger != nil {
		e.cfg.logger.Info(
			"configuring peer listeners",
			zap.Strings("listen-peer-urls", e.cfg.getLPURLs()),
		)
	}
	if e.Peers, err = configurePeerListeners(cfg); err != nil {
		return e, err
	}

	if e.cfg.logger != nil {
		e.cfg.logger.Info(
			"configuring client listeners",
			zap.Strings("listen-client-urls", e.cfg.getLCURLs()),
		)
	}
	// 配置要监听的集群外所有客户端的的URLs
	if e.sctxs, err = configureClientListeners(cfg); err != nil {
		return e, err
	}

	// 罗列 监听的客户端URLs
	for _, sctx := range e.sctxs {
		e.Clients = append(e.Clients, sctx.l)
	}

	var (
		urlsmap types.URLsMap
		token   string
	)

	// 通过是wal目录下否存在WAL文件，判断该成员是否已被初始化过
	memberInitialized := true
	if !isMemberInitialized(cfg) {
		memberInitialized = false
		// urlsmap包含了初始化集群成员的URLs
		urlsmap, token, err = cfg.PeerURLsMapAndToken("etcd")
		if err != nil {
			return e, fmt.Errorf("error setting up initial cluster: %v", err)
		}
	}

	// 压缩，默认为0，分为两种模式
	// 第一种模式，周期性压缩模式，时间间隔cfg.AutoCompactionRetention*小时
	// 第二种模式，revision模式，每隔5min执行一次
	// AutoCompactionRetention defaults to "0" if not set.
	if len(cfg.AutoCompactionRetention) == 0 {
		cfg.AutoCompactionRetention = "0"
	}
	// 返回时间周期，或者revision大小
	autoCompactionRetention, err := parseCompactionRetention(cfg.AutoCompactionMode, cfg.AutoCompactionRetention)
	if err != nil {
		return e, err
	}

	// 备份
	backendFreelistType := parseBackendFreelistType(cfg.ExperimentalBackendFreelistType)

	// 构建etcdserver
	// embed/config.go中Config对部分字段的作用和意义有详细解释
	// etcdserver/config.go中ServerConfig对部分字段的作用和意义有详细解释
	srvcfg := etcdserver.ServerConfig{
		// 成员
		Name:       cfg.Name,
		ClientURLs: cfg.ACUrls, // advertise client urls
		PeerURLs:   cfg.APUrls, // advertise peer urls
		// 数据
		DataDir:         cfg.Dir,
		DedicatedWALDir: cfg.WalDir,
		// 快照
		SnapshotCount:          cfg.SnapshotCount,
		SnapshotCatchUpEntries: cfg.SnapshotCatchUpEntries,
		MaxSnapFiles:           cfg.MaxSnapFiles,
		MaxWALFiles:            cfg.MaxWalFiles,
		// 集群初始化
		InitialPeerURLsMap:  urlsmap,
		InitialClusterToken: token,
		DiscoveryURL:        cfg.Durl,
		DiscoveryProxy:      cfg.Dproxy,
		NewCluster:          cfg.IsNewCluster(),
		PeerTLSInfo:         cfg.PeerTLSInfo,
		// 选举
		TickMs:                     cfg.TickMs,
		ElectionTicks:              cfg.ElectionTicks(),
		InitialElectionTickAdvance: cfg.InitialElectionTickAdvance,
		// 压缩
		AutoCompactionRetention: autoCompactionRetention,
		AutoCompactionMode:      cfg.AutoCompactionMode,
		CompactionBatchLimit:    cfg.ExperimentalCompactionBatchLimit,
		// 备份
		QuotaBackendBytes:    cfg.QuotaBackendBytes,
		BackendBatchLimit:    cfg.BackendBatchLimit,
		BackendFreelistType:  backendFreelistType,
		BackendBatchInterval: cfg.BackendBatchInterval,
		// 请求连接
		MaxTxnOps:           cfg.MaxTxnOps,
		MaxRequestBytes:     cfg.MaxRequestBytes,
		StrictReconfigCheck: cfg.StrictReconfigCheck,
		// 请求认证
		ClientCertAuthEnabled: cfg.ClientTLSInfo.ClientCertAuth,
		AuthToken:             cfg.AuthToken,
		BcryptCost:            cfg.BcryptCost,
		CORS:                  cfg.CORS,
		HostWhitelist:         cfg.HostWhitelist,
		InitialCorruptCheck:   cfg.ExperimentalInitialCorruptCheck,
		CorruptCheckTime:      cfg.ExperimentalCorruptCheckTime,
		PreVote:               cfg.PreVote,
		// 运行日志
		Logger:                cfg.logger,
		LoggerConfig:          cfg.loggerConfig,
		LoggerCore:            cfg.loggerCore,
		LoggerWriteSyncer:     cfg.loggerWriteSyncer,
		Debug:                 cfg.Debug,
		ForceNewCluster:       cfg.ForceNewCluster,
		EnableGRPCGateway:     cfg.EnableGRPCGateway,
		EnableLeaseCheckpoint: cfg.ExperimentalEnableLeaseCheckpoint,
	}
	print(e.cfg.logger, *cfg, srvcfg, memberInitialized)
	//New etcdserver、
	if e.Server, err = etcdserver.NewServer(srvcfg); err != nil {
		return e, err
	}

	// buffer channel so goroutines on closed connections won't wait forever
	e.errc = make(chan error, len(e.Peers)+len(e.Clients)+2*len(e.sctxs))

	// newly started member ("memberInitialized==false")
	// does not need corruption check
	if memberInitialized {
		if err = e.Server.CheckInitialHashKV(); err != nil {
			// set "EtcdServer" to nil, so that it does not block on "EtcdServer.Close()"
			// (nothing to close since rafthttp transports have not been started)
			e.Server = nil
			return e, err
		}
	}

	// 启动etcdserver
	e.Server.Start()

	// 启动peer/client/metrics三类监听服务，当有请求时，执行对应的Handler
	if err = e.servePeers(); err != nil {
		return e, err
	}
	if err = e.serveClients(); err != nil {
		return e, err
	}
	if err = e.serveMetrics(); err != nil {
		return e, err
	}

	if e.cfg.logger != nil {
		e.cfg.logger.Info(
			"now serving peer/client/metrics",
			zap.String("local-member-id", e.Server.ID().String()),
			zap.Strings("initial-advertise-peer-urls", e.cfg.getAPURLs()),
			zap.Strings("listen-peer-urls", e.cfg.getLPURLs()),
			zap.Strings("advertise-client-urls", e.cfg.getACURLs()),
			zap.Strings("listen-client-urls", e.cfg.getLCURLs()),
			zap.Strings("listen-metrics-urls", e.cfg.getMetricsURLs()),
		)
	}
	serving = true
	return e, nil
}
```

> 代码路径：etcdserver/server.go

```go
// Start performs any initialization of the Server necessary for it to
// begin serving requests. It must be called before Do or Process.
// Start must be non-blocking; any long-running server functionality
// should be implemented in goroutines.
// 运行etcdserver
func (s *EtcdServer) Start() {
	s.start()
	s.goAttach(func() { s.adjustTicks() })
	s.goAttach(func() { s.publish(s.Cfg.ReqTimeout()) })
	s.goAttach(s.purgeFile)
	s.goAttach(func() { monitorFileDescriptor(s.getLogger(), s.stopping) })
	s.goAttach(s.monitorVersions)
	// 线性一致性读，保证对客户端的读请求的响应的数据的一致性
	s.goAttach(s.linearizableReadLoop)
	s.goAttach(s.monitorKVHash)
}
```

```go
// start prepares and starts server in a new goroutine. It is no longer safe to
// modify a server's fields after it has been sent to Start.
// This function is just used for testing.
// 启动etcdserver
func (s *EtcdServer) start() {
	lg := s.getLogger()

	// 快照
	if s.Cfg.SnapshotCount == 0 {
		if lg != nil {
			lg.Info(
				"updating snapshot-count to default",
				zap.Uint64("given-snapshot-count", s.Cfg.SnapshotCount),
				zap.Uint64("updated-snapshot-count", DefaultSnapshotCount),
			)
		} else {
			plog.Infof("set snapshot count to default %d", DefaultSnapshotCount)
		}
		// 如果没有设置，取默认值：100,000
		s.Cfg.SnapshotCount = DefaultSnapshotCount
	}
	if s.Cfg.SnapshotCatchUpEntries == 0 {
		if lg != nil {
			lg.Info(
				"updating snapshot catch-up entries to default",
				zap.Uint64("given-snapshot-catchup-entries", s.Cfg.SnapshotCatchUpEntries),
				zap.Uint64("updated-snapshot-catchup-entries", DefaultSnapshotCatchUpEntries),
			)
		}
		// 如果没有设置，取默认值：5000
		// 对于有延迟的Follower（一般认为延迟在毫秒之间，产生的日志条目在10K以内），日志条目与领导者相差大于该值，需要完全同步领导者的日志快照
		s.Cfg.SnapshotCatchUpEntries = DefaultSnapshotCatchUpEntries
	}

	// 创建通信通道
	s.w = wait.New()
	s.applyWait = wait.NewTimeList()
	s.done = make(chan struct{})
	s.stop = make(chan struct{})
	s.stopping = make(chan struct{})
	s.ctx, s.cancel = context.WithCancel(context.Background())
	s.readwaitc = make(chan struct{}, 1)
	s.readNotifier = newNotifier()
	s.leaderChanged = make(chan struct{})

	if s.ClusterVersion() != nil {
		if lg != nil {
			lg.Info(
				"starting etcd server",
				zap.String("local-member-id", s.ID().String()),
				zap.String("local-server-version", version.Version),
				zap.String("cluster-id", s.Cluster().ID().String()),
				zap.String("cluster-version", version.Cluster(s.ClusterVersion().String())),
			)
		} else {
			plog.Infof("starting server... [version: %v, cluster version: %v]", version.Version, version.Cluster(s.ClusterVersion().String()))
		}
		membership.ClusterVersionMetrics.With(prometheus.Labels{"cluster_version": s.ClusterVersion().String()}).Set(1)
	} else {
		if lg != nil {
			lg.Info(
				"starting etcd server",
				zap.String("local-member-id", s.ID().String()),
				zap.String("local-server-version", version.Version),
				zap.String("cluster-version", "to_be_decided"),
			)
		} else {
			plog.Infof("starting server... [version: %v, cluster version: to_be_decided]", version.Version)
		}
	}

	// TODO: if this is an empty log, writes all peer infos
	// into the first entry
	// 运行etcdserver
	go s.run()
}
```

重要数据结构

Etcd数据结构

```go
// Etcd contains a running etcd server and its listeners.
type Etcd struct {
	Peers   []*peerListener
	Clients []net.Listener
	// a map of contexts for the servers that serves client requests.
	sctxs            map[string]*serveCtx
	metricsListeners []net.Listener

	Server *etcdserver.EtcdServer

	cfg   Config // embed Config
	stopc chan struct{}
	errc  chan error

	closeOnce sync.Once
}
```

EtcdServer 数据结构

```go
// EtcdServer is the production implementation of the Server interface
type EtcdServer struct {
	// inflightSnapshots holds count the number of snapshots currently inflight.
	inflightSnapshots int64  // must use atomic operations to access; keep 64-bit aligned.
	appliedIndex      uint64 // must use atomic operations to access; keep 64-bit aligned.
	committedIndex    uint64 // must use atomic operations to access; keep 64-bit aligned.
	term              uint64 // must use atomic operations to access; keep 64-bit aligned.
	lead              uint64 // must use atomic operations to access; keep 64-bit aligned.

	// consistIndex used to hold the offset of current executing entry
	// It is initialized to 0 before executing any entry.
	consistIndex consistentIndex // must use atomic operations to access; keep 64-bit aligned.
	r            raftNode        // uses 64-bit atomics; keep 64-bit aligned.

	readych chan struct{}
	Cfg     ServerConfig

	lgMu *sync.RWMutex
	lg   *zap.Logger

	w wait.Wait

	readMu sync.RWMutex
	// read routine notifies etcd server that it waits for reading by sending an empty struct to
	// readwaitC
	readwaitc chan struct{}
	// readNotifier is used to notify the read routine that it can process the request
	// when there is no error
	readNotifier *notifier

	// stop signals the run goroutine should shutdown.
	stop chan struct{}
	// stopping is closed by run goroutine on shutdown.
	stopping chan struct{}
	// done is closed when all goroutines from start() complete.
	done chan struct{}
	// leaderChanged is used to notify the linearizable read loop to drop the old read requests.
	leaderChanged   chan struct{}
	leaderChangedMu sync.RWMutex

	errorc     chan error
	id         types.ID
	attributes membership.Attributes

	cluster *membership.RaftCluster

	v2store     v2store.Store
	snapshotter *snap.Snapshotter

	applyV2 ApplierV2

	// applyV3 is the applier with auth and quotas
	applyV3 applierV3
	// applyV3Base is the core applier without auth or quotas
	applyV3Base applierV3
	applyWait   wait.WaitTime

	kv         mvcc.ConsistentWatchableKV
	lessor     lease.Lessor
	bemu       sync.Mutex
	be         backend.Backend
	authStore  auth.AuthStore
	alarmStore *v3alarm.AlarmStore

	stats  *stats.ServerStats
	lstats *stats.LeaderStats

	SyncTicker *time.Ticker
	// compactor is used to auto-compact the KV.
	compactor v3compactor.Compactor

	// peerRt used to send requests (version, lease) to peers.
	peerRt   http.RoundTripper
	reqIDGen *idutil.Generator

	// forceVersionC is used to force the version monitor loop
	// to detect the cluster version immediately.
	forceVersionC chan struct{}

	// wgMu blocks concurrent waitgroup mutation while server stopping
	wgMu sync.RWMutex
	// wg is used to wait for the go routines that depends on the server state
	// to exit when stopping the server.
	wg sync.WaitGroup

	// ctx is used for etcd-initiated requests that may need to be canceled
	// on etcd server shutdown.
	ctx    context.Context
	cancel context.CancelFunc

	leadTimeMu      sync.RWMutex
	leadElectedTime time.Time

	*AccessController
}
```



### Etcd线性一致性读

线性一致性（Linearizable Read）读，通俗来讲，就是读请求需要读到最新的已经commit的数据，不会读到老数据。

由于所有的节点都可以处理用户的读请求，所以可能导致从不同的节点读数据可能会出现不一致，为解决该问题，通过一种称为ReadIndex的机制来实现线性一致读。

基本原理：

1. Leader首先通过某种机制确认自己依然是Leader；
2. Leader需要给客户端返回最近已应用的数据：即最新被应用到状态机的数据



接收请求并触发一致性读方法

> 代码路径：etcdserver/v3_server.go

```go
func (s *EtcdServer) Range(ctx context.Context, r *pb.RangeRequest) (*pb.RangeResponse, error) {
	var resp *pb.RangeResponse
	var err error
	defer func(start time.Time) {
		warnOfExpensiveReadOnlyRangeRequest(s.getLogger(), start, r, resp, err)
	}(time.Now())

	if !r.Serializable {
        // 通知read routine处理客户端请求
		err = s.linearizableReadNotify(ctx)
		if err != nil {
			return nil, err
		}
	}
	chk := func(ai *auth.AuthInfo) error {
		return s.authStore.IsRangePermitted(ai, r.Key, r.RangeEnd)
	}

	get := func() { resp, err = s.applyV3Base.Range(nil, r) }
	if serr := s.doSerialize(ctx, chk, get); serr != nil {
		err = serr
		return nil, err
	}
	return resp, err
}
```

```go
func (s *EtcdServer) linearizableReadNotify(ctx context.Context) error {
	s.readMu.RLock()
	// 通知read routine处理客户端请求
	nc := s.readNotifier
	s.readMu.RUnlock()

	// signal linearizable loop for current notify if it hasn't been already
	select {
	case s.readwaitc <- struct{}{}:
	default:
	}

	// wait for read state notification
	select {
	case <-nc.c:
		return nc.err
	case <-ctx.Done():
		return ctx.Err()
	case <-s.done:
		return ErrStopped
	}
}
```

```go
// 线性一致性读
func (s *EtcdServer) linearizableReadLoop() {
	var rs raft.ReadState

	for {
		ctxToSend := make([]byte, 8)
		id1 := s.reqIDGen.Next()
        // ctxToSend：id1
		binary.BigEndian.PutUint64(ctxToSend, id1)
		// 通知丢弃已读请求
		leaderChangedNotifier := s.leaderChangedNotify()
		select {
		case <-leaderChangedNotifier:
			continue
		// 接收读请求
		case <-s.readwaitc:
		case <-s.stopping:
			return
		}

		nextnr := newNotifier()

		s.readMu.Lock()
		nr := s.readNotifier
		s.readNotifier = nextnr
		s.readMu.Unlock()

		lg := s.getLogger()
		cctx, cancel := context.WithTimeout(context.Background(), s.Cfg.ReqTimeout())
		// ReadIndex机制,ctxToSend：id1
		if err := s.r.ReadIndex(cctx, ctxToSend); err != nil {
			cancel()
			if err == raft.ErrStopped {
				return
			}
			if lg != nil {
				lg.Warn("failed to get read index from Raft", zap.Error(err))
			} else {
				plog.Errorf("failed to get read index from raft: %v", err)
			}
			readIndexFailed.Inc()
			nr.notify(err)
			continue
		}
		cancel()

		var (
			timeout bool
			done    bool
		)
		for !timeout && !done {
			select {
			case rs = <-s.r.readStateC:
				done = bytes.Equal(rs.RequestCtx, ctxToSend)
				if !done {
					// a previous request might time out. now we should ignore the response of it and
					// continue waiting for the response of the current requests.
					id2 := uint64(0)
					if len(rs.RequestCtx) == 8 {
						id2 = binary.BigEndian.Uint64(rs.RequestCtx)
					}
					if lg != nil {
						lg.Warn(
							"ignored out-of-date read index response; local node read indexes queueing up and waiting to be in sync with leader",
							zap.Uint64("sent-request-id", id1),
							zap.Uint64("received-request-id", id2),
						)
					} else {
						plog.Warningf("ignored out-of-date read index response; local node read indexes queueing up and waiting to be in sync with leader (request ID want %d, got %d)", id1, id2)
					}
					slowReadIndex.Inc()
				}
			case <-leaderChangedNotifier:
				timeout = true
				readIndexFailed.Inc()
				// return a retryable error.
				nr.notify(ErrLeaderChanged)
			case <-time.After(s.Cfg.ReqTimeout()):
				if lg != nil {
					lg.Warn("timed out waiting for read index response (local node might have slow network)", zap.Duration("timeout", s.Cfg.ReqTimeout()))
				} else {
					plog.Warningf("timed out waiting for read index response (local node might have slow network)")
				}
				nr.notify(ErrTimeout)
				timeout = true
				slowReadIndex.Inc()
			case <-s.stopping:
				return
			}
		}
		if !done {
			continue
		}

		if ai := s.getAppliedIndex(); ai < rs.Index {
			select {
			case <-s.applyWait.Wait(rs.Index):
			case <-s.stopping:
				return
			}
		}
		// unblock all l-reads requested at indices before rs.Index
		nr.notify(nil)
	}
}
```

ReadIndex实现

> 代码路径：raft/node.go

```go
func (n *node) ReadIndex(ctx context.Context, rctx []byte) error {
    // 消息类型：MsgReadIndex，消息Data:rctx，为消息ID
	return n.step(ctx, pb.Message{Type: pb.MsgReadIndex, Entries: []pb.Entry{{Data: rctx}}})
}

// pb.Message数据结构
type Message struct {
	Type             MessageType `protobuf:"varint,1,opt,name=type,enum=raftpb.MessageType" json:"type"`
	To               uint64      `protobuf:"varint,2,opt,name=to" json:"to"`
	From             uint64      `protobuf:"varint,3,opt,name=from" json:"from"`
	Term             uint64      `protobuf:"varint,4,opt,name=term" json:"term"`
	LogTerm          uint64      `protobuf:"varint,5,opt,name=logTerm" json:"logTerm"`
	Index            uint64      `protobuf:"varint,6,opt,name=index" json:"index"`
	Entries          []Entry     `protobuf:"bytes,7,rep,name=entries" json:"entries"`
	Commit           uint64      `protobuf:"varint,8,opt,name=commit" json:"commit"`
	Snapshot         Snapshot    `protobuf:"bytes,9,opt,name=snapshot" json:"snapshot"`
	Reject           bool        `protobuf:"varint,10,opt,name=reject" json:"reject"`
	RejectHint       uint64      `protobuf:"varint,11,opt,name=rejectHint" json:"rejectHint"`
	Context          []byte      `protobuf:"bytes,12,opt,name=context" json:"context,omitempty"`
	XXX_unrecognized []byte      `json:"-"`
}

//pb.Entry数据结构
type Entry struct {
	Term             uint64    `protobuf:"varint,2,opt,name=Term" json:"Term"`
	Index            uint64    `protobuf:"varint,3,opt,name=Index" json:"Index"`
	Type             EntryType `protobuf:"varint,1,opt,name=Type,enum=raftpb.EntryType" json:"Type"`
	Data             []byte    `protobuf:"bytes,4,opt,name=Data" json:"Data,omitempty"`
	XXX_unrecognized []byte    `json:"-"`
}
```

```go
func (n *node) step(ctx context.Context, m pb.Message) error {
	return n.stepWithWaitOption(ctx, m, false)
}

// Step advances the state machine using msgs. The ctx.Err() will be returned,
// if any.
func (n *node) stepWithWaitOption(ctx context.Context, m pb.Message, wait bool) error {
    // 消息类型：MsgReadIndex
	if m.Type != pb.MsgProp {
		select {
		// 请求消息发送给node.recvc信道，由node.run进行处理
		case n.recvc <- m:
			return nil
		case <-ctx.Done():
			return ctx.Err()
		case <-n.done:
			return ErrStopped
		}
	}
	ch := n.propc
	pm := msgWithResult{m: m}
	if wait {
		pm.result = make(chan error, 1)
	}
	select {
	case ch <- pm:
		if !wait {
			return nil
		}
	case <-ctx.Done():
		return ctx.Err()
	case <-n.done:
		return ErrStopped
	}
	select {
	case err := <-pm.result:
		if err != nil {
			return err
		}
	case <-ctx.Done():
		return ctx.Err()
	case <-n.done:
		return ErrStopped
	}
	return nil
}
```

node的入口函数接收node.recvc信道的消息，处理请求

> 代码路径：raft/node.go

```go
func (n *node) run() {
	var propc chan msgWithResult
	var readyc chan Ready
	var advancec chan struct{}
	var rd Ready

	r := n.rn.raft

	lead := None

	for {
		if advancec != nil {
			readyc = nil
		} else if n.rn.HasReady() {
			// Populate a Ready. Note that this Ready is not guaranteed to
			// actually be handled. We will arm readyc, but there's no guarantee
			// that we will actually send on it. It's possible that we will
			// service another channel instead, loop around, and then populate
			// the Ready again. We could instead force the previous Ready to be
			// handled first, but it's generally good to emit larger Readys plus
			// it simplifies testing (by emitting less frequently and more
			// predictably).
			rd = n.rn.readyWithoutAccept()
			readyc = n.readyc
		}

		// 不是Leader
		if lead != r.lead {
			if r.hasLeader() {
				if lead == None {
					r.logger.Infof("raft.node: %x elected leader %x at term %d", r.id, r.lead, r.Term)
				} else {
					r.logger.Infof("raft.node: %x changed leader from %x to %x at term %d", r.id, lead, r.lead, r.Term)
				}
				propc = n.propc
			} else {
				r.logger.Infof("raft.node: %x lost leader %x at term %d", r.id, lead, r.Term)
				propc = nil
			}
			lead = r.lead
		}

		select {
		// TODO: maybe buffer the config propose if there exists one (the way
		// described in raft dissertation)
		// Currently it is dropped in Step silently.
		// 接受stepWithWaitOption的通过node.propc信道传送过来的msg
		case pm := <-propc:
			m := pm.m
			m.From = r.id
			err := r.Step(m)
			if pm.result != nil {
				pm.result <- err
				close(pm.result)
			}
		// 接受stepWithWaitOption的通过node.recvc信道传送过来的msg
		case m := <-n.recvc:
			// filter out response message from unknown From.
			if pr := r.prs.Progress[m.From]; pr != nil || !IsResponseMsg(m.Type) {
                // pr为空， 由于消息类型为MsgReadIndex，IsResponseMsg(m.Type)返回false
				r.Step(m)
			}
		case cc := <-n.confc:
			_, okBefore := r.prs.Progress[r.id]
			cs := r.applyConfChange(cc)
			// If the node was removed, block incoming proposals. Note that we
			// only do this if the node was in the config before. Nodes may be
			// a member of the group without knowing this (when they're catching
			// up on the log and don't have the latest config) and we don't want
			// to block the proposal channel in that case.
			//
			// NB: propc is reset when the leader changes, which, if we learn
			// about it, sort of implies that we got readded, maybe? This isn't
			// very sound and likely has bugs.
			if _, okAfter := r.prs.Progress[r.id]; okBefore && !okAfter {
				var found bool
				for _, sl := range [][]uint64{cs.Voters, cs.VotersOutgoing} {
					for _, id := range sl {
						if id == r.id {
							found = true
						}
					}
				}
				if !found {
					propc = nil
				}
			}
			select {
			case n.confstatec <- cs:
			case <-n.done:
			}
		case <-n.tickc:
			n.rn.Tick()
		case readyc <- rd:
			n.rn.acceptReady(rd)
			advancec = n.advancec
		case <-advancec:
			n.rn.Advance(rd)
			rd = Ready{}
			advancec = nil
		case c := <-n.status:
			c <- getStatus(r)
		case <-n.stop:
			close(n.done)
			return
		}
	}
}
```

> 代码路径：raft/raft.go

```go
// 消息类型为MsgReadIndex
func (r *raft) Step(m pb.Message) error {
	// Handle the message term, which may result in our stepping down to a follower.
	switch {
    // 本地msg
	case m.Term == 0:
	// 外部msg
	case m.Term > r.Term:
		if m.Type == pb.MsgVote || m.Type == pb.MsgPreVote {
			force := bytes.Equal(m.Context, []byte(campaignTransfer))
			inLease := r.checkQuorum && r.lead != None && r.electionElapsed < r.electionTimeout
			if !force && inLease {
				// If a server receives a RequestVote request within the minimum election timeout
				// of hearing from a current leader, it does not update its term or grant its vote
				r.logger.Infof("%x [logterm: %d, index: %d, vote: %x] ignored %s from %x [logterm: %d, index: %d] at term %d: lease is not expired (remaining ticks: %d)",
					r.id, r.raftLog.lastTerm(), r.raftLog.lastIndex(), r.Vote, m.Type, m.From, m.LogTerm, m.Index, r.Term, r.electionTimeout-r.electionElapsed)
				return nil
			}
		}
		switch {
		case m.Type == pb.MsgPreVote:
			// Never change our term in response to a PreVote
		case m.Type == pb.MsgPreVoteResp && !m.Reject:
			// We send pre-vote requests with a term in our future. If the
			// pre-vote is granted, we will increment our term when we get a
			// quorum. If it is not, the term comes from the node that
			// rejected our vote so we should become a follower at the new
			// term.
		default:
			r.logger.Infof("%x [term: %d] received a %s message with higher term from %x [term: %d]",
				r.id, r.Term, m.Type, m.From, m.Term)
			if m.Type == pb.MsgApp || m.Type == pb.MsgHeartbeat || m.Type == pb.MsgSnap {
				r.becomeFollower(m.Term, m.From)
			} else {
                // 状态转换为Follower
				r.becomeFollower(m.Term, None)
			}
		}

    // 外部msg
	case m.Term < r.Term:
		if (r.checkQuorum || r.preVote) && (m.Type == pb.MsgHeartbeat || m.Type == pb.MsgApp) {
			// We have received messages from a leader at a lower term. It is possible
			// that these messages were simply delayed in the network, but this could
			// also mean that this node has advanced its term number during a network
			// partition, and it is now unable to either win an election or to rejoin
			// the majority on the old term. If checkQuorum is false, this will be
			// handled by incrementing term numbers in response to MsgVote with a
			// higher term, but if checkQuorum is true we may not advance the term on
			// MsgVote and must generate other messages to advance the term. The net
			// result of these two features is to minimize the disruption caused by
			// nodes that have been removed from the cluster's configuration: a
			// removed node will send MsgVotes (or MsgPreVotes) which will be ignored,
			// but it will not receive MsgApp or MsgHeartbeat, so it will not create
			// disruptive term increases, by notifying leader of this node's activeness.
			// The above comments also true for Pre-Vote
			//
			// When follower gets isolated, it soon starts an election ending
			// up with a higher term than leader, although it won't receive enough
			// votes to win the election. When it regains connectivity, this response
			// with "pb.MsgAppResp" of higher term would force leader to step down.
			// However, this disruption is inevitable to free this stuck node with
			// fresh election. This can be prevented with Pre-Vote phase.
			r.send(pb.Message{To: m.From, Type: pb.MsgAppResp})
		} else if m.Type == pb.MsgPreVote {
			// Before Pre-Vote enable, there may have candidate with higher term,
			// but less log. After update to Pre-Vote, the cluster may deadlock if
			// we drop messages with a lower term.
			r.logger.Infof("%x [logterm: %d, index: %d, vote: %x] rejected %s from %x [logterm: %d, index: %d] at term %d",
				r.id, r.raftLog.lastTerm(), r.raftLog.lastIndex(), r.Vote, m.Type, m.From, m.LogTerm, m.Index, r.Term)
			r.send(pb.Message{To: m.From, Term: r.Term, Type: pb.MsgPreVoteResp, Reject: true})
		} else {
			// ignore other cases
			r.logger.Infof("%x [term: %d] ignored a %s message with lower term from %x [term: %d]",
				r.id, r.Term, m.Type, m.From, m.Term)
		}
		return nil
	}

	switch m.Type {
	case pb.MsgHup:
		if r.state != StateLeader {
			if !r.promotable() {
				r.logger.Warningf("%x is unpromotable and can not campaign; ignoring MsgHup", r.id)
				return nil
			}
			ents, err := r.raftLog.slice(r.raftLog.applied+1, r.raftLog.committed+1, noLimit)
			if err != nil {
				r.logger.Panicf("unexpected error getting unapplied entries (%v)", err)
			}
			if n := numOfPendingConf(ents); n != 0 && r.raftLog.committed > r.raftLog.applied {
				r.logger.Warningf("%x cannot campaign at term %d since there are still %d pending configuration changes to apply", r.id, r.Term, n)
				return nil
			}

			r.logger.Infof("%x is starting a new election at term %d", r.id, r.Term)
			if r.preVote {
				r.campaign(campaignPreElection)
			} else {
				r.campaign(campaignElection)
			}
		} else {
			r.logger.Debugf("%x ignoring MsgHup because already leader", r.id)
		}

	case pb.MsgVote, pb.MsgPreVote:
		// We can vote if this is a repeat of a vote we've already cast...
		canVote := r.Vote == m.From ||
			// ...we haven't voted and we don't think there's a leader yet in this term...
			(r.Vote == None && r.lead == None) ||
			// ...or this is a PreVote for a future term...
			(m.Type == pb.MsgPreVote && m.Term > r.Term)
		// ...and we believe the candidate is up to date.
		if canVote && r.raftLog.isUpToDate(m.Index, m.LogTerm) {
			// Note: it turns out that that learners must be allowed to cast votes.
			// This seems counter- intuitive but is necessary in the situation in which
			// a learner has been promoted (i.e. is now a voter) but has not learned
			// about this yet.
			// For example, consider a group in which id=1 is a learner and id=2 and
			// id=3 are voters. A configuration change promoting 1 can be committed on
			// the quorum `{2,3}` without the config change being appended to the
			// learner's log. If the leader (say 2) fails, there are de facto two
			// voters remaining. Only 3 can win an election (due to its log containing
			// all committed entries), but to do so it will need 1 to vote. But 1
			// considers itself a learner and will continue to do so until 3 has
			// stepped up as leader, replicates the conf change to 1, and 1 applies it.
			// Ultimately, by receiving a request to vote, the learner realizes that
			// the candidate believes it to be a voter, and that it should act
			// accordingly. The candidate's config may be stale, too; but in that case
			// it won't win the election, at least in the absence of the bug discussed
			// in:
			// https://github.com/etcd-io/etcd/issues/7625#issuecomment-488798263.
			r.logger.Infof("%x [logterm: %d, index: %d, vote: %x] cast %s for %x [logterm: %d, index: %d] at term %d",
				r.id, r.raftLog.lastTerm(), r.raftLog.lastIndex(), r.Vote, m.Type, m.From, m.LogTerm, m.Index, r.Term)
			// When responding to Msg{Pre,}Vote messages we include the term
			// from the message, not the local term. To see why, consider the
			// case where a single node was previously partitioned away and
			// it's local term is now out of date. If we include the local term
			// (recall that for pre-votes we don't update the local term), the
			// (pre-)campaigning node on the other end will proceed to ignore
			// the message (it ignores all out of date messages).
			// The term in the original message and current local term are the
			// same in the case of regular votes, but different for pre-votes.
			r.send(pb.Message{To: m.From, Term: m.Term, Type: voteRespMsgType(m.Type)})
			if m.Type == pb.MsgVote {
				// Only record real votes.
				r.electionElapsed = 0
				r.Vote = m.From
			}
		} else {
			r.logger.Infof("%x [logterm: %d, index: %d, vote: %x] rejected %s from %x [logterm: %d, index: %d] at term %d",
				r.id, r.raftLog.lastTerm(), r.raftLog.lastIndex(), r.Vote, m.Type, m.From, m.LogTerm, m.Index, r.Term)
			r.send(pb.Message{To: m.From, Term: r.Term, Type: voteRespMsgType(m.Type), Reject: true})
		}

	default:
		err := r.step(r, m)
		if err != nil {
			return err
		}
	}
	return nil
}
```



### 参考资料

1. etcd-raft 源码分析：https://zhuanlan.zhihu.com/p/49792009
2. etcd源码阅读与分析：https://jiajunhuang.com/articles/2018_11_20-etcd_source_code_analysis_raftexample.md.html
3. etcd-raft的线性一致读方法一：ReadIndex：https://zhuanlan.zhihu.com/p/31050303
4. etcd raft如何实现Linearizable Read：https://zhuanlan.zhihu.com/p/27869566
5. etcd Raft库解析：https://www.codedump.info/post/20180922-etcd-raft/
6. etcd raft 设计与实现（一）：https://zhuanlan.zhihu.com/p/54846720