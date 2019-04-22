



package event

import (
	"github.com/aidoc/go-aidoc/lib/i18"
	"github.com/ethereum/go-ethereum/event"
)

func ExampleNewSubscription() {
	// 创建一个在 ch 上发送 10 个整数的订阅。
	ch := make(chan int)
	sub := event.NewSubscription(func(quit <-chan struct{}) error {
		for i := 0; i < 10; i++ {
			select {
			case ch <- i:
			case <-quit:
				i18.I18_print.Println("unsubscribed")
				return nil
			}
		}
		return nil
	})

	// 这是消费者 它读取5个整数，然后中止订阅。
	// 请注意，Unsubscribe会在生产者关闭之前等待。
	for i := range ch {
		i18.I18_print.Println(i)
		if i == 4 {
			sub.Unsubscribe()
			break
		}
	}
	// 输出的:
	// 0
	// 1
	// 2
	// 3
	// 4
	// 退订
}
