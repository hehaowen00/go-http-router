package gohttprouter

import (
	"strconv"
	"strings"
	"testing"
)

func TestStaticBuckets(t *testing.T) {
	var st staticTable

	registered := []int{1, 2, 63, 64, 254, 255, 256, 257, 300, 1000, 1 << 16}
	for i, n := range registered {
		st.set("/"+strings.Repeat("a", n-1), handlerPtr(i))
	}

	for i, n := range registered {
		key := "/" + strings.Repeat("a", n-1)

		lo, hi := st.bucket(n)
		if lo >= hi {
			t.Fatalf("bucket(%d) is empty (false negative)", n)
		}

		if idx, ok := st.scan(key, lo, hi); !ok || idx != handlerPtr(i) {
			t.Errorf("scan(len %d) = %d, %v, want %d", n, idx, ok, i)
		}
	}

	for _, n := range []int{3, 62, 65, 253, 258, 999, 1 << 17} {
		lo, hi := st.bucket(n)
		key := "/" + strings.Repeat("a", n-1)

		if _, ok := st.scan(key, lo, hi); ok {
			t.Errorf("scan(len %d) found a key that was never set", n)
		}

		if n < staticLenBits && lo < hi {
			t.Errorf("bucket(%d) = [%d, %d), want empty", n, lo, hi)
		}
	}
}

func TestStaticLenMixedLengths(t *testing.T) {
	r := New[int]()

	short := "/"
	if err := r.Add("GET", short, 1); err != nil {
		t.Fatal(err)
	}

	mid := "/" + strings.Repeat("m", 254)
	long := "/" + strings.Repeat("x", 300)

	if err := r.Add("GET", mid, 2); err != nil {
		t.Fatal(err)
	}

	if err := r.Add("GET", long, 3); err != nil {
		t.Fatal(err)
	}

	check := func(path string, want int) {
		t.Helper()
		if h := r.Search("GET", path, &Params{}); h == nil || *h != want {
			t.Fatalf("Search(%q) = %v, want %d", path, h, want)
		}
	}
	check(short, 1)
	check(mid, 2)
	check(long, 3)

	paths := []string{
		"/nope", "/" + strings.Repeat("y", 255),
		"/" + strings.Repeat("z", 400),
	}

	for _, path := range paths {
		if h := r.Search("GET", path, &Params{}); h != nil {
			t.Fatalf("Search(%q...) = %v, want nil", path[:min(len(path), 8)], h)
		}
	}

	for _, n := range []int{1, 255, 301} {
		if lo, hi := r.static[methodGet].bucket(n); lo >= hi {
			t.Errorf("bucket(%d) is empty, want a route", n)
		}
	}
}

func TestStaticLenRemoveRebuilds(t *testing.T) {
	r := New[int]()

	paths := make([]string, 3)
	for i, pad := range []int{260, 300, 340} {
		paths[i] = "/" + strings.Repeat("r", pad) + "/seg" + strconv.Itoa(i)
		if err := r.Add("GET", paths[i], i); err != nil {
			t.Fatal(err)
		}
	}

	r.Remove("GET", paths[1])

	if h := r.Search("GET", paths[0], &Params{}); h == nil || *h != 0 {
		t.Fatalf("survivor 0: got %v", h)
	}

	if h := r.Search("GET", paths[2], &Params{}); h == nil || *h != 2 {
		t.Fatalf("survivor 2: got %v", h)
	}

	if h := r.Search("GET", paths[1], &Params{}); h != nil {
		t.Fatalf("removed: got %v, want nil", h)
	}

	xs := []string{paths[0], paths[2]}

	for _, p := range xs {
		if lo, hi := r.static[methodGet].bucket(len(p)); lo >= hi {
			t.Errorf("rebuilt buckets lost surviving length %d", len(p))
		}
	}
}
