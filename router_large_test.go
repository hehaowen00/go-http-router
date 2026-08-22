package gohttprouter

import "testing"

var largeAPI = [][]string{}

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

func BenchmarkRouterLarge(b *testing.B) {
	if len(largeAPI) == 0 {
		b.Skip()
		return
	}

	r := New[int]()
	for i, route := range largeAPI {
		r.Add(route[0], route[1], i)
	}
	params := Params{}
	b.ResetTimer()

	for i := 0; b.Loop(); i++ {
		route := largeAPI[i%len(largeAPI)]
		handler := r.Search(route[0], route[1], &params)
		if handler == nil {
			b.Fatalf("route not found: %s %s", route[0], route[1])
		}
	}
}

func BenchmarkRouterLargeAll(b *testing.B) {
	if len(largeAPI) == 0 {
		b.Skip()
		return
	}

	r := New[int]()
	for i, route := range largeAPI {
		r.Add(route[0], route[1], i)
	}

	params := Params{}
	b.ResetTimer()

	for i := 0; b.Loop(); i++ {
		for _, route := range largeAPI {
			handler := r.Search(route[0], route[1], &params)
			if handler == nil {
				b.Fatalf("route not found: %s %s", route[0], route[1])
			}
		}
	}
}
