package gohttprouter

import (
	"slices"
	"strconv"
	"strings"
	"testing"
)

func TestStaticLenHashFalseNegatives(t *testing.T) {
	var s staticLenFilter

	registered := []int{0, 1, 63, 64, 255, 256, 257, 300, 1000, 1 << 16}
	for _, n := range registered {
		s.set(n)
	}

	for _, n := range registered {
		if !s.has(n) {
			t.Errorf("has(%d) = false, want true (false negative!)", n)
		}
	}

	var huge staticLenFilter
	for _, n := range []int{1 << 20, 1 << 30, 1 << 40} {
		huge.set(n)
		if !huge.has(n) {
			t.Errorf("has(%d) = false, want true", n)
		}
	}
}

func TestStaticLenHashFalsePositiveRate(t *testing.T) {
	registered := []int{5, 6, 7, 9, 10, 11, 12, 13, 14, 15, 19, 20}

	var s staticLenFilter

	for _, n := range registered {
		s.set(n)
	}

	const maxFPRate = 0.10
	probes := 0
	total := 0

	for n := range 1 << 14 {
		if slices.Contains(registered, n) {
			continue
		}

		total++
		if s.has(n) {
			probes++
		}
	}

	rate := float64(probes) / float64(total)
	t.Log("fp rate", rate)

	if rate > maxFPRate {
		t.Errorf("false positive rate %.4f > %.2f", rate, maxFPRate)
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
		if !r.staticLen[methodGet].has(n) {
			t.Errorf("staticLen.has(%d) = false, want true", n)
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
		if !r.staticLen[methodGet].has(len(p)) {
			t.Errorf("rebuilt set lost surviving length %d", len(p))
		}
	}
}
