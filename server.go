/*
 * Copyright 2019 Azz. All rights reserved.
 * Use of this source code is governed by a GPL-3.0
 * license that can be found in the LICENSE file.
 */

package kinoko_web

import (
	"container/list"
	"context"
	"encoding/json"
	"fmt"
	"github.com/kinoko-projects/kinoko"
	"html/template"
	"net/http"
	"os"
	"os/signal"
	"regexp"
	"runtime"
	"strings"
	"syscall"
	"time"
)

type RequestHandlerFunc func(ctx *RequestCtx) interface{}

type HttpConfig struct {
	Address           string        `inject:"kinoko.web.server.address:"`
	ReadTimeout       time.Duration `inject:"kinoko.web.server.read-timeout"`
	ReadHeaderTimeout time.Duration `inject:"kinoko.web.server.read-header-timeout"`
	WriteTimeout      time.Duration `inject:"kinoko.web.server.write-timeout"`
	IdleTimeout       time.Duration `inject:"kinoko.web.server.idle-timeout"`
}

type SSLConfig struct {
	EnableSSL bool   `inject:"kinoko.web.ssl.enable:false"`
	CertFile  string `inject:"kinoko.web.ssl.cert-file:"`
	KeyFile   string `inject:"kinoko.web.ssl.key-file:"`
}

type HttpServer struct {
	handlers   *RequestHandler
	HttpConfig *HttpConfig `inject:""`
	SSLConfig  *SSLConfig  `inject:""`
}

type RequestMapper interface {
	GET(pattern string, handler RequestHandlerFunc, properties ...HandlerProperties) RequestMapper
	PUT(pattern string, handler RequestHandlerFunc, properties ...HandlerProperties) RequestMapper
	DELETE(pattern string, handler RequestHandlerFunc, properties ...HandlerProperties) RequestMapper
	POST(pattern string, handler RequestHandlerFunc, properties ...HandlerProperties) RequestMapper
	Mapping(method RequestMethod, pattern string, handler RequestHandlerFunc, properties ...HandlerProperties) RequestMapper
}

type HttpController interface {
	Mapping(mapper RequestMapper)
}

// customize response by handle it manually return true if handled
type ResponseResolver interface {
	ResolveResponse(v interface{}, wr http.ResponseWriter) bool
}

// customizable resolver for any http request, put any custom properties on Property field, eg: query paging
type RequestResolver interface {
	ResolveRequest(ctx *RequestCtx)
}

type RequestHandler struct {
	mapping          map[RequestMethod]*prefixNode
	interceptorChain InterceptorChain
	responseResolver *list.List
}

type RequestMethod string

var logger = kinoko.NewLogger("HttpServer")

const (
	Get     RequestMethod = "GET"
	Post    RequestMethod = "POST"
	Put     RequestMethod = "PUT"
	Delete  RequestMethod = "DELETE"
	Head    RequestMethod = "HEAD"
	Options RequestMethod = "OPTIONS"
)

type RequestMapping struct {
	method  RequestMethod
	depth   int
	pattern string
	handler RequestHandlerFunc
}

type prefixNode struct {
	mapped      bool
	matchAll    bool
	prefix      string
	placeholder string
	handler     RequestHandlerFunc
	properties  map[string]interface{}
	parent      *prefixNode
	children    map[string]*prefixNode
}

type HandlerProperties struct {
	k string
	v interface{}
}

var urlFormatRegexp []*regexp.Regexp

func init() {
	r1, _ := regexp.Compile("\\.+/")
	r2, _ := regexp.Compile("/{2,}")
	urlFormatRegexp = []*regexp.Regexp{r1, r2}
}

func (s *HttpServer) GET(pattern string, handler RequestHandlerFunc, properties ...HandlerProperties) RequestMapper {
	return s.Mapping(Get, pattern, handler, properties...)
}
func (s *HttpServer) POST(pattern string, handler RequestHandlerFunc, properties ...HandlerProperties) RequestMapper {
	return s.Mapping(Post, pattern, handler, properties...)
}
func (s *HttpServer) DELETE(pattern string, handler RequestHandlerFunc, properties ...HandlerProperties) RequestMapper {
	return s.Mapping(Delete, pattern, handler, properties...)
}
func (s *HttpServer) PUT(pattern string, handler RequestHandlerFunc, properties ...HandlerProperties) RequestMapper {
	return s.Mapping(Put, pattern, handler, properties...)
}

func (s *HttpServer) Mapping(method RequestMethod, pattern string, handler RequestHandlerFunc, properties ...HandlerProperties) RequestMapper {

	if s.handlers.mapping == nil {
		s.handlers.mapping = map[RequestMethod]*prefixNode{
			Get:     {prefix: "", children: map[string]*prefixNode{}},
			Put:     {prefix: "", children: map[string]*prefixNode{}},
			Post:    {prefix: "", children: map[string]*prefixNode{}},
			Delete:  {prefix: "", children: map[string]*prefixNode{}},
			Head:    {prefix: "", children: map[string]*prefixNode{}},
			Options: {prefix: "", children: map[string]*prefixNode{}}}

	}
	// format pattern
	pattern = strings.TrimSpace(pattern)
	if len(pattern) == 0 {
		panic("empty pattern")
	}
	if pattern[0] == '/' {
		if len(pattern) == 1 {
			pattern = ""
		} else {
			pattern = pattern[1:]
		}
	}
	prefixes := strings.Split(pattern, "/")
	currentNode := s.handlers.mapping[method]
	for i := 0; pattern != "" && i < len(prefixes); i++ {
		prefix := prefixes[i]
		placeholder := ""

		//node already match all patterns
		if currentNode.matchAll {
			panic("ambiguous mapping")
		}

		if len(prefix) > 0 && prefix[0] == ':' {
			placeholder = prefix[1:]
			prefix = "*"
		} else if len(prefix) > 0 && prefix[0] == '*' {
			placeholder = prefix[1:]
			prefix = "*"
			if i < len(prefix)-1 {
				logger.Error(pattern, "full pattern placeholder must be the end")
				return s
			}

			if currentNode.children[prefix] != nil {
				logger.Error(pattern, "ambiguous mapping")
				return s
			}

			nextNode := &prefixNode{prefix: prefix, placeholder: placeholder,
				mapped: false, children: nil, matchAll: true}
			currentNode.children[prefix] = nextNode
			currentNode = nextNode
			break

		}

		nextNode := currentNode.children[prefix]

		//allocate children node
		if nextNode == nil {
			nextNode = &prefixNode{prefix: prefix, placeholder: placeholder,
				mapped: false, children: map[string]*prefixNode{}, matchAll: false}
			currentNode.children[prefix] = nextNode
		}
		currentNode = nextNode
	}

	if currentNode.mapped {
		logger.Error(pattern, "ambiguous mapping")
	}

	currentNode.mapped = true
	currentNode.handler = handler

	currentNode.properties = map[string]interface{}{}
	for _, property := range properties {
		currentNode.properties[property.k] = property.v
	}

	logger.Info("URL Mapped", method, "/"+pattern)
	return s
}

func (c *RequestHandler) ServeHTTP(wr http.ResponseWriter, r *http.Request) {
	var obj interface{} = nil
	pv := make(map[string]string)
	url := r.URL.Path

	//format the url
	for _, reg := range urlFormatRegexp {
		url = reg.ReplaceAllString(url, "/")
	}

	split := strings.Split(url[1:], "/")
	currentNode := c.mapping[RequestMethod(r.Method)]

	for i := 0; url[1:] != "" && i < len(split); i++ {
		s := split[i]
		node := currentNode.children[s]

		if node == nil {
			node = currentNode.children["*"]
			if node == nil {
				currentNode = nil
				break
			}
			if node.matchAll {
				pv[node.placeholder] = strings.Join(split[i:], "/")
				currentNode = node
				break
			} else {
				pv[node.placeholder] = s
			}
		}
		currentNode = node
	}

	//mapped
	if currentNode != nil && currentNode.mapped {

		ctx := NewRequestCtx(r.URL.Query(), pv, r, r.MultipartForm, wr)

		//recover from any exception
		defer func() {
			//panic
			if err := recover(); err != nil {
				if ctx.SQL != nil {
					ctx.SQL.Rollback() //rollback any uncommitted transaction
				}
				HttpError(wr, http.StatusInternalServerError, fmt.Sprint(err), true)
			}
		}()

		var intercepted bool
		// firstly, handle with interceptorChain
		intercepted, obj = c.interceptorChain.CallInterceptors(ctx, currentNode.properties)

		if !intercepted {
			obj = currentNode.handler(ctx)
		}

		//find a proper response wrapper
		for e := c.responseResolver.Front(); e != nil; e = e.Next() {
			if e.Value.(ResponseResolver).ResolveResponse(obj, wr) {
				return
			}
		}
		//commit any uncommitted transaction
		if ctx.SQL != nil {
			ctx.SQL.Commit()
		}
		//default wrapper
		c.defaultResponseResolver(obj, wr)
	} else {
		//unmapped url
		http.NotFound(wr, r)
		return
	}

}

// append to the top of response
func (s *HttpServer) AddResponseResolver(wrapper ResponseResolver) {
	s.handlers.responseResolver.PushFront(wrapper)
}

//default response wrapper
func (c *RequestHandler) defaultResponseResolver(v interface{}, wr http.ResponseWriter) bool {
	var err error

	//Nil
	if v == nil {
		return true
	}

	//Unhandled error
	if e, ok := v.(error); ok {
		HttpError(wr, http.StatusInternalServerError, e.Error(), false)
		return true
	}

	switch (v).(type) {
	case string:
		wr.Header().Set("Content-Type", "text/plain")
		_, err = wr.Write([]byte((v).(string))) //return origin value if string
	default:
		wr.Header().Set("Content-Type", "application/json")
		if bytes, err := json.Marshal(v); err != nil {
			http.Error(wr, err.Error(), 500)
		} else {
			_, err = wr.Write(bytes)
		}
	}

	if err != nil {
		logger.Error("IO Error occurs at response -", err.Error())
	}
	return true
}

func (s *HttpServer) StartServer() {
	s.startWith(true)
}

func (s *HttpServer) StartServerAsync() *http.Server {
	return s.startWith(false)
}

func (s *HttpServer) AddInterceptor(interceptor Interceptor) {
	s.handlers.interceptorChain.AddInterceptor(interceptor)
}
func (s *HttpServer) startWith(block bool) *http.Server {

	//default :8080
	if s.HttpConfig.Address == "" {
		s.HttpConfig.Address = ":8080"
	}

	server := &http.Server{
		Handler:           s.handlers,
		Addr:              s.HttpConfig.Address,
		WriteTimeout:      s.HttpConfig.WriteTimeout,
		ReadTimeout:       s.HttpConfig.ReadTimeout,
		ReadHeaderTimeout: s.HttpConfig.ReadHeaderTimeout,
		IdleTimeout:       s.HttpConfig.IdleTimeout,
	}

	go func() {
		var err error
		logger.Info("Kinoko web server started at", s.HttpConfig.Address)
		if s.SSLConfig.EnableSSL {
			logger.Info("SSL is enabled.")
			err = server.ListenAndServeTLS(s.SSLConfig.CertFile, s.SSLConfig.KeyFile)
		} else {
			err = server.ListenAndServe()
		}

		logger.Warn(err)

	}()

	if !block {
		return server
	}

	sig := make(chan os.Signal)
	signal.Notify(sig, syscall.SIGINT)
	<-sig

	c, _ := context.WithCancel(context.Background())
	_ = server.Shutdown(c)
	return nil
}

const errorPage = "<h1>%v %v</h1><h2>%v</h2><p>%v</p>"

func HttpError(wr http.ResponseWriter, status int, info string, showtrace bool) {
	wr.Header().Set("Content-Type", "text/html; charset=utf-8")
	wr.Header().Set("X-Content-Type-Options", "nosniff")
	wr.WriteHeader(status)

	callers := ""
	if showtrace {
		for i := 3; true; i++ {
			_, file, line, ok := runtime.Caller(i)
			if !ok {
				break
			}
			callers = callers + fmt.Sprintf("<div>%v:%v</div>\n", file, line)
		}
	}
	html := fmt.Sprintf(errorPage, status, http.StatusText(status), template.HTMLEscapeString(info), callers)
	_, _ = fmt.Fprintln(wr, html)
}

func NewProperty(k string, v interface{}) HandlerProperties {
	return HandlerProperties{k, v}
}
