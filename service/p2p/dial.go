package p2p

import (
	"container/heap"
	"crypto/rand"
	"errors"
	"net"
	"time"

	"github.com/aidoc/go-aidoc/service/p2p/discover"
	"github.com/aidoc/go-aidoc/service/p2p/netutil"
	"github.com/aidoc/go-aidoc/lib/i18"
	"fmt"
)

const (
	// 这是在重拨某个节点之间等待的时间。
	dialHistoryExpiration = 30 * time.Second

	//发现查找受到限制，每隔几秒就只能运行一次。
	lookupInterval = 4 * time.Second

	// 如果在这段时间内未找到对等端，则尝试连接初始引导节点。
	fallbackInterval = 20 * time.Second

	// 端点分辨率受到有界退避的限制。
	initialResolveDelay = 60 * time.Second
	maxResolveDelay     = time.Hour
)

// NodeDialer 用于连接到网络中的节点，通常使用底层的 net.Dialer，但也在测试中使用 net.Pipe
type NodeDialer interface {
	Dial(*discover.Node) (net.Conn, error)
}

// TCPDialer 通过使用 net.Dialer 创建与网络中节点的 TCP 连接来实现 NodeDialer 接口
type TCPDialer struct {
	*net.Dialer
}

// 拨号创建与节点的TCP连接
func (t TCPDialer) Dial(dest *discover.Node) (net.Conn, error) {
	addr := &net.TCPAddr{IP: dest.IP, Port: int(dest.TCP)}
	return t.Dialer.Dial("tcp", addr.String())
}

// dialstate 计划拨号和发现查找。
// 它有机会在 Server.run 的主循环的每次迭代中计算新任务。
type dialstate struct {
	maxDynDials int
	ntab        discoverTable
	netrestrict *netutil.Netlist

	lookupRunning bool
	dialing       map[discover.NodeID]connFlag
	lookupBuf     []*discover.Node // 当前发现查找结果
	randomNodes   []*discover.Node // 从表中填写
	static        map[discover.NodeID]*dialTask
	hist          *dialHistory

	start     time.Time        // 首次使用拨号器的时间
	bootnodes []*discover.Node // 没有对等体时默认拨号
}

type discoverTable interface {
	Self() *discover.Node
	Close()
	Resolve(target discover.NodeID) *discover.Node
	Lookup(target discover.NodeID) []*discover.Node
	ReadRandomNodes([]*discover.Node) int
}

//
type dialHistory []pastDial

// pastDial是拨号历史记录中的一个条目。
type pastDial struct {
	id  discover.NodeID
	exp time.Time
}

type task interface {
	Do(*Server)
}

// 为每个拨打的节点生成dialTask。
// 任务运行时无法访问其字段。
type dialTask struct {
	flags        connFlag
	dest         *discover.Node
	lastResolved time.Time
	resolveDelay time.Duration
}

// discoverTask运行发现表操作。
// 任何时候只有一个discoverTask处于活动状态。
// discoverTask.Do执行随机查找。
type discoverTask struct {
	results []*discover.Node
}

// 如果没有其他任务可以在Server.run中保持循环，则会生成waitExpireTask。
type waitExpireTask struct {
	time.Duration
}

func newDialState(static []*discover.Node, bootnodes []*discover.Node, ntab discoverTable, maxdyn int, netrestrict *netutil.Netlist) *dialstate {
	s := &dialstate{
		maxDynDials: maxdyn,
		ntab:        ntab,
		netrestrict: netrestrict,
		static:      make(map[discover.NodeID]*dialTask),
		dialing:     make(map[discover.NodeID]connFlag),
		bootnodes:   make([]*discover.Node, len(bootnodes)),
		randomNodes: make([]*discover.Node, maxdyn/2),
		hist:        new(dialHistory),
	}
	copy(s.bootnodes, bootnodes)
	for _, n := range static {
		s.addStatic(n)
	}
	return s
}

func (s *dialstate) addStatic(n *discover.Node) {
	s.static[n.ID] = &dialTask{flags: staticDialedConn, dest: n}
}

// 这会覆盖任务而不是更新现有任务
// 条目，为用户提供强制解析操作的机会。
func (s *dialstate) removeStatic(n *discover.Node) {
	// 这将删除任务，以便将来尝试连接。
	delete(s.static, n.ID)
	// 这将删除以前的拨号时间戳，以便应用程序
	// 可以强制服务器立即重新连接所选对等体。
	s.hist.remove(n.ID)
}

func (s *dialstate) newTasks(nRunning int, peers map[discover.NodeID]*Peer, now time.Time) []task {
	if s.start.IsZero() {
		s.start = now
	}

	var newtasks []task
	addDial := func(flag connFlag, n *discover.Node) bool {
		if err := s.checkDial(n, peers); err != nil {
			log_p2p.Trace("跳过拨号候选人", "id", n.ID, "addr", &net.TCPAddr{IP: n.IP, Port: int(n.TCP)},   err)
			return false
		}
		s.dialing[n.ID] = flag
		newtasks = append(newtasks, &dialTask{flags: flag, dest: n})
		return true
	}

	// 此时需要计算动态拨号的数量。
	needDynDials := s.maxDynDials
	for _, p := range peers {
		if p.rw.is(dynDialedConn) {
			needDynDials--
		}
	}
	for _, flag := range s.dialing {
		if flag&dynDialedConn != 0 {
			needDynDials--
		}
	}

	// 在每次调用时使拨号历史记录到期。
	s.hist.expire(now)

	// 如果未连接，则为静态节点创建拨号。
	for id, t := range s.static {
		err := s.checkDial(t.dest, peers)
		switch err {
		case errNotWhitelisted, errSelf:
			log_p2p.Warn("删除静态拨号候选", "id", t.dest.ID, "addr", &net.TCPAddr{IP: t.dest.IP, Port: int(t.dest.TCP)},   err)
			delete(s.static, t.dest.ID)
		case nil:
			s.dialing[id] = t.flags
			newtasks = append(newtasks, t)
		}
	}
	// 如果我们没有任何对等方，请尝试拨打随机bootnode。
	// 这种情况对于testnet（和专用网络）非常有用，
	// 在这种情况下，发现表可能充满了大多数不良对等体，因此很难找到好的对等体。
	if len(peers) == 0 && len(s.bootnodes) > 0 && needDynDials > 0 && now.Sub(s.start) > fallbackInterval {
		bootnode := s.bootnodes[0]
		s.bootnodes = append(s.bootnodes[:0], s.bootnodes[1:]...)
		s.bootnodes = append(s.bootnodes, bootnode)

		if addDial(dynDialedConn, bootnode) {
			needDynDials--
		}
	}
	// 使用表中的随机节点获取一半必要的动态拨号。
	randomCandidates := needDynDials / 2
	if randomCandidates > 0 {
		n := s.ntab.ReadRandomNodes(s.randomNodes)
		for i := 0; i < randomCandidates && i < n; i++ {
			if addDial(dynDialedConn, s.randomNodes[i]) {
				needDynDials--
			}
		}
	}
	//从随机查找结果创建动态拨号，从结果缓冲区中删除trie项。
	i := 0
	for ; i < len(s.lookupBuf) && needDynDials > 0; i++ {
		if addDial(dynDialedConn, s.lookupBuf[i]) {
			needDynDials--
		}
	}
	s.lookupBuf = s.lookupBuf[:copy(s.lookupBuf, s.lookupBuf[i:])]
	// 如果需要更多候选项，则启动发现查找。
	if len(s.lookupBuf) < needDynDials && !s.lookupRunning {
		s.lookupRunning = true
		newtasks = append(newtasks, &discoverTask{})
	}

	// 如果已尝试所有候选项且当前没有任务处于活动状态，则启动计时器以等待下一个节点过期。
	// 这应该可以防止由于没有挂起事件而未勾选拨号器逻辑的情况。
	if nRunning == 0 && len(newtasks) == 0 && s.hist.Len() > 0 {
		t := &waitExpireTask{s.hist.min().exp.Sub(now)}
		newtasks = append(newtasks, t)
	}
	return newtasks
}

var (
	errSelf             = errors.New("是自身")
	errAlreadyDialing   = errors.New("正在连接")
	errAlreadyConnected = errors.New("已经连接")
	errRecentlyDialed   = errors.New("最近连过")
	errNotWhitelisted   = errors.New("不在白名单中")
)

func (s *dialstate) checkDial(n *discover.Node, peers map[discover.NodeID]*Peer) error {
	_, dialing := s.dialing[n.ID]
	switch {
	case dialing:
		return errAlreadyDialing
	case peers[n.ID] != nil:
		return errAlreadyConnected
	case s.ntab != nil && n.ID == s.ntab.Self().ID:
		return errSelf
	case s.netrestrict != nil && !s.netrestrict.Contains(n.IP):
		return errNotWhitelisted
	case s.hist.contains(n.ID):
		return errRecentlyDialed
	}
	return nil
}

func (s *dialstate) taskDone(t task, now time.Time) {
	switch t := t.(type) {
	case *dialTask:
		s.hist.add(t.dest.ID, now.Add(dialHistoryExpiration))
		delete(s.dialing, t.dest.ID)
	case *discoverTask:
		s.lookupRunning = false
		s.lookupBuf = append(s.lookupBuf, t.results...)
	}
}

func (t *dialTask) Do(srv *Server) {
	if t.dest.Incomplete() {
		if !t.resolve(srv) {
			return
		}
	}
	err := t.dial(srv, t.dest)
	if err != nil {
		log_p2p.Trace("拨号错误", "task", t,   err)
		// 如果拨号失败，请尝试解析静态节点的ID。
		if _, ok := err.(*dialError); ok && t.flags&staticDialedConn != 0 {
			if t.resolve(srv) {
				t.dial(srv, t.dest)
			}
		}
	}
}

// 解析尝试使用发现查找目标的当前端点。
//
// 使用退避来限制解析操作，以避免使用对不存在的节点的无用查询充斥发现网络。
// 找到节点时，退避延迟会重置。
func (t *dialTask) resolve(srv *Server) bool {
	if srv.ntab == nil {
		log_p2p.Debug("无法解析节点", "id", t.dest.ID,   "发现被禁用")
		return false
	}
	if t.resolveDelay == 0 {
		t.resolveDelay = initialResolveDelay
	}
	if time.Since(t.lastResolved) < t.resolveDelay {
		return false
	}
	resolved := srv.ntab.Resolve(t.dest.ID)
	t.lastResolved = time.Now()
	if resolved == nil {
		t.resolveDelay *= 2
		if t.resolveDelay > maxResolveDelay {
			t.resolveDelay = maxResolveDelay
		}
		log_p2p.Debug("解析节点失败", "id", t.dest.ID, "newdelay", t.resolveDelay)
		return false
	}
	// 找到了该节点。
	t.resolveDelay = initialResolveDelay
	t.dest = resolved
	log_p2p.Debug("已解决的节点", "id", t.dest.ID, "addr", &net.TCPAddr{IP: t.dest.IP, Port: int(t.dest.TCP)})
	return true
}

type dialError struct {
	error
}

// 拨号执行实际的连接尝试。
func (t *dialTask) dial(srv *Server, dest *discover.Node) error {
	fd, err := srv.Dialer.Dial(dest)
	if err != nil {
		return &dialError{err}
	}
	mfd := newMeteredConn(fd, false)
	return srv.SetupConn(mfd, t.flags, dest)
}

func (t *dialTask) String() string {
	return fmt.Sprintf("%v %x %v:%d", t.flags, t.dest.ID[:8], t.dest.IP, t.dest.TCP)
}

func (t *discoverTask) Do(srv *Server) {
	// 每当需要动态拨号时，newTasks都会生成查找任务。
	// 查找需要花一些时间，否则事件循环旋转太快。
	next := srv.lastLookup.Add(lookupInterval)
	if now := time.Now(); now.Before(next) {
		time.Sleep(next.Sub(now))
	}
	srv.lastLookup = time.Now()
	var target discover.NodeID
	rand.Read(target[:])
	t.results = srv.ntab.Lookup(target)
}

func (t *discoverTask) String() string {
	s := "发现查找"
	if len(t.results) > 0 {
		s += i18.I18_print.Sprintf(" (%d 结果)", len(t.results))
	}
	return s
}

func (t waitExpireTask) Do(*Server) {
	time.Sleep(t.Duration)
}
func (t waitExpireTask) String() string {
	return i18.I18_print.Sprintf("等待拨打hist expire（%v）", t.Duration)
}

// 仅使用这些方法来访问或修改dialHistory。
func (h dialHistory) min() pastDial {
	return h[0]
}
func (h *dialHistory) add(id discover.NodeID, exp time.Time) {
	heap.Push(h, pastDial{id, exp})

}
func (h *dialHistory) remove(id discover.NodeID) bool {
	for i, v := range *h {
		if v.id == id {
			heap.Remove(h, i)
			return true
		}
	}
	return false
}
func (h dialHistory) contains(id discover.NodeID) bool {
	for _, v := range h {
		if v.id == id {
			return true
		}
	}
	return false
}
func (h *dialHistory) expire(now time.Time) {
	for h.Len() > 0 && h.min().exp.Before(now) {
		heap.Pop(h)
	}
}

//heap.Interface样板
func (h dialHistory) Len() int           { return len(h) }
func (h dialHistory) Less(i, j int) bool { return h[i].exp.Before(h[j].exp) }
func (h dialHistory) Swap(i, j int)      { h[i], h[j] = h[j], h[i] }
func (h *dialHistory) Push(x interface{}) {
	*h = append(*h, x.(pastDial))
}
func (h *dialHistory) Pop() interface{} {
	old := *h
	n := len(old)
	x := old[n-1]
	*h = old[0 : n-1]
	return x
}
