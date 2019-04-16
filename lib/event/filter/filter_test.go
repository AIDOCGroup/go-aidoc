

package filter

import (
	"testing"
	"time"
)

// 简单测试以检查基线匹配/不匹配过滤是否有效。
func TestFilters(t *testing.T) {
	fm := New()
	fm.Start()

	// 注册两个过滤器以捕获发布的数据
	first := make(chan struct{})
	fm.Install(Generic{
		Str1: "hello",
		Fn: func(data interface{}) {
			first <- struct{}{}
		},
	})
	second := make(chan struct{})
	fm.Install(Generic{
		Str1: "hello1",
		Str2: "hello",
		Fn: func(data interface{}) {
			second <- struct{}{}
		},
	})

	//发布一个只应与第一个过滤器匹配的事件
	fm.Notify(Generic{Str1: "hello"}, true)
	fm.Stop()

	// 确保只有匹配的过滤器触发
	select {
	case <-first:
	case <-time.After(100 * time.Millisecond):
		t.Error("匹配过滤器超时")
	}
	select {
	case <-second:
		t.Error("不匹配滤波器发射")
	case <-time.After(100 * time.Millisecond):
	}
}
