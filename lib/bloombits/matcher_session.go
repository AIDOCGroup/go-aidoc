package bloombits

import (
	"time"
	"sync"
	"context"
	"sync/atomic"
)

// MatcherSession由一个已启动的匹配器返回，用作主动运行匹配操作的终止符。
type MatcherSession struct {
	matcher *Matcher

	closer sync.Once     // 同步对象以确保我们只关闭一次
	quit   chan struct{} // 退出通道以请求管道终止
	kill   chan struct{} // 用于表示非正常强制关闭的术语通道

	ctx context.Context // 轻客户端用于中止过滤的上下文
	err atomic.Value    // 跟踪链中深层检索失败的全局错误

	pend sync.WaitGroup
}

// Close停止匹配进程并等待所有子进程在返回之前终止。
// 超时可用于正常关闭，允许在此时间之前完成当前正在运行的检索。
func (s *MatcherSession) Close() {
	s.closer.Do(func() {
		// 信号终止并等待所有 goroutines 拆除
		close(s.quit)
		time.AfterFunc(time.Second, func() { close(s.kill) })
		s.pend.Wait()
	})
}
//错误返回匹配会话期间遇到的任何失败。
func (s *MatcherSession) Error() error {
	if err := s.err.Load(); err != nil {
		return err.(error)
	}
	return nil
}
// AllocateRetrieval 为客户端进程分配一个bloom位索引，该客户端进程可以立即重新获取并获取分配
// 给该位的段内容，或者等待一段时间以便请求更多段。
func (s *MatcherSession) AllocateRetrieval() (uint, bool) {
	fetcher := make(chan uint)

	select {
	case <-s.quit:
		return 0, false
	case s.matcher.retrievers <- fetcher:
		bit, ok := <-fetcher
		return bit, ok
	}
}
// PendingSections 返回属于给定 bloom 位索引的待处理区段检索的数量。
func (s *MatcherSession) PendingSections(bit uint) int {
	fetcher := make(chan uint)

	select {
	case <-s.quit:
		return 0
	case s.matcher.counters <- fetcher:
		fetcher <- bit
		return int(<-fetcher)
	}
}

// AllocateSections将已分配的位任务队列的全部或部分分配给请求进程。
func (s *MatcherSession) AllocateSections(bit uint, count int) []uint64 {
	fetcher := make(chan *Retrieval)

	select {
	case <-s.quit:
		return nil
	case s.matcher.retrievals <- fetcher:
		task := &Retrieval{
			Bit:      bit,
			Sections: make([]uint64, count),
		}
		fetcher <- task
		return (<-fetcher).Sections
	}
}

// DeliverSections为要注入处理管道的特定bloom位索引提供一批段位向量。
func (s *MatcherSession) DeliverSections(bit uint, sections []uint64, bitsets [][]byte) {
	select {
	case <-s.kill:
		return
	case s.matcher.deliveries <- &Retrieval{Bit: bit, Sections: sections, Bitsets: bitsets}:
	}
}

// Multiplex轮询匹配器会话以获取重新执行任务，并将其多路复用到请求的检索队列中，以便与其他会话一起进行服务。
//
//此方法将区块会话的生命周期。 即使在会话结束后，任何正在进行的请求都需要回复！ 在这种情况下，空的反应很好。
func (s *MatcherSession) Multiplex(batch int, wait time.Duration, mux chan chan *Retrieval) {
	for {
		// 分配新的bloom位索引以检索数据，完成后停止
		bit, ok := s.AllocateRetrieval()
		if !ok {
			return
		}
		// 如果我们低于批量限制，则分配位，节流一点
		if s.PendingSections(bit) < batch {
			select {
			case <-s.quit:
				// 会话终止，我们无法有意义地服务，中止
				s.AllocateSections(bit, 0)
				s.DeliverSections(bit, []uint64{}, [][]byte{})
				return

			case <-time.After(wait):
				// 限制，获取任何可用的东西
			}
		}
		// 分配尽可能多的处理和请求服务
		sections := s.AllocateSections(bit, batch)
		request := make(chan *Retrieval)

		select {
		case <-s.quit:
			// 会话终止，我们无法有意义地服务，中止
			s.DeliverSections(bit, sections, make([][]byte, len(sections)))
			return

		case mux <- request:
			// 接受检索，必须在我们中止之前到达
			request <- &Retrieval{Bit: bit, Sections: sections, Context: s.ctx}

			result := <-request
			if result.Error != nil {
				s.err.Store(result.Error)
				s.Close()
			}
			s.DeliverSections(result.Bit, result.Sections, result.Bitsets)
		}
	}
}
