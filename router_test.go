package gohttprouter

import (
	"math/rand/v2"
	"net/http"
	"slices"
	"testing"

	"github.com/hehaowen00/go-inspect"
)

func TestURL(t *testing.T) {
	req, _ := http.NewRequest(http.MethodGet, "https://google.com", nil)
	q := req.URL.Query()
	q.Add("a", "b")
	q.Add("b", "a")
	req.URL.RawQuery = q.Encode()

	t.Log(req.URL)
	t.Log(req.URL.Host)
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
		"users/posts",
		"/:id",
		"/files/*path",
		"//users//posts//",
	}

	expected := [][]string{
		{"/public/"},
		{"/api/"},
		{"/api/hello/"},
		{"/api/hello/world/"},
		{"/api/hello/", ":message", "/n/"},
		{"/api/hello/n/"},
		{"/api/hello/", ":a", ":b"},
		{"/users/posts/"},
		{":id"},
		{"/files/", "*path"},
		{"/users/posts/"},
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
	r := New[int]()
	r.Add(http.MethodGet, "/", 1)

	params := Params{}

	for _, path := range []string{"/", "", "//"} {
		if h := r.Search(http.MethodGet, path, &params); h == nil || *h != 1 {
			t.Fatalf("Search(%q) = %v, want root handler", path, h)
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

	r := New[int]()

	for i, route := range routes {
		if err := r.Add(http.MethodGet, route, i); err != nil {
			t.Fatalf("add %s: %v", route, err)
		}
	}

	params := Params{}

	for i, route := range routes {
		h := r.Search(http.MethodGet, route, &params)
		if h == nil || *h != i {
			t.Fatalf("Search(%s) = %v, want %d", route, h, i)
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

	t.Log("memory", r.MemSize())

	for i, route := range githubAPI {
		r.Remove(route[0], route[1])
		if h := r.Search(route[0], route[1], &params); h != nil && *h == i {
			t.Fatalf("failed to remove %s %s", route[0], route[1])
		}
	}
}

func TestRouterLarge(t *testing.T) {
	if len(largeAPI) == 0 {
		t.Skip()
		return
	}

	r := New[int]()
	params := Params{}

	for i, route := range largeAPI {
		if err := r.Add(route[0], route[1], i); err != nil {
			t.Fatalf("add %s %s: %v", route[0], route[1], err)
		}
	}

	for _, route := range largeAPI {
		h := r.Search(route[0], route[1], &params)
		if h == nil {
			inspect.Inspect(r)
			t.Log("failed to find", route[0], route[1])
			t.FailNow()
		}
	}

	t.Log("memory", r.MemSize())

	for i, route := range largeAPI {
		r.Remove(route[0], route[1])
		if h := r.Search(route[0], route[1], &params); h != nil && *h == i {
			t.Fatalf("failed to remove %s %s", route[0], route[1])
		}
	}
}

func BenchmarkBuildGithubAPI(b *testing.B) {
	b.ReportAllocs()

	for i := 0; b.Loop(); i++ {
		r := New[int]()

		for _, route := range githubAPI {
			r.Add(route[0], route[1], i)
		}
	}
}

func BenchmarkBuildGithubAPIInsertOnly(b *testing.B) {
	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; b.Loop(); i++ {
		r := New[int]()

		for i, route := range githubAPI {
			r.Add(route[0], route[1], i)
		}
	}
}

func BenchmarkRouterGithub(b *testing.B) {
	r := New[int]()
	for i, route := range githubAPI {
		r.Add(route[0], route[1], i)
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

func BenchmarkRouterLarge(b *testing.B) {
	if len(largeAPI) == 0 {
		b.Skip()
		return
	}

	b.ResetTimer()

	r := New[int]()
	for i, route := range largeAPI {
		r.Add(route[0], route[1], i)
	}
	params := Params{}

	for i := 0; b.Loop(); i++ {
		route := largeAPI[i%len(largeAPI)]
		handler := r.Search(route[0], route[1], &params)
		if handler == nil {
			b.Fatalf("route not found: %s %s", route[0], route[1])
		}
	}
}

func BenchmarkRouterGithubParallel(b *testing.B) {
	r := New[int]()
	for i, route := range githubAPI {
		if err := r.Add(route[0], route[1], i); err != nil {
			b.Fatal(err)
		}
	}

	reqs := make([]struct{ method, path string }, len(githubAPI))
	for i, route := range githubAPI {
		reqs[i] = struct{ method, path string }{
			method: route[0],
			path:   concretePath(route[1]),
		}
	}

	for _, req := range reqs {
		params := Params{}
		if r.Search(req.method, req.path, &params) == nil {
			b.Fatalf("route not found: %s %s", req.method, req.path)
		}
	}

	b.ReportAllocs()
	b.ResetTimer()

	b.RunParallel(func(pb *testing.PB) {
		params := Params{}

		for pb.Next() {
			for _, req := range reqs {
				if r.Search(req.method, req.path, &params) == nil {
					b.Errorf("route not found: %s %s", req.method, req.path)
				}
			}
		}
	})
}

func BenchmarkRouterGithubAll(b *testing.B) {
	r := New[int]()
	for i, route := range githubAPI {
		r.Add(route[0], route[1], i)
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

func BenchmarkRouterLargeAll(b *testing.B) {
	if len(largeAPI) == 0 {
		b.Skip()
		return
	}

	b.ResetTimer()

	r := New[int]()
	for i, route := range largeAPI {
		r.Add(route[0], route[1], i)
	}
	params := Params{}

	for i := 0; b.Loop(); i++ {
		for _, route := range largeAPI {
			handler := r.Search(route[0], route[1], &params)
			if handler == nil {
				b.Fatalf("route not found: %s %s", route[0], route[1])
			}
		}
	}
}

func BenchmarkRouterGithubRandom(b *testing.B) {
	r := New[int]()
	for i, route := range githubAPI {
		r.Add(route[0], route[1], i)
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
