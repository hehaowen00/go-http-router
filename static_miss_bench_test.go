package gohttprouter

import (
	"fmt"
	"math/rand"
	"net/http"
	"strings"
	"testing"
)

// concretePath turns a route pattern into a representative request path by
// replacing each param segment with a short value and each catch-all with a
// two-segment remainder. These are the paths that actually hit the tree in
// production, unlike searching the raw ":param" pattern.
func concretePath(pattern string) string {
	segments := strings.Split(pattern, "/")
	var b strings.Builder
	b.Grow(len(pattern) + 8)

	for i, seg := range segments {
		if i > 0 {
			b.WriteByte('/')
		}

		switch {
		case strings.HasPrefix(seg, ":"):
			b.WriteString("x")
		case strings.HasPrefix(seg, "*"):
			b.WriteString("a/b")
		default:
			b.WriteString(seg)
		}
	}

	return b.String()
}

func paramRequests(routes [][]string) []struct{ method, path string } {
	var reqs []struct{ method, path string }

	for _, route := range routes {
		if strings.ContainsAny(route[1], ":*") {
			reqs = append(reqs, struct{ method, path string }{
				method: route[0],
				path:   concretePath(route[1]),
			})
		}
	}

	return reqs
}

func BenchmarkRouterGithubParams(b *testing.B) {
	r := New[int]()
	for i, route := range githubAPI {
		if err := r.Add(route[0], route[1], i); err != nil {
			b.Fatal(err)
		}
	}

	reqs := paramRequests(githubAPI)
	params := Params{}

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; b.Loop(); i++ {
		req := reqs[i%len(reqs)]
		if r.Search(req.method, req.path, &params) == nil {
			b.Fatalf("route not found: %s %s", req.method, req.path)
		}
	}
}

// BenchmarkParamMissSingle isolates the static-map miss paid by a param
// search. The static route is long enough that its staticMaxLen covers the
// concrete param path, so the current guard does a map lookup before falling
// through to the tree.
func BenchmarkParamMissSingle(b *testing.B) {
	r := New[int]()
	if err := r.Add("GET", "/users/:id", 1); err != nil {
		b.Fatal(err)
	}
	if err := r.Add("GET", "/aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", 2); err != nil {
		b.Fatal(err)
	}

	params := Params{}

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; b.Loop(); i++ {
		if r.Search("GET", "/users/123", &params) == nil {
			b.Fatal("route not found")
		}
	}
}

// 2000 static routes of one length, requested in shuffled order. The GitHub
// set never has more than 4 static routes of a length, so this is the only
// benchmark that exercises a large bucket.
func BenchmarkRouterManyStatic(b *testing.B) {
	r := New[int]()

	paths := make([]string, 2000)
	for i := range paths {
		paths[i] = fmt.Sprintf("/pages/p-%04d", i)
		if err := r.Add(http.MethodGet, paths[i], i); err != nil {
			b.Fatal(err)
		}
	}

	rand.New(rand.NewSource(1)).Shuffle(len(paths), func(i, j int) {
		paths[i], paths[j] = paths[j], paths[i]
	})

	params := Params{}

	for i := 0; b.Loop(); i++ {
		if r.Search(http.MethodGet, paths[i%len(paths)], &params) == nil {
			b.Fatal("miss")
		}
	}
}
