package event

import (
	"errors"
	"reflect"
	"sync"
)

var errBadChannel = errors.New("事件: Subscribe 参数没有可发送的通道类型")

// Feed实现一对多订阅，其中事件的载体是一个通道。
// 发送给Feed的值同时传送到所有订阅的频道。
//
// Feed只能用于单一类型。 类型由第一个发送或确定
// 订阅操作。 如果类型不匹配，对这些方法的后续调用会发生混乱。
//
// 零值可以使用了。
type Feed struct {
	once      sync.Once        // 确保init只运行一次
	sendLock  chan struct{}    // sendLock有一个单元素缓冲区，在保持时为空。它保护sendCases。
	removeSub chan interface{} // 中断发送
	sendCases caseList         // 发送使用的有效选择案例集

	// 收件箱保存新订阅的频道，直到它们被添加到sendCases。
	mu     sync.Mutex
	inbox  caseList
	etype  reflect.Type
	closed bool
}

// 这是sendCases中第一个实际订阅频道的索引。
// sendCases [0]是 removeSub 通道的 SelectRecv 案例。
const firstSubSendCase = 1

type feedTypeError struct {
	got, want reflect.Type
	op        string
}

func (e feedTypeError) Error() string {
	return "事件: 错误的类型" + e.op + " 得到 " + e.got.String() + ", 想 " + e.want.String()
}

func (f *Feed) init() {
	f.removeSub = make(chan interface{})
	f.sendLock = make(chan struct{}, 1)
	f.sendLock <- struct{}{}
	f.sendCases = caseList{{Chan: reflect.ValueOf(f.removeSub), Dir: reflect.SelectRecv}}
}
// 订阅会向Feed添加频道。 未来的发送将在通道上发送，直到订阅被取消。
// 添加的所有通道必须具有相同的元素类型。
//
// 通道应该有足够的缓冲区空间，以避免阻塞其他用户。
// 缓慢的订阅者不会被删除。

func (f *Feed) Subscribe(channel interface{}) Subscription {
	f.once.Do(f.init)

	chanval := reflect.ValueOf(channel)
	chantyp := chanval.Type()
	if chantyp.Kind() != reflect.Chan || chantyp.ChanDir()&reflect.SendDir == 0 {
		panic(errBadChannel)
	}
	sub := &feedSub{feed: f, channel: chanval, err: make(chan error, 1)}

	f.mu.Lock()
	defer f.mu.Unlock()
	if !f.typecheck(chantyp.Elem()) {
		panic(feedTypeError{op: "Subscribe", got: chantyp, want: reflect.ChanOf(reflect.SendDir, f.etype)})
	}
	// 将选择案例添加到收件箱。
	// 下一个Send会将它添加到f.sendCases。
	cas := reflect.SelectCase{Dir: reflect.SelectSend, Chan: chanval}
	f.inbox = append(f.inbox, cas)
	return sub
}

// 注意：来电者必须持有f.mu
func (f *Feed) typecheck(typ reflect.Type) bool {
	if f.etype == nil {
		f.etype = typ
		return true
	}
	return f.etype == typ
}

func (f *Feed) remove(sub *feedSub) {
	// 首先从收件箱中删除，其中包含尚未添加到f.sendCases的频道。
	ch := sub.channel.Interface()
	f.mu.Lock()
	index := f.inbox.find(ch)
	if index != -1 {
		f.inbox = f.inbox.delete(index)
		f.mu.Unlock()
		return
	}
	f.mu.Unlock()

	select {
	case f.removeSub <- ch:
		// 发送将从 f.sendCases 中删除该频道。
	case <-f.sendLock:
		// No Send 正在进行中，现在删除了我们拥有发送锁定的频道。
		f.sendCases = f.sendCases.delete(f.sendCases.find(ch))
		f.sendLock <- struct{}{}
	}
}
// 发送到所有订阅的频道。 它返回发送者的订阅数。
func (f *Feed) Send(value interface{}) (nsent int) {
	rvalue := reflect.ValueOf(value)

	f.once.Do(f.init)
	<-f.sendLock

	//当获取到发送锁后，把inbox的数据附加到send case. inbox是一个caselist
	f.mu.Lock()
	f.sendCases = append(f.sendCases, f.inbox...)
	f.inbox = nil

	if !f.typecheck(rvalue.Type()) {
		f.sendLock <- struct{}{}
		panic(feedTypeError{op: "Send", got: rvalue.Type(), want: f.etype})
	}
	f.mu.Unlock()

	// 设置传值到所有的监听channel
	for i := firstSubSendCase; i < len(f.sendCases); i++ {
		f.sendCases[i].Send = rvalue
	}

	// 发送直到选择除removeSub之外的所有通道。 'cases'跟踪sendCases的前缀。
	// 当发送成功时，相应的大小写移动到'个案'的末尾，并缩小一个元素。
	cases := f.sendCases
	for {
		// 快速路径：在添加到选择集之前尝试无阻塞地发送。
		// 如果订阅者足够快并且有可用的缓冲区空间，通常应该会成功。
		for i := firstSubSendCase; i < len(cases); i++ {
			if cases[i].Chan.TrySend(rvalue) {
				nsent++
				cases = cases.deactivate(i)
				i--
			}
		}
		if len(cases) == firstSubSendCase {
			break
		}
		// 选择所有接收器，等待它们解除阻塞。
		chosen, recv, _ := reflect.Select(cases)
		if chosen == 0 /* <-f.removeSub */ {
			index := f.sendCases.find(recv.Interface())
			f.sendCases = f.sendCases.delete(index)
			if index >= 0 && index < len(cases) {
				// 收缩'案件'也是因为拆除的案件仍然有效。
				cases = f.sendCases[:len(cases)-1]
			}
		} else {
			cases = cases.deactivate(chosen)
			nsent++
		}
	}
	//忘记发送的值并移交发送锁。
	for i := firstSubSendCase; i < len(f.sendCases); i++ {
		f.sendCases[i].Send = reflect.Value{}
	}
	f.sendLock <- struct{}{}
	return nsent
}


// func (cs caseList) String() string {
//     s := "["
//     for i, cas := range cs {
//             if i != 0 {
//                     s += ", "
//             }
//             switch cas.Dir {
//             case reflect.SelectSend:
//                     s += i18.I18_print.Sprintf("%v<-", cas.Chan.Interface())
//             case reflect.SelectRecv:
//                     s += i18.I18_print.Sprintf("<-%v", cas.Chan.Interface())
//             }
//     }
//     return s + "]"
// }
