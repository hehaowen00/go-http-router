package gohttprouter

import (
	"log"
	"net/http"
	"slices"
	"testing"

	"github.com/hehaowen00/go-inspect"
)

type dummyHandler struct{}

func (dummyHandler) Handle(c Context) {
	n, err := c.Write(http.StatusOK, []byte("ok"))
	log.Println(n, err)
}

func TestHandlersCount(t *testing.T) {
	var h methodHandler

	if got := h.count; got != 0 {
		t.Fatalf("empty handlers: count = %d, want 0", got)
	}

	h.Insert(http.MethodGet, dummyHandler{})

	if got := h.count; got != 1 {
		t.Fatalf("after GET: count = %d, want 1", got)
	}

	for _, m := range []string{
		http.MethodPost,
		http.MethodPut,
		http.MethodDelete,
		http.MethodConnect,
		http.MethodOptions,
	} {
		h.Insert(m, dummyHandler{})
	}

	if got := h.count; got != 6 {
		t.Fatalf("all six methods: count = %d, want 6", got)
	}

	h.Insert(http.MethodGet, dummyHandler{})
	if got := h.count; got != 6 {
		t.Fatalf("after re-setting GET: count = %d, want 6", got)
	}

	handler := h.Get(http.MethodPatch)
	if handler != nil {
		t.Fatalf("expected nil for PATCH, got %v", handler)
	}
}

func TestPathSplit(t *testing.T) {
	paths := []string{
		"/public//",
		"/api//",
		"/api/hello//",
		"/api/hello/world/",
		"/api/hello/:message/n//",
		"/api/hello//n",
		"/api/hello/:a/:b",
	}

	expected := [][]string{
		{"/public/"},
		{"/api/"},
		{"/api/hello/"},
		{"/api/hello/world/"},
		{"/api/hello/", ":message", "/n/"},
		{"/api/hello/n/"},
		{"/api/hello/", ":a", ":b"},
	}

	for i, p := range paths {
		xs := splitPath(p)
		if !slices.Equal(xs, expected[i]) {
			t.Log("test fail", xs, expected[i])
			t.FailNow()
		}
	}
}

func TestRouter(t *testing.T) {
	routes := []string{
		"/public/",
		"/api/",
		"/ap/",
		"/api/hello",
		"/api/hello/world",
		"/api/goodbye",
		"/api/hello/me",
		"/api/hello/:message",
		// "/api/hello/:a/:b",
		"/api/hello/:message/n",
		"/api/help",
	}

	r := New()

	for _, route := range routes {
		r.GET(route, dummyHandler{})
	}

	inspect.Inspect(r)

	params := Params{}

	for _, route := range routes {
		h := r.Search(http.MethodGet, route, &params)
		if h == nil {
			t.Log(route)
			t.FailNow()
		}
	}
}

func TestRouterGithub(t *testing.T) {
	r := New()
	params := Params{}

	for _, route := range githubAPI {
		r.Add(route[0], route[1], dummyHandler{})
		h := r.Search(route[0], route[1], &params)
		if h == nil {
			inspect.Inspect(r)
			t.Log("failed to find", route[0], route[1])
			t.FailNow()
		}
	}

	for _, route := range githubAPI {
		h := r.Search(route[0], route[1], &params)
		if h == nil {
			inspect.Inspect(r)
			t.Log("failed to find", route[0], route[1])
			t.FailNow()
		}
	}
}

func BenchmarkRouter(b *testing.B) {
	r := New()
	for _, route := range githubAPI {
		r.Add(route[0], route[1], dummyHandler{})
	}

	params := Params{}

	for i := 0; b.Loop(); i++ {
		for _, route := range githubAPI {
			handler := r.Search(route[0], route[1], &params)
			if handler == nil {
				b.Fatalf("route not found: %s %s", route[0], route[1])
			}
		}
	}
}
