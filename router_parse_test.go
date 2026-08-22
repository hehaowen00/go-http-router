package gohttprouter

import "testing"

var parseAPI = [][]string{
	// Objects
	{"POST", "/1/classes/:className"},
	{"GET", "/1/classes/:className/:objectId"},
	{"PUT", "/1/classes/:className/:objectId"},
	{"GET", "/1/classes/:className"},
	{"DELETE", "/1/classes/:className/:objectId"},

	// Users
	{"POST", "/1/users"},
	{"GET", "/1/login"},
	{"GET", "/1/users/:objectId"},
	{"PUT", "/1/users/:objectId"},
	{"GET", "/1/users"},
	{"DELETE", "/1/users/:objectId"},
	{"POST", "/1/requestPasswordReset"},

	// Roles
	{"POST", "/1/roles"},
	{"GET", "/1/roles/:objectId"},
	{"PUT", "/1/roles/:objectId"},
	{"GET", "/1/roles"},
	{"DELETE", "/1/roles/:objectId"},

	// Files
	{"POST", "/1/files/:fileName"},

	// Analytics
	{"POST", "/1/events/:eventName"},

	// Push Notifications
	{"POST", "/1/push"},

	// Installations
	{"POST", "/1/installations"},
	{"GET", "/1/installations/:objectId"},
	{"PUT", "/1/installations/:objectId"},
	{"GET", "/1/installations"},
	{"DELETE", "/1/installations/:objectId"},

	// Cloud Functions
	{"POST", "/1/functions"},
}

func TestRouterParse(t *testing.T) {
	r := New[int]()
	params := Params{}

	for i, route := range parseAPI {
		if err := r.Add(route[0], route[1], i); err != nil {
			t.Fatalf("add %s %s: %v", route[0], route[1], err)
		}
	}

	for _, route := range parseAPI {
		h := r.Search(route[0], route[1], &params)
		if h == nil {
			t.Log("failed to find", route[0], route[1])
			t.FailNow()
		}
	}

	t.Log("memory", r.MemSize())

	for i, route := range parseAPI {
		r.Remove(route[0], route[1])
		if h := r.Search(route[0], route[1], &params); h != nil && *h == i {
			t.Fatalf("failed to remove %s %s", route[0], route[1])
		}
	}
}

func BenchmarkBuildParseAPI(b *testing.B) {
	b.ReportAllocs()

	for i := 0; b.Loop(); i++ {
		r := New[int]()

		for _, route := range parseAPI {
			r.Add(route[0], route[1], i)
		}
	}
}

func BenchmarkRouterParse(b *testing.B) {
	r := New[int]()

	for i, route := range parseAPI {
		r.Add(route[0], route[1], i)
	}

	params := Params{}
	b.ResetTimer()

	for i := 0; b.Loop(); i++ {
		route := parseAPI[i%len(parseAPI)]
		handler := r.Search(route[0], route[1], &params)
		if handler == nil {
			b.Fatalf("route not found: %s %s", route[0], route[1])
		}
	}
}
