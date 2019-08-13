package kinoko_web

import (
	"sort"
	"sync"
)

type InterceptorAction int

const (
	_ InterceptorAction = iota

	// call next interceptorChain Routine
	// returned value must be nil
	Continue

	// block the request,
	// returned value can be assigned by the second parameter
	Block

	// skip other interceptors, enter normal handler, not recommended
	// returned value must be nil
	Skip
)

// eg: return Continue, nil
//	   return Block, "No Permission"
//	   return Skip, nil

type Interceptor interface {

	// higher prior
	Priority() int
	Intercept(ctx *RequestCtx, properties map[string]interface{}) (InterceptorAction, interface{})
}

type InterceptorChain struct {
	sync.Mutex
	interceptor []Interceptor
}

func NewInterceptorChain() *InterceptorChain {
	return &InterceptorChain{interceptor: []Interceptor{}}
}

func (r *InterceptorChain) AddInterceptor(interceptor Interceptor) {
	r.Lock()
	ins := append(r.interceptor, interceptor)
	sort.SliceStable(ins, func(i, j int) bool {
		return ins[i].Priority() < ins[j].Priority()
	})
	r.interceptor = ins
	r.Unlock()
}

// interceptorChain chain returns a bool tell if the request should be blocked or resumed
// second parameter will be treat as the response body
func (r *InterceptorChain) CallInterceptors(ctx *RequestCtx, properties map[string]interface{}) (bool, interface{}) {
	for _, i := range r.interceptor {
		action, ret := i.Intercept(ctx, properties)
		switch action {
		case Continue:
			if ret != nil {
				logger.Warn("Continued interceptors returns no nil value makes no sense")
			}
			continue
		case Block:
			return true, ret
		case Skip:
			if ret != nil {
				logger.Warn("Skipped interceptors returns no nil value makes no sense")
			}
			return false, nil
		}
	}
	return false, nil
}
