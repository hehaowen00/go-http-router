package gohttprouter

import (
	"net/http"
	"slices"
	"testing"
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
