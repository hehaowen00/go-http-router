package gohttprouter

import (
	"net/http"
)

type Context struct {
	req    *http.Request
	params *Params
	w      http.ResponseWriter
}

func (c *Context) Request() *http.Request {
	return c.req
}

func (c *Context) Params() *Params {
	return c.params
}

func (c *Context) Writer() http.ResponseWriter {
	return c.w
}
