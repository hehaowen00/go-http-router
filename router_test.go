package gohttprouter

import (
	"log"
	"math/rand/v2"
	"net/http"
	"slices"
	"testing"

	"github.com/hehaowen00/go-inspect"
)

type dummyHandler struct{}

func (dummyHandler) Handle(c Context) {
	w := c.Writer()
	w.WriteHeader(http.StatusOK)

	n, err := w.Write([]byte("ok"))
	log.Println(n, err)
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

func TestRootRoute(t *testing.T) {
	r := New[dummyHandler]()
	r.Add(http.MethodGet, "/", dummyHandler{})

	params := Params{}

	for _, path := range []string{"/", "", "//"} {
		if h := r.Search(http.MethodGet, path, &params); h == nil {
			t.Fatalf("Search(%q) = nil, want root handler", path)
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
		"/api/hello/:a/:b",
		"/api/hello/:message/n",
		"/api/help",
	}

	r := New[dummyHandler]()

	for _, route := range routes {
		r.Add(http.MethodGet, route, dummyHandler{})
	}

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
	r := New[int]()
	params := Params{}

	for i, route := range githubAPI {
		if err := r.Add(route[0], route[1], i); err != nil {
			t.Fatalf("add %s %s: %v", route[0], route[1], err)
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

	for i, route := range githubAPI {
		r.Remove(route[0], route[1])
		if h := r.Search(route[0], route[1], &params); h != nil && *h == i {
			t.Fatalf("failed to remove %s %s", route[0], route[1])
		}
	}
}

func BenchmarkBuildGithubAPI(b *testing.B) {
	b.ReportAllocs()

	for i := 0; b.Loop(); i++ {
		r := New[dummyHandler]()

		for _, route := range githubAPI {
			r.Add(route[0], route[1], dummyHandler{})
		}
	}
}

func BenchmarkBuildGithubAPIInsertOnly(b *testing.B) {
	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; b.Loop(); i++ {
		r := New[dummyHandler]()

		for _, route := range githubAPI {
			r.Add(route[0], route[1], dummyHandler{})
		}
	}
}

func BenchmarkRouterGithub(b *testing.B) {
	r := New[dummyHandler]()
	for _, route := range githubAPI {
		r.Add(route[0], route[1], dummyHandler{})
	}

	params := Params{}

	for i := 0; b.Loop(); i++ {
		route := githubAPI[i%len(githubAPI)]
		handler := r.Search(route[0], route[1], &params)
		if handler == nil {
			b.Fatalf("route not found: %s %s", route[0], route[1])
		}
	}
}

func BenchmarkRouterGithubAll(b *testing.B) {
	r := New[dummyHandler]()
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

func BenchmarkRouterGithubRandom(b *testing.B) {
	r := New[dummyHandler]()
	for _, route := range githubAPI {
		r.Add(route[0], route[1], dummyHandler{})
	}

	xs := rand.Perm(len(githubAPI))
	b.ResetTimer()

	params := Params{}

	for i := 0; b.Loop(); i++ {
		for _, j := range xs {
			route := githubAPI[j]

			handler := r.Search(route[0], route[1], &params)
			if handler == nil {
				b.Fatalf("route not found: %s %s", route[0], route[1])
			}
		}
	}
}
