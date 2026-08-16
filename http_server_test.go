package gohttprouter

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type paramsCtxKey struct{}

func paramsFrom(r *http.Request) *Params {
	return r.Context().Value(paramsCtxKey{}).(*Params)
}

func serve(r *Router[http.HandlerFunc]) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		var params Params
		h := r.Search(req.Method, req.URL.Path, &params)
		if h == nil {
			http.NotFound(w, req)
			return
		}

		ctx := context.WithValue(req.Context(), paramsCtxKey{}, &params)
		(*h)(w, req.WithContext(ctx))
	})
}

func TestHTTPServerParams(t *testing.T) {
	r := New[http.HandlerFunc]()

	r.Add(http.MethodGet, "/about", func(w http.ResponseWriter, req *http.Request) {
		io.WriteString(w, "static=ok")
	})
	r.Add(
		http.MethodGet,
		"/users/:id",
		func(w http.ResponseWriter, req *http.Request) {
			p := paramsFrom(req)
			io.WriteString(w, "id="+p.Get("id")+"&q="+req.URL.Query().Get("q"))
		},
	)
	r.Add(
		http.MethodGet,
		"/users/:id/posts/:postID",
		func(w http.ResponseWriter, req *http.Request) {
			p := paramsFrom(req)
			io.WriteString(w, "id="+p.Get("id")+"&post="+p.Get("postID"))
		},
	)
	r.Add(
		http.MethodGet,
		"/files/*path",
		func(w http.ResponseWriter, req *http.Request) {
			p := paramsFrom(req)
			io.WriteString(w, "path="+p.Get("path"))
		},
	)

	srv := httptest.NewServer(serve(r))
	defer srv.Close()

	cases := []struct {
		path string
		want string
	}{
		{"/about", "static=ok"},
		{"/users/42", "id=42&q="},
		{"/users/42?q=hi", "id=42&q=hi"},
		{"/users/alice/posts/7", "id=alice&post=7"},
		{"/files/a/b/c", "path=a/b/c"},
		{"/missing", "404"},
	}

	for _, c := range cases {
		resp, err := http.Get(srv.URL + c.path)
		if err != nil {
			t.Fatalf("GET %s: %v", c.path, err)
		}
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		if c.want == "404" {
			if resp.StatusCode != http.StatusNotFound {
				t.Errorf("GET %s = status %d, want 404", c.path, resp.StatusCode)
			}
			continue
		}

		if resp.StatusCode != http.StatusOK {
			t.Errorf("GET %s = status %d, want 200", c.path, resp.StatusCode)
		}
		if got := strings.TrimSpace(string(body)); got != c.want {
			t.Errorf("GET %s = %q, want %q", c.path, got, c.want)
		}
	}
}
