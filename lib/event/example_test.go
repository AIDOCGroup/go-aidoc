

package event

import "github.com/aidoc/go-aidoc/lib/i18"
import (
	"github.com/aidoc/go-aidoc/lib/logger"
)

func ExampleTypeMux() {
	type someEvent struct{ I int }
	type otherEvent struct{ S string }
	type yetAnotherEvent struct{ X, Y int }

	var mux TypeMux

	// 启动订阅者。
	done := make(chan struct{})
	sub := mux.Subscribe(someEvent{}, otherEvent{})
	go func() {
		for event := range sub.Chan() {
			logger.InfoF(i18.I18_print.GetValue("收到: %#v\n"), event.Data)
		}
		logger.Info("处理完毕")
		close(done)
	}()

	// 发布一些活动。
	mux.Post(someEvent{5})
	mux.Post(yetAnotherEvent{X: 3, Y: 4})
	mux.Post(someEvent{6})
	mux.Post(otherEvent{"whoa"})

	// 停止关闭所有订阅频道。
	// 订阅者goroutine将打印“完成”并退出。
	mux.Stop()

	// 等待订户返回。
	<-done
	//输出：
	//收到：event.someEvent {I：5}
	//收到：event.someEvent {I：6}
	//收到：event.otherEvent {S：“whoa”}
	//完成

}
