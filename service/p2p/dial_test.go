



package p2p

import (
	"encoding/binary"
	"net"
	"reflect"
	"testing"
	"time"

	"github.com/davecgh/go-spew/spew"
	"github.com/aidoc/go-aidoc/service/p2p/discover"
	"github.com/aidoc/go-aidoc/service/p2p/netutil"
)

func init() {
	spew.Config.Indent = "\t"
}

type dialtest struct {
	init   *dialstate // 测试之前和之后的状态。
	rounds []round
}

type round struct {
	peers []*Peer // 当前的对等节点集
	done  []task  // 这轮完成的任务
	new   []task  // 结果必须与此匹配
}

func runDialTest(t *testing.T, test dialtest) {
	var (
		vtime   time.Time
		running int
	)
	pm := func(ps []*Peer) map[discover.NodeID]*Peer {
		m := make(map[discover.NodeID]*Peer)
		for _, p := range ps {
			m[p.rw.id] = p
		}
		return m
	}
	for i, round := range test.rounds {
		for _, task := range round.done {
			running--
			if running < 0 {
				panic("运行任务计数器下溢")
			}
			test.init.taskDone(task, vtime)
		}

		new := test.init.newTasks(running, pm(round.peers), vtime)
		if !sametasks(new, round.new) {
			t.Errorf("轮 %d：新任务不匹配：\n得到%v \n想要%v \n状态：%v \n正在运行：%v \n",
				i, spew.Sdump(new), spew.Sdump(round.new), spew.Sdump(test.init), spew.Sdump(running))
		}

		// 每一轮时间提前16秒。
		vtime = vtime.Add(16 * time.Second)
		running += len(new)
	}
}

type fakeTable []*discover.Node

func (t fakeTable) Self() *discover.Node                     { return new(discover.Node) }
func (t fakeTable) Close()                                   {}
func (t fakeTable) Lookup(discover.NodeID) []*discover.Node  { return nil }
func (t fakeTable) Resolve(discover.NodeID) *discover.Node   { return nil }
func (t fakeTable) ReadRandomNodes(buf []*discover.Node) int { return copy(buf, t) }

// 此测试检查从发现结果启动动态拨号。
func TestDialStateDynDial(t *testing.T) {
	runDialTest(t, dialtest{
		init: newDialState(nil, nil, fakeTable{}, 5, nil),
		rounds: []round{
			// 启动发现查询。
			{
				peers: []*Peer{
					{rw: &conn{flags: staticDialedConn, id: uintID(0)}},
					{rw: &conn{flags: dynDialedConn, id: uintID(1)}},
					{rw: &conn{flags: dynDialedConn, id: uintID(2)}},
				},
				new: []task{&discoverTask{}},
			},
			// 动态拨号在完成后启动。
			{
				peers: []*Peer{
					{rw: &conn{flags: staticDialedConn, id: uintID(0)}},
					{rw: &conn{flags: dynDialedConn, id: uintID(1)}},
					{rw: &conn{flags: dynDialedConn, id: uintID(2)}},
				},
				done: []task{
					&discoverTask{results: []*discover.Node{
						{ID: uintID(2)}, // 这个已经连接，没有拨打。
						{ID: uintID(3)},
						{ID: uintID(4)},
						{ID: uintID(5)},
						{ID: uintID(6)}, // 这些没有尝试，因为最大dyn拨号是5
						{ID: uintID(7)}, // ...
					}},
				},
				new: []task{
					&dialTask{flags: dynDialedConn, dest: &discover.Node{ID: uintID(3)}},
					&dialTask{flags: dynDialedConn, dest: &discover.Node{ID: uintID(4)}},
					&dialTask{flags: dynDialedConn, dest: &discover.Node{ID: uintID(5)}},
				},
			},
			// 部分拨号已完成但尚未启动新拨号，因为活动拨号计数和动态对等计数的总和为 == maxDynDials。
			{
				peers: []*Peer{
					{rw: &conn{flags: staticDialedConn, id: uintID(0)}},
					{rw: &conn{flags: dynDialedConn, id: uintID(1)}},
					{rw: &conn{flags: dynDialedConn, id: uintID(2)}},
					{rw: &conn{flags: dynDialedConn, id: uintID(3)}},
					{rw: &conn{flags: dynDialedConn, id: uintID(4)}},
				},
				done: []task{
					&dialTask{flags: dynDialedConn, dest: &discover.Node{ID: uintID(3)}},
					&dialTask{flags: dynDialedConn, dest: &discover.Node{ID: uintID(4)}},
				},
			},
			// 由于已达到 maxDynDials，因此未在此轮次中启动新的拨号任务。
			{
				peers: []*Peer{
					{rw: &conn{flags: staticDialedConn, id: uintID(0)}},
					{rw: &conn{flags: dynDialedConn, id: uintID(1)}},
					{rw: &conn{flags: dynDialedConn, id: uintID(2)}},
					{rw: &conn{flags: dynDialedConn, id: uintID(3)}},
					{rw: &conn{flags: dynDialedConn, id: uintID(4)}},
					{rw: &conn{flags: dynDialedConn, id: uintID(5)}},
				},
				done: []task{
					&dialTask{flags: dynDialedConn, dest: &discover.Node{ID: uintID(5)}},
				},
				new: []task{
					&waitExpireTask{Duration: 14 * time.Second},
				},
			},
			// 在这一轮中，id为 2 的对等体掉线。 重复使用上次发现查找的查询结果。
			{
				peers: []*Peer{
					{rw: &conn{flags: staticDialedConn, id: uintID(0)}},
					{rw: &conn{flags: dynDialedConn, id: uintID(1)}},
					{rw: &conn{flags: dynDialedConn, id: uintID(3)}},
					{rw: &conn{flags: dynDialedConn, id: uintID(4)}},
					{rw: &conn{flags: dynDialedConn, id: uintID(5)}},
				},
				new: []task{
					&dialTask{flags: dynDialedConn, dest: &discover.Node{ID: uintID(6)}},
				},
			},
			// 更多同伴（3,4）下车并拨打ID 6完成。
			// 重复使用发现查找的最后一个查询结果，并生成一个新查询结果，因为需要更多候选项。
			{
				peers: []*Peer{
					{rw: &conn{flags: staticDialedConn, id: uintID(0)}},
					{rw: &conn{flags: dynDialedConn, id: uintID(1)}},
					{rw: &conn{flags: dynDialedConn, id: uintID(5)}},
				},
				done: []task{
					&dialTask{flags: dynDialedConn, dest: &discover.Node{ID: uintID(6)}},
				},
				new: []task{
					&dialTask{flags: dynDialedConn, dest: &discover.Node{ID: uintID(7)}},
					&discoverTask{},
				},
			},
			// Peer 7已连接，但仍然没有足够的动态对等体（5个中有4个）。
			// 但是，发现已在运行，因此请确保没有启动新功能。
			{
				peers: []*Peer{
					{rw: &conn{flags: staticDialedConn, id: uintID(0)}},
					{rw: &conn{flags: dynDialedConn, id: uintID(1)}},
					{rw: &conn{flags: dynDialedConn, id: uintID(5)}},
					{rw: &conn{flags: dynDialedConn, id: uintID(7)}},
				},
				done: []task{
					&dialTask{flags: dynDialedConn, dest: &discover.Node{ID: uintID(7)}},
				},
			},
			// 使用空集完成正在运行的节点发现。
			// 应立即请求新的查找。
			{
				peers: []*Peer{
					{rw: &conn{flags: staticDialedConn, id: uintID(0)}},
					{rw: &conn{flags: dynDialedConn, id: uintID(1)}},
					{rw: &conn{flags: dynDialedConn, id: uintID(5)}},
					{rw: &conn{flags: dynDialedConn, id: uintID(7)}},
				},
				done: []task{
					&discoverTask{},
				},
				new: []task{
					&discoverTask{},
				},
			},
		},
	})
}

// 如果没有连接对等端，则调用引导节点，但不会连接。
func TestDialStateDynDialBootnode(t *testing.T) {
	bootnodes := []*discover.Node{
		{ID: uintID(1)},
		{ID: uintID(2)},
		{ID: uintID(3)},
	}
	table := fakeTable{
		{ID: uintID(4)},
		{ID: uintID(5)},
		{ID: uintID(6)},
		{ID: uintID(7)},
		{ID: uintID(8)},
	}
	runDialTest(t, dialtest{
		init: newDialState(nil, bootnodes, table, 5, nil),
		rounds: []round{
			// 尝试了 2 次动态拨号，引导节点等待回退间隔
			{
				new: []task{
					&dialTask{flags: dynDialedConn, dest: &discover.Node{ID: uintID(4)}},
					&dialTask{flags: dynDialedConn, dest: &discover.Node{ID: uintID(5)}},
					&discoverTask{},
				},
			},
			// 没有拨号成功，bootnodes 仍然挂起回退间隔
			{
				done: []task{
					&dialTask{flags: dynDialedConn, dest: &discover.Node{ID: uintID(4)}},
					&dialTask{flags: dynDialedConn, dest: &discover.Node{ID: uintID(5)}},
				},
			},
			// 没有拨号成功，bootnodes仍然挂起回退间隔
			{},
			// 没有拨号成功，尝试了2次动态拨号，并且在达到回退间隔时也尝试了1次启动
			{
				new: []task{
					&dialTask{flags: dynDialedConn, dest: &discover.Node{ID: uintID(1)}},
					&dialTask{flags: dynDialedConn, dest: &discover.Node{ID: uintID(4)}},
					&dialTask{flags: dynDialedConn, dest: &discover.Node{ID: uintID(5)}},
				},
			},
			// 没有拨号成功，尝试第二个 bootnode
			{
				done: []task{
					&dialTask{flags: dynDialedConn, dest: &discover.Node{ID: uintID(1)}},
					&dialTask{flags: dynDialedConn, dest: &discover.Node{ID: uintID(4)}},
					&dialTask{flags: dynDialedConn, dest: &discover.Node{ID: uintID(5)}},
				},
				new: []task{
					&dialTask{flags: dynDialedConn, dest: &discover.Node{ID: uintID(2)}},
				},
			},
			// 没有拨号成功，尝试第三个 bootnode
			{
				done: []task{
					&dialTask{flags: dynDialedConn, dest: &discover.Node{ID: uintID(2)}},
				},
				new: []task{
					&dialTask{flags: dynDialedConn, dest: &discover.Node{ID: uintID(3)}},
				},
			},
			// 没有拨号成功，再次尝试第一个 bootnode，重试过期的随机节点
			{
				done: []task{
					&dialTask{flags: dynDialedConn, dest: &discover.Node{ID: uintID(3)}},
				},
				new: []task{
					&dialTask{flags: dynDialedConn, dest: &discover.Node{ID: uintID(1)}},
					&dialTask{flags: dynDialedConn, dest: &discover.Node{ID: uintID(4)}},
					&dialTask{flags: dynDialedConn, dest: &discover.Node{ID: uintID(5)}},
				},
			},
			// 随机拨号成功，不再尝试启动节点
			{
				peers: []*Peer{
					{rw: &conn{flags: dynDialedConn, id: uintID(4)}},
				},
				done: []task{
					&dialTask{flags: dynDialedConn, dest: &discover.Node{ID: uintID(1)}},
					&dialTask{flags: dynDialedConn, dest: &discover.Node{ID: uintID(4)}},
					&dialTask{flags: dynDialedConn, dest: &discover.Node{ID: uintID(5)}},
				},
			},
		},
	})
}

func TestDialStateDynDialFromTable(t *testing.T) {
	// 此表始终按以下顺序返回相同的随机节点。
	table := fakeTable{
		{ID: uintID(1)},
		{ID: uintID(2)},
		{ID: uintID(3)},
		{ID: uintID(4)},
		{ID: uintID(5)},
		{ID: uintID(6)},
		{ID: uintID(7)},
		{ID: uintID(8)},
	}

	runDialTest(t, dialtest{
		init: newDialState(nil, nil, table, 10, nil),
		rounds: []round{
			// 拨打 ReadRandomNodes 返回的 8 个节点中的 5 个。
			{
				new: []task{
					&dialTask{flags: dynDialedConn, dest: &discover.Node{ID: uintID(1)}},
					&dialTask{flags: dynDialedConn, dest: &discover.Node{ID: uintID(2)}},
					&dialTask{flags: dynDialedConn, dest: &discover.Node{ID: uintID(3)}},
					&dialTask{flags: dynDialedConn, dest: &discover.Node{ID: uintID(4)}},
					&dialTask{flags: dynDialedConn, dest: &discover.Node{ID: uintID(5)}},
					&discoverTask{},
				},
			},
			// 拨号节点1,2成功。 启动查找拨号。
			{
				peers: []*Peer{
					{rw: &conn{flags: dynDialedConn, id: uintID(1)}},
					{rw: &conn{flags: dynDialedConn, id: uintID(2)}},
				},
				done: []task{
					&dialTask{flags: dynDialedConn, dest: &discover.Node{ID: uintID(1)}},
					&dialTask{flags: dynDialedConn, dest: &discover.Node{ID: uintID(2)}},
					&discoverTask{results: []*discover.Node{
						{ID: uintID(10)},
						{ID: uintID(11)},
						{ID: uintID(12)},
					}},
				},
				new: []task{
					&dialTask{flags: dynDialedConn, dest: &discover.Node{ID: uintID(10)}},
					&dialTask{flags: dynDialedConn, dest: &discover.Node{ID: uintID(11)}},
					&dialTask{flags: dynDialedConn, dest: &discover.Node{ID: uintID(12)}},
					&discoverTask{},
				},
			},
			// 拨号节点3,4,5失败。 查找的拨号成功。
			{
				peers: []*Peer{
					{rw: &conn{flags: dynDialedConn, id: uintID(1)}},
					{rw: &conn{flags: dynDialedConn, id: uintID(2)}},
					{rw: &conn{flags: dynDialedConn, id: uintID(10)}},
					{rw: &conn{flags: dynDialedConn, id: uintID(11)}},
					{rw: &conn{flags: dynDialedConn, id: uintID(12)}},
				},
				done: []task{
					&dialTask{flags: dynDialedConn, dest: &discover.Node{ID: uintID(3)}},
					&dialTask{flags: dynDialedConn, dest: &discover.Node{ID: uintID(4)}},
					&dialTask{flags: dynDialedConn, dest: &discover.Node{ID: uintID(5)}},
					&dialTask{flags: dynDialedConn, dest: &discover.Node{ID: uintID(10)}},
					&dialTask{flags: dynDialedConn, dest: &discover.Node{ID: uintID(11)}},
					&dialTask{flags: dynDialedConn, dest: &discover.Node{ID: uintID(12)}},
				},
			},
			// 等待到期。 没有启动 waitExpireTask，因为发现查询仍在运行。
			{
				peers: []*Peer{
					{rw: &conn{flags: dynDialedConn, id: uintID(1)}},
					{rw: &conn{flags: dynDialedConn, id: uintID(2)}},
					{rw: &conn{flags: dynDialedConn, id: uintID(10)}},
					{rw: &conn{flags: dynDialedConn, id: uintID(11)}},
					{rw: &conn{flags: dynDialedConn, id: uintID(12)}},
				},
			},
			// 不再尝试节点3,4，因为只尝试了前两个返回的随机节点（节点1,2）并且它们已经连接。
			{
				peers: []*Peer{
					{rw: &conn{flags: dynDialedConn, id: uintID(1)}},
					{rw: &conn{flags: dynDialedConn, id: uintID(2)}},
					{rw: &conn{flags: dynDialedConn, id: uintID(10)}},
					{rw: &conn{flags: dynDialedConn, id: uintID(11)}},
					{rw: &conn{flags: dynDialedConn, id: uintID(12)}},
				},
			},
		},
	})
}

//  此测试检查未拨打与 netrestrict 列表不匹配的候选项。
func TestDialStateNetRestrict(t *testing.T) {
	// 此表始终按以下顺序返回相同的随机节点。
	table := fakeTable{
		{ID: uintID(1), IP: net.ParseIP("127.0.0.1")},
		{ID: uintID(2), IP: net.ParseIP("127.0.0.2")},
		{ID: uintID(3), IP: net.ParseIP("127.0.0.3")},
		{ID: uintID(4), IP: net.ParseIP("127.0.0.4")},
		{ID: uintID(5), IP: net.ParseIP("127.0.2.5")},
		{ID: uintID(6), IP: net.ParseIP("127.0.2.6")},
		{ID: uintID(7), IP: net.ParseIP("127.0.2.7")},
		{ID: uintID(8), IP: net.ParseIP("127.0.2.8")},
	}
	restrict := new(netutil.Netlist)
	restrict.Add("127.0.2.0/24")

	runDialTest(t, dialtest{
		init: newDialState(nil, nil, table, 10, restrict),
		rounds: []round{
			{
				new: []task{
					&dialTask{flags: dynDialedConn, dest: table[4]},
					&discoverTask{},
				},
			},
		},
	})
}

// 此测试检查是否已启动静态拨号。
func TestDialStateStaticDial(t *testing.T) {
	wantStatic := []*discover.Node{
		{ID: uintID(1)},
		{ID: uintID(2)},
		{ID: uintID(3)},
		{ID: uintID(4)},
		{ID: uintID(5)},
	}

	runDialTest(t, dialtest{
		init: newDialState(wantStatic, nil, fakeTable{}, 0, nil),
		rounds: []round{
			// 为尚未连接的节点启动静态拨号。
			{
				peers: []*Peer{
					{rw: &conn{flags: dynDialedConn, id: uintID(1)}},
					{rw: &conn{flags: dynDialedConn, id: uintID(2)}},
				},
				new: []task{
					&dialTask{flags: staticDialedConn, dest: &discover.Node{ID: uintID(3)}},
					&dialTask{flags: staticDialedConn, dest: &discover.Node{ID: uintID(4)}},
					&dialTask{flags: staticDialedConn, dest: &discover.Node{ID: uintID(5)}},
				},
			},
			// 此轮中没有启动任何新任务，因为所有静态节点都已连接或仍在拨打。
			{
				peers: []*Peer{
					{rw: &conn{flags: dynDialedConn, id: uintID(1)}},
					{rw: &conn{flags: dynDialedConn, id: uintID(2)}},
					{rw: &conn{flags: staticDialedConn, id: uintID(3)}},
				},
				done: []task{
					&dialTask{flags: staticDialedConn, dest: &discover.Node{ID: uintID(3)}},
				},
			},
			//  没有启动新的拨号任务，因为现在所有静态节点都已连接。
			{
				peers: []*Peer{
					{rw: &conn{flags: dynDialedConn, id: uintID(1)}},
					{rw: &conn{flags: dynDialedConn, id: uintID(2)}},
					{rw: &conn{flags: staticDialedConn, id: uintID(3)}},
					{rw: &conn{flags: staticDialedConn, id: uintID(4)}},
					{rw: &conn{flags: staticDialedConn, id: uintID(5)}},
				},
				done: []task{
					&dialTask{flags: staticDialedConn, dest: &discover.Node{ID: uintID(4)}},
					&dialTask{flags: staticDialedConn, dest: &discover.Node{ID: uintID(5)}},
				},
				new: []task{
					&waitExpireTask{Duration: 14 * time.Second},
				},
			},
			// 等待拨号历史记录过期，不会产生任何新任务。
			{
				peers: []*Peer{
					{rw: &conn{flags: dynDialedConn, id: uintID(1)}},
					{rw: &conn{flags: dynDialedConn, id: uintID(2)}},
					{rw: &conn{flags: staticDialedConn, id: uintID(3)}},
					{rw: &conn{flags: staticDialedConn, id: uintID(4)}},
					{rw: &conn{flags: staticDialedConn, id: uintID(5)}},
				},
			},
			// 如果静态节点被丢弃，则应立即对其进行重拨，无论它是最初是静态的还是动态的。
			{
				peers: []*Peer{
					{rw: &conn{flags: dynDialedConn, id: uintID(1)}},
					{rw: &conn{flags: staticDialedConn, id: uintID(3)}},
					{rw: &conn{flags: staticDialedConn, id: uintID(5)}},
				},
				new: []task{
					&dialTask{flags: staticDialedConn, dest: &discover.Node{ID: uintID(2)}},
					&dialTask{flags: staticDialedConn, dest: &discover.Node{ID: uintID(4)}},
				},
			},
		},
	})
}

// 此测试检查静态对等体是否会在重新添加到静态列表时立即重拨。
func TestDialStaticAfterReset(t *testing.T) {
	wantStatic := []*discover.Node{
		{ID: uintID(1)},
		{ID: uintID(2)},
	}

	rounds := []round{
		// 为尚未连接的节点启动静态拨号。
		{
			peers: nil,
			new: []task{
				&dialTask{flags: staticDialedConn, dest: &discover.Node{ID: uintID(1)}},
				&dialTask{flags: staticDialedConn, dest: &discover.Node{ID: uintID(2)}},
			},
		},
		// 没有新的拨号任务，所有对等都已连接。
		{
			peers: []*Peer{
				{rw: &conn{flags: staticDialedConn, id: uintID(1)}},
				{rw: &conn{flags: staticDialedConn, id: uintID(2)}},
			},
			done: []task{
				&dialTask{flags: staticDialedConn, dest: &discover.Node{ID: uintID(1)}},
				&dialTask{flags: staticDialedConn, dest: &discover.Node{ID: uintID(2)}},
			},
			new: []task{
				&waitExpireTask{Duration: 30 * time.Second},
			},
		},
	}
	dTest := dialtest{
		init:   newDialState(wantStatic, nil, fakeTable{}, 0, nil),
		rounds: rounds,
	}
	runDialTest(t, dTest)
	for _, n := range wantStatic {
		dTest.init.removeStatic(n)
		dTest.init.addStatic(n)
	}
	// 在不删除同伴的情况下，最近会拨打他们
	runDialTest(t, dTest)
}

// 此测试检查过去的刻度盘是否在一段时间内未重试。
func TestDialStateCache(t *testing.T) {
	wantStatic := []*discover.Node{
		{ID: uintID(1)},
		{ID: uintID(2)},
		{ID: uintID(3)},
	}

	runDialTest(t, dialtest{
		init: newDialState(wantStatic, nil, fakeTable{}, 0, nil),
		rounds: []round{
			// 为尚未连接的节点启动静态拨号。
			{
				peers: nil,
				new: []task{
					&dialTask{flags: staticDialedConn, dest: &discover.Node{ID: uintID(1)}},
					&dialTask{flags: staticDialedConn, dest: &discover.Node{ID: uintID(2)}},
					&dialTask{flags: staticDialedConn, dest: &discover.Node{ID: uintID(3)}},
				},
			},
			// 此回合没有启动任何新任务，因为所有静态节点都已连接或仍在拨打。
			{
				peers: []*Peer{
					{rw: &conn{flags: staticDialedConn, id: uintID(1)}},
					{rw: &conn{flags: staticDialedConn, id: uintID(2)}},
				},
				done: []task{
					&dialTask{flags: staticDialedConn, dest: &discover.Node{ID: uintID(1)}},
					&dialTask{flags: staticDialedConn, dest: &discover.Node{ID: uintID(2)}},
				},
			},
			// 启动补救任务以等待节点3的历史记录条目到期。
			{
				peers: []*Peer{
					{rw: &conn{flags: dynDialedConn, id: uintID(1)}},
					{rw: &conn{flags: dynDialedConn, id: uintID(2)}},
				},
				done: []task{
					&dialTask{flags: staticDialedConn, dest: &discover.Node{ID: uintID(3)}},
				},
				new: []task{
					&waitExpireTask{Duration: 14 * time.Second},
				},
			},
			// 仍在等待节点3的条目在缓存中到期。
			{
				peers: []*Peer{
					{rw: &conn{flags: dynDialedConn, id: uintID(1)}},
					{rw: &conn{flags: dynDialedConn, id: uintID(2)}},
				},
			},
			// 节点3的缓存条目已过期并重试。
			{
				peers: []*Peer{
					{rw: &conn{flags: dynDialedConn, id: uintID(1)}},
					{rw: &conn{flags: dynDialedConn, id: uintID(2)}},
				},
				new: []task{
					&dialTask{flags: staticDialedConn, dest: &discover.Node{ID: uintID(3)}},
				},
			},
		},
	})
}

func TestDialResolve(t *testing.T) {
	resolved := discover.NewNode(uintID(1), net.IP{127, 0, 55, 234}, 3333, 4444)
	table := &resolveMock{answer: resolved}
	state := newDialState(nil, nil, table, 0, nil)

	// 检查是否使用不完整的 ID 生成任务。
	dest := discover.NewNode(uintID(1), nil, 0, 0)
	state.addStatic(dest)
	tasks := state.newTasks(0, nil, time.Time{})
	if !reflect.DeepEqual(tasks, []task{&dialTask{flags: staticDialedConn, dest: dest}}) {
		t.Fatalf("预期拨号任务，获得 %#v", tasks)
	}

	// 现在运行任务，它应该解析一次ID。
	config := Config{Dialer: TCPDialer{&net.Dialer{Deadline: time.Now().Add(-5 * time.Minute)}}}
	srv := &Server{ntab: table, Config: config}
	tasks[0].Do(srv)
	if !reflect.DeepEqual(table.resolveCalls, []discover.NodeID{dest.ID}) {
		t.Fatalf("错误的解决电话，得到%v", table.resolveCalls)
	}

	// 将其报告为拨号程序，它应更新静态节点记录。
	state.taskDone(tasks[0], time.Now())
	if state.static[uintID(1)].dest != resolved {
		t.Fatalf("state.dest not updated")
	}
}

// 比较任务列表但不关心订单。
func sametasks(a, b []task) bool {
	if len(a) != len(b) {
		return false
	}
next:
	for _, ta := range a {
		for _, tb := range b {
			if reflect.DeepEqual(ta, tb) {
				continue next
			}
		}
		return false
	}
	return true
}

func uintID(i uint32) discover.NodeID {
	var id discover.NodeID
	binary.BigEndian.PutUint32(id[:], i)
	return id
}

// 为 TestDialResolve 实现 discoverTable
type resolveMock struct {
	resolveCalls []discover.NodeID
	answer       *discover.Node
}

func (t *resolveMock) Resolve(id discover.NodeID) *discover.Node {
	t.resolveCalls = append(t.resolveCalls, id)
	return t.answer
}

func (t *resolveMock) Self() *discover.Node                     { return new(discover.Node) }
func (t *resolveMock) Close()                                   {}
func (t *resolveMock) Bootstrap([]*discover.Node)               {}
func (t *resolveMock) Lookup(discover.NodeID) []*discover.Node  { return nil }
func (t *resolveMock) ReadRandomNodes(buf []*discover.Node) int { return 0 }
