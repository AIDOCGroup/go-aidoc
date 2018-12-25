package bloombits

import (
	"bytes"
	"math/big"
	"math/rand"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// 测试调度程序可以对基础提取程序进行重复数据删除和转发检索请求，并提供响应服务，与请求客户端或服务数据提取程序的并发性无关。
func TestSchedulerSingleClientSingleFetcher(t *testing.T) { testScheduler(t, 1, 1, 5000) }
func TestSchedulerSingleClientMultiFetcher(t *testing.T)  { testScheduler(t, 1, 10, 5000) }
func TestSchedulerMultiClientSingleFetcher(t *testing.T)  { testScheduler(t, 10, 1, 5000) }
func TestSchedulerMultiClientMultiFetcher(t *testing.T)   { testScheduler(t, 10, 10, 5000) }

func testScheduler(t *testing.T, clients int, fetchers int, requests int) {
	f := newScheduler(0)

	// 创建一批响应 bloom 位请求并将它们传递给调度程序的处理程序 goroutine。
	var fetchPend sync.WaitGroup
	fetchPend.Add(fetchers)
	defer fetchPend.Wait()

	fetch := make(chan *request, 16)
	defer close(fetch)

	var delivered uint32
	for i := 0; i < fetchers; i++ {
		go func() {
			defer fetchPend.Done()

			for req := range fetch {
				time.Sleep(time.Duration(rand.Intn(int(100 * time.Microsecond))))
				atomic.AddUint32(&delivered, 1)

				f.deliver([]uint64{
					req.section + uint64(requests), // 未请求的数据（确保它不会超出范围）
					req.section,                    // 请求的数据
					req.section,                    // 重复数据（确保它不会使任何事物双重关闭）
				}, [][]byte{
					{},
					new(big.Int).SetUint64(req.section).Bytes(),
					new(big.Int).SetUint64(req.section).Bytes(),
				})
			}
		}()
	}
	// 启动一批 goroutine 以同时运行调度任务
	quit := make(chan struct{})

	var pend sync.WaitGroup
	pend.Add(clients)

	for i := 0; i < clients; i++ {
		go func() {
			defer pend.Done()

			in := make(chan uint64, 16)
			out := make(chan []byte, 16)

			f.run(in, fetch, out, quit, &pend)

			go func() {
				for j := 0; j < requests; j++ {
					in <- uint64(j)
				}
				close(in)
			}()

			for j := 0; j < requests; j++ {
				bits := <-out
				if want := new(big.Int).SetUint64(uint64(j)).Bytes(); !bytes.Equal(bits, want) {
					t.Errorf("向量 %d：传递的内容不匹配：有 %x，想要 %x", j, bits, want)
				}
			}
		}()
	}
	pend.Wait()

	if have := atomic.LoadUint32(&delivered); int(have) != requests {
		t.Errorf("请求计数不匹配：有 %v,想要 %v", have, requests)
	}
}
