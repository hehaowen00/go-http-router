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

// Insert refreshes search targets one wildcard at a time. After every Add
// they must match what a full refreshSearchTargets pass would compute.
func TestIncrementalSearchTargets(t *testing.T) {
	type target struct {
		searchNode nodePtr
		skip       uint8
	}

	snapshot := func(nodes []node) []target {
		var res []target
		for i := range nodes {
			for j := range nodes[i].numWildcards() {
				wc := nodes[i].cold.wildcard[j]
				res = append(res, target{wc.searchNode, wc.skip})
			}
		}
		return res
	}

	for name, routes := range map[string][][]string{
		"github": githubAPI,
		"parse":  parseAPI,
		"large":  largeAPI,
	} {
		r := New[int]()

		for i, route := range routes {
			if err := r.Add(route[0], route[1], i); err != nil {
				t.Fatal(err)
			}

			got := snapshot(r.nodes)
			refreshSearchTargets(r.nodes)

			if want := snapshot(r.nodes); !slices.Equal(got, want) {
				t.Fatalf("%s: stale search targets after adding %s %s",
					name, route[0], route[1])
			}
		}
	}
}

// nodeCold keeps its first wildcard inline and spills to the heap from the
// second. Removing and re-adding across that boundary must keep every
// remaining route intact.
func TestWildcardInlineSpill(t *testing.T) {
	r := New[int]()

	routes := []string{"/a/:x/b", "/a/:y/c", "/a/:z/d"}
	for i, route := range routes {
		if err := r.Add("GET", route, i); err != nil {
			t.Fatal(err)
		}
	}

	check := func(path string, want int, key, value string) {
		t.Helper()

		var p Params
		p.path = path

		h := r.Search("GET", path, &p)
		if want < 0 {
			if h != nil {
				t.Fatalf("Search(%q) = %d, want nil", path, *h)
			}
			return
		}

		if h == nil || *h != want {
			t.Fatalf("Search(%q) = %v, want %d", path, h, want)
		}

		if got := p.Get(key); got != value {
			t.Fatalf("Search(%q): %s = %q, want %q", path, key, got, value)
		}
	}

	check("/a/1/b", 0, "x", "1")
	check("/a/2/c", 1, "y", "2")
	check("/a/3/d", 2, "z", "3")

	r.Remove("GET", "/a/:y/c")
	check("/a/1/b", 0, "x", "1")
	check("/a/2/c", -1, "", "")
	check("/a/3/d", 2, "z", "3")

	r.Remove("GET", "/a/:x/b")
	check("/a/1/b", -1, "", "")
	check("/a/3/d", 2, "z", "3")

	if err := r.Add("GET", "/a/:w/e", 3); err != nil {
		t.Fatal(err)
	}
	check("/a/3/d", 2, "z", "3")
	check("/a/4/e", 3, "w", "4")
}
