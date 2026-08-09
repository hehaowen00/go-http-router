package gohttprouter

import (
	"log"
	"net/http"
	"slices"
	"testing"
)

type dummyHandler struct{}

func (dummyHandler) Handle(c Context) {
	n, err := c.Write(http.StatusOK, []byte("ok"))
	log.Println(n, err)
}

func TestHandlersCount(t *testing.T) {
	var h methodHandler

	if got := h.count(); got != 0 {
		t.Fatalf("empty handlers: count = %d, want 0", got)
	}

	h.insertHandler(http.MethodGet, dummyHandler{})

	if got := h.count(); got != 1 {
		t.Fatalf("after GET: count = %d, want 1", got)
	}

	for _, m := range []string{
		http.MethodPost,
		http.MethodPut,
		http.MethodDelete,
		http.MethodConnect,
		http.MethodOptions,
	} {
		h.insertHandler(m, dummyHandler{})
	}

	if got := h.count(); got != 6 {
		t.Fatalf("all six methods: count = %d, want 6", got)
	}

	h.insertHandler(http.MethodGet, dummyHandler{})
	if got := h.count(); got != 6 {
		t.Fatalf("after re-setting GET: count = %d, want 6", got)
	}

	handler := h.getHandler(http.MethodPatch)
	if handler != nil {
		t.Fatalf("expected nil for PATCH, got %v", handler)
	}
}

func TestPathSplit(t *testing.T) {
	paths := []string{
		"/public//",
		"/api//",
		"/api/hello//",
		"/api/hello/world",
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

	params := Params{}

	for _, route := range routes {
		h := r.Search(http.MethodGet, route, &params)
		if h == nil {
			t.Log(route)
			t.FailNow()
		}
	}

	// r.Search(http.MethodGet, "/api/../hello", &params)
}

// func TestRouterGithub(t *testing.T) {
// 	r := New()
// 	for _, route := range githubAPI {
// 		r.Insert(route[0], route[1], dummyHandler{})
// 		params := map[string]string{}
// 		h := r.Search(route[0], route[1], params)
// 		if h == nil {
// 			t.Log("failed to find", route[0], route[1])
// 			t.FailNow()
// 		}
// 	}

// 	for _, route := range githubAPI {
// 		params := map[string]string{}
// 		h := r.Search(route[0], route[1], params)
// 		if h == nil {
// 			t.Log("failed to find", route[0], route[1])
// 			t.FailNow()
// 		}
// 	}
// }

// func BenchmarkRouter(b *testing.B) {
// 	r := New()
// 	for _, route := range githubAPI {
// 		r.Insert(route[0], route[1], dummyHandler{})
// 	}

// 	params := map[string]string{}

// 	for i := 0; b.Loop(); i++ {
// 		route := githubAPI[i%len(githubAPI)]
// 		handler := r.Search(route[0], route[1], params)
// 		if handler == nil {
// 			b.Fatalf("route not found: %s %s", route[0], route[1])
// 		}
// 	}
// }
