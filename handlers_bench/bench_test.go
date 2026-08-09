package handlersbench

import (
	"net/http"
	"testing"
)

type Handler interface {
	Handle()
}

type handler struct{}

func (h handler) Handle() {
}

type fieldStruct struct {
	get     Handler
	query   Handler
	post    Handler
	patch   Handler
	put     Handler
	delete  Handler
	connect Handler
	options Handler
	head    Handler
}

func BenchmarkField(b *testing.B) {
	fs := &fieldStruct{
		get:     handler{},
		query:   handler{},
		post:    handler{},
		patch:   handler{},
		put:     handler{},
		delete:  handler{},
		connect: handler{},
		options: handler{},
		head:    handler{},
	}

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		h := fs.get
		if h == nil {
			b.Fatal("handler is nil")
		}
	}
}

type mapStruct struct {
	handlers map[string]Handler
}

func BenchmarkMap(b *testing.B) {
	ms := &mapStruct{
		handlers: map[string]Handler{
			http.MethodGet:     handler{},
			"QUERY":            handler{},
			http.MethodPost:    handler{},
			http.MethodPatch:   handler{},
			http.MethodPut:     handler{},
			http.MethodDelete:  handler{},
			http.MethodConnect: handler{},
			http.MethodOptions: handler{},
			http.MethodHead:    handler{},
		},
	}

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		h := ms.handlers[http.MethodGet]
		if h == nil {
			b.Fatal("handler is nil")
		}
	}
}

type arrayStruct struct {
	handlers [9]Handler
}

func methodToIndex(method string) int {
	switch method {
	case http.MethodGet:
		return 0
	case "QUERY":
		return 1
	case http.MethodPost:
		return 2
	case http.MethodPatch:
		return 3
	case http.MethodPut:
		return 4
	case http.MethodDelete:
		return 5
	case http.MethodConnect:
		return 6
	case http.MethodOptions:
		return 7
	case http.MethodHead:
		return 8
	default:
		return -1
	}
}

func BenchmarkArray(b *testing.B) {
	as := &arrayStruct{
		handlers: [9]Handler{
			handler{},
			handler{},
			handler{},
			handler{},
			handler{},
			handler{},
			handler{},
			handler{},
			handler{},
		},
	}

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		h := as.handlers[methodToIndex(http.MethodGet)]
		if h == nil {
			b.Fatal("handler is nil")
		}
	}
}
