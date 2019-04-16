



package event

import (
	"fmt"
	"reflect"
	"sync"
	"testing"
	"time"
	"github.com/aidoc/go-aidoc/lib/i18"
)

func TestFeedPanics(t *testing.T) {
	{
		var f Feed
		f.Send(int(2))
		want := feedTypeError{op: "Send", got: reflect.TypeOf(uint64(0)), want: reflect.TypeOf(int(0))}
		if err := checkPanic(want, func() { f.Send(uint64(2)) }); err != nil {
			t.Error(err)
		}
	}
	{
		var f Feed
		ch := make(chan int)
		f.Subscribe(ch)
		want := feedTypeError{op: "Send", got: reflect.TypeOf(uint64(0)), want: reflect.TypeOf(int(0))}
		if err := checkPanic(want, func() { f.Send(uint64(2)) }); err != nil {
			t.Error(err)
		}
	}
	{
		var f Feed
		f.Send(int(2))
		want := feedTypeError{op: "Subscribe", got: reflect.TypeOf(make(chan uint64)), want: reflect.TypeOf(make(chan<- int))}
		if err := checkPanic(want, func() { f.Subscribe(make(chan uint64)) }); err != nil {
			t.Error(err)
		}
	}
	{
		var f Feed
		if err := checkPanic(errBadChannel, func() { f.Subscribe(make(<-chan int)) }); err != nil {
			t.Error(err)
		}
	}
	{
		var f Feed
		if err := checkPanic(errBadChannel, func() { f.Subscribe(int(0)) }); err != nil {
			t.Error(err)
		}
	}
}

func checkPanic(want error, fn func()) (err error) {
	defer func() {
		panic := recover()
		if panic == nil {
			err = fmt.Errorf(i18.I18_print.Sprintf("没有恐慌"))
		} else if !reflect.DeepEqual(panic, want) {
			err = fmt.Errorf(i18.I18_print.Sprintf("惊慌失措错误：获得 %q，想要 %q", panic, want))
		}
	}()
	fn()
	return nil
}

func TestFeed(t *testing.T) {
	var feed Feed
	var done, subscribed sync.WaitGroup
	subscriber := func(i int) {
		defer done.Done()

		subchan := make(chan int)
		sub := feed.Subscribe(subchan)
		timeout := time.NewTimer(2 * time.Second)
		subscribed.Done()

		select {
		case v := <-subchan:
			if v != 1 {
				t.Errorf("%d: 收到的价值 %d ，想要 1", i, v)
			}
		case <-timeout.C:
			t.Errorf("%d: 收到超时 ", i)
		}

		sub.Unsubscribe()
		select {
		case _, ok := <-sub.Err():
			if ok {
				t.Errorf("%d: 取消订阅后错误频道未关闭 ", i)
			}
		case <-timeout.C:
			t.Errorf("%d: 取消订阅超时 ", i)
		}
	}

	const n = 1000
	done.Add(n)
	subscribed.Add(n)
	for i := 0; i < n; i++ {
		go subscriber(i)
	}
	subscribed.Wait()
	if nsent := feed.Send(1); nsent != n {
		t.Errorf("首先发送 %d 次，想要 %d ", nsent, n)
	}
	if nsent := feed.Send(2); nsent != 0 {
		t.Errorf("第二次发送 %d 次，想要 0", nsent)
	}
	done.Wait()
}

func TestFeedSubscribeSameChannel(t *testing.T) {
	var (
		feed Feed
		done sync.WaitGroup
		ch   = make(chan int)
		sub1 = feed.Subscribe(ch)
		sub2 = feed.Subscribe(ch)
		_    = feed.Subscribe(ch)
	)
	expectSends := func(value, n int) {
		if nsent := feed.Send(value); nsent != n {
			t.Errorf("发送 %d 次，想要 %d", nsent, n)
		}
		done.Done()
	}
	expectRecv := func(wantValue, n int) {
		for i := 0; i < n; i++ {
			if v := <-ch; v != wantValue {
				t.Errorf("收到 %d，想要 %d", v, wantValue)
			}
		}
	}

	done.Add(1)
	go expectSends(1, 3)
	expectRecv(1, 3)
	done.Wait()

	sub1.Unsubscribe()

	done.Add(1)
	go expectSends(2, 2)
	expectRecv(2, 2)
	done.Wait()

	sub2.Unsubscribe()

	done.Add(1)
	go expectSends(3, 1)
	expectRecv(3, 1)
	done.Wait()
}

func TestFeedSubscribeBlockedPost(t *testing.T) {
	var (
		feed   Feed
		nsends = 2000
		ch1    = make(chan int)
		ch2    = make(chan int)
		wg     sync.WaitGroup
	)
	defer wg.Wait()

	feed.Subscribe(ch1)
	wg.Add(nsends)
	for i := 0; i < nsends; i++ {
		go func() {
			feed.Send(99)
			wg.Done()
		}()
	}

	sub2 := feed.Subscribe(ch2)
	defer sub2.Unsubscribe()

	// 当ch1 收到 N 次时我们就完成了。
	// ch2 上的接收次数取决于调度。
	for i := 0; i < nsends; {
		select {
		case <-ch1:
			i++
		case <-ch2:
		}
	}
}

func TestFeedUnsubscribeBlockedPost(t *testing.T) {
	var (
		feed   Feed
		nsends = 200
		chans  = make([]chan int, 2000)
		subs   = make([]Subscription, len(chans))
		bchan  = make(chan int)
		bsub   = feed.Subscribe(bchan)
		wg     sync.WaitGroup
	)
	for i := range chans {
		chans[i] = make(chan int, nsends)
	}

	// 排队一些发送。 当没有读取 bchan 时，这些都不能取得进展。
	wg.Add(nsends)
	for i := 0; i < nsends; i++ {
		go func() {
			feed.Send(99)
			wg.Done()
		}()
	}
	// 订阅其他频道。
	for i, ch := range chans {
		subs[i] = feed.Subscribe(ch)
	}
	// 再次取消订阅。
	for _, sub := range subs {
		sub.Unsubscribe()
	}
	// 取消区块发送。
	bsub.Unsubscribe()
	wg.Wait()
}

// 检查在发送期间取消订阅频道即使该频道已经发送也是如此。
func TestFeedUnsubscribeSentChan(t *testing.T) {
	var (
		feed Feed
		ch1  = make(chan int)
		ch2  = make(chan int)
		sub1 = feed.Subscribe(ch1)
		sub2 = feed.Subscribe(ch2)
		wg   sync.WaitGroup
	)
	defer sub2.Unsubscribe()

	wg.Add(1)
	go func() {
		feed.Send(0)
		wg.Done()
	}()

	// 等待ch1上的值。
	<-ch1
	// 取消订阅 ch1，将其从发送案例中删除。
	sub1.Unsubscribe()

	// 收到 ch2，完成发送。
	<-ch2
	wg.Wait()

	// 重新发送。
	// 这应该仅发送到ch2，因此一旦在ch2上接收到值，等待组将解除阻塞。
	wg.Add(1)
	go func() {
		feed.Send(0)
		wg.Done()
	}()
	<-ch2
	wg.Wait()
}

func TestFeedUnsubscribeFromInbox(t *testing.T) {
	var (
		feed Feed
		ch1  = make(chan int)
		ch2  = make(chan int)
		sub1 = feed.Subscribe(ch1)
		sub2 = feed.Subscribe(ch1)
		sub3 = feed.Subscribe(ch2)
	)
	if len(feed.inbox) != 3 {
		t.Errorf("订阅后收件箱长度 ！= 3")
	}
	if len(feed.sendCases) != 1 {
		t.Errorf("取消订阅后，sendCases非空")
	}

	sub1.Unsubscribe()
	sub2.Unsubscribe()
	sub3.Unsubscribe()
	if len(feed.inbox) != 0 {
		t.Errorf("取消订阅后收件箱非空")
	}
	if len(feed.sendCases) != 1 {
		t.Errorf("取消订阅后，sendCases非空")
	}
}

func BenchmarkFeedSend1000(b *testing.B) {
	var (
		done  sync.WaitGroup
		feed  Feed
		nsubs = 1000
	)
	subscriber := func(ch <-chan int) {
		for i := 0; i < b.N; i++ {
			<-ch
		}
		done.Done()
	}
	done.Add(nsubs)
	for i := 0; i < nsubs; i++ {
		ch := make(chan int, 200)
		feed.Subscribe(ch)
		go subscriber(ch)
	}

	// 实际基准。
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if feed.Send(i) != nsubs {
			panic("错误的发送次数")
		}
	}

	b.StopTimer()
	done.Wait()
}
