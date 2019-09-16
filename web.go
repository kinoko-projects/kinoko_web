package kinoko_web

import (
	"container/list"
	"context"
	"github.com/kinoko-projects/kinoko"
)

func (s *HttpServer) Start(ctx context.Context) {
	s.StartServer()
	kinoko.Application.Exit()
}

func (s *HttpServer) Initialize() error {
	s.handlers = &RequestHandler{responseResolver: list.New()}
	controllers := kinoko.Application.GetImplementedSpores((*HttpController)(nil))
	for _, controller := range controllers {
		controller.(HttpController).Mapping(s)
	}

	interceptors := kinoko.Application.GetImplementedSpores((*Interceptor)(nil))
	for _, interceptor := range interceptors {
		s.AddInterceptor(interceptor.(Interceptor))
	}

	responseResolvers := kinoko.Application.GetImplementedSpores((*ResponseResolver)(nil))
	for _, responseResolver := range responseResolvers {
		s.AddResponseResolver(responseResolver.(ResponseResolver))
	}

	return nil
}
