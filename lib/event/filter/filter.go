

//包 过滤器 实现事件 过滤器。
package filter

import "reflect"

type Filter interface {
	Compare(Filter) bool
	Trigger(data interface{})
}

type FilterEvent struct {
	filter Filter
	data   interface{}
}

type Filters struct {
	id       int
	watchers map[int]Filter     // 观察者 过滤器类型
	ch       chan FilterEvent   // 过滤事件

	quit chan struct{}
}

// 创建 过滤器
func New() *Filters {
	return &Filters{
		ch:       make(chan FilterEvent),
		watchers: make(map[int]Filter),
		quit:     make(chan struct{}),
	}
}
// 启动  过滤器
func (f *Filters) Start() {
	go f.loop()
}

// 停用 过滤器
func (f *Filters) Stop() {
	close(f.quit)
}

// 通知 报信
func (f *Filters) Notify(filter Filter, data interface{}) {
	f.ch <- FilterEvent{filter, data}
}

// 安装 （设置）
func (f *Filters) Install(watcher Filter) int {
	f.watchers[f.id] = watcher
	f.id++

	return f.id - 1
}

// 卸载
func (f *Filters) Uninstall(id int) {
	delete(f.watchers, id)
}

// 循环
func (f *Filters) loop() {
out:
	for {
		select {
		case <-f.quit:
			break out
		case event := <-f.ch:
			for _, watcher := range f.watchers {
				if reflect.TypeOf(watcher) == reflect.TypeOf(event.filter) {
					if watcher.Compare(event.filter) {
						watcher.Trigger(event.data)
					}
				}
			}
		}
	}
}

// 比赛（ 匹配 ）
func (f *Filters) Match(a, b Filter) bool {
	return reflect.TypeOf(a) == reflect.TypeOf(b) && a.Compare(b)
}

//获取
func (f *Filters) Get(i int) Filter {
	return f.watchers[i]
}
