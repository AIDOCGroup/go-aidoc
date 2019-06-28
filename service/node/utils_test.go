

// 包含测试使用的一批实用程序类型声明。 由于节点在唯一类型上运行，因此需要很多节点来检查各种功能。

package node

import (
	"reflect"

	"github.com/aidoc/go-aidoc/service/p2p"
	"github.com/aidoc/go-aidoc/service/rpc"
)

// NoopService是Service接口的一个简单实现。
type NoopService struct{}

func (s *NoopService) Protocols() []p2p.Protocol { return nil }
func (s *NoopService) APIs() []rpc.API           { return nil }
func (s *NoopService) Start(*p2p.Server) error   { return nil }
func (s *NoopService) Stop() error               { return nil }

func NewNoopService(*ServiceContext) (Service, error) { return new(NoopService), nil }

// 所有包装基本 NoopService 的服务集导致相同的方法签名但外部类型不同。
type NoopServiceA struct{ NoopService }
type NoopServiceB struct{ NoopService }
type NoopServiceC struct{ NoopService }

func NewNoopServiceA(*ServiceContext) (Service, error) { return new(NoopServiceA), nil }
func NewNoopServiceB(*ServiceContext) (Service, error) { return new(NoopServiceB), nil }
func NewNoopServiceC(*ServiceContext) (Service, error) { return new(NoopServiceC), nil }

// InstrumentedService 是 Service 的一个实现，所有接口方法都可以检测返回值和事件挂钩。
type InstrumentedService struct {
	protocols []p2p.Protocol
	apis      []rpc.API
	start     error
	stop      error

	protocolsHook func()
	startHook     func(*p2p.Server)
	stopHook      func()
}

func NewInstrumentedService(*ServiceContext) (Service, error) { return new(InstrumentedService), nil }

func (s *InstrumentedService) Protocols() []p2p.Protocol {
	if s.protocolsHook != nil {
		s.protocolsHook()
	}
	return s.protocols
}

func (s *InstrumentedService) APIs() []rpc.API {
	return s.apis
}

func (s *InstrumentedService) Start(server *p2p.Server) error {
	if s.startHook != nil {
		s.startHook(server)
	}
	return s.start
}

func (s *InstrumentedService) Stop() error {
	if s.stopHook != nil {
		s.stopHook()
	}
	return s.stop
}

// InstrumentingWrapper 是一种专门用于将通用 InstrumentedService 返回到返回特定包装的服务构造函数的方法。
type InstrumentingWrapper func(base ServiceConstructor) ServiceConstructor

func InstrumentingWrapperMaker(base ServiceConstructor, kind reflect.Type) ServiceConstructor {
	return func(ctx *ServiceContext) (Service, error) {
		obj, err := base(ctx)
		if err != nil {
			return nil, err
		}
		wrapper := reflect.New(kind)
		wrapper.Elem().Field(0).Set(reflect.ValueOf(obj).Elem())

		return wrapper.Interface().(Service), nil
	}
}

// 所有包装基础InstrumentedService的服务集导致相同的方法签名但外部类型不同。
type InstrumentedServiceA struct{ InstrumentedService }
type InstrumentedServiceB struct{ InstrumentedService }
type InstrumentedServiceC struct{ InstrumentedService }

func InstrumentedServiceMakerA(base ServiceConstructor) ServiceConstructor {
	return InstrumentingWrapperMaker(base, reflect.TypeOf(InstrumentedServiceA{}))
}

func InstrumentedServiceMakerB(base ServiceConstructor) ServiceConstructor {
	return InstrumentingWrapperMaker(base, reflect.TypeOf(InstrumentedServiceB{}))
}

func InstrumentedServiceMakerC(base ServiceConstructor) ServiceConstructor {
	return InstrumentingWrapperMaker(base, reflect.TypeOf(InstrumentedServiceC{}))
}

// OneMethodAPI是测试服务返回的单方法API处理程序。
type OneMethodAPI struct {
	fun func()
}

func (api *OneMethodAPI) TheOneMethod() {
	if api.fun != nil {
		api.fun()
	}
}
