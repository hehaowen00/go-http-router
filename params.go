package gohttprouter

import (
	"net/http"
)

const maxParams = 32

// cannot have more than 32 params
// paths are validated on router build to not have more than 32 params
type Params struct {
	entries [maxParams]param
	idx     paramsIndex
	path    string
}

type paramsIndex int

type param struct {
	key        string
	valueStart int32
	valueEnd   int32
}

func (p *Params) Use(req *http.Request) {
	p.path = req.URL.Path
}

func (p *Params) Get(key string) string {
	for idx := range p.idx {
		e := p.entries[idx]

		if key[0] == e.key[0] && key == e.key {
			if len(p.path) < int(e.valueEnd) {
				return ""
			}

			return p.path[e.valueStart:e.valueEnd]
		}
	}

	return ""
}

func (p *Params) set(key string, valueStart, valueEnd int32) {
	p.entries[p.idx] = param{key, valueStart, valueEnd}
	p.idx++
}

func (p *Params) save() paramsIndex {
	return p.idx
}

func (p *Params) restore(idx paramsIndex) {
	p.idx = idx
}

func (p *Params) reset() {
	p.idx = 0
}
