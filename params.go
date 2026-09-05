package gohttprouter

import (
	"net/http"
)

const maxParams = 32

// cannot have more than 32 params
// paths are validated on router build to not have more than 32 params
// Field order is load-bearing: idx sits immediately before entries so that
// recording a param touches one cache line instead of two. With entries first
// idx landed at offset 768, twelve lines away from the entry being written,
// and every set paid for both.
type Params struct {
	idx     paramsIndex
	entries [maxParams]param
	path    string
}

type paramsIndex int

// key is a paramID rather than a string so that recording a param stores no
// pointer: no GC write barrier, and 12 bytes instead of 24.
type param struct {
	valueStart int32
	valueEnd   int32
	key        paramID
}

func (p *Params) Use(req *http.Request) {
	p.path = req.URL.Path
}

func (p *Params) Get(key string) string {
	for idx := range p.idx {
		e := p.entries[idx]
		k := nameOf(e.key)

		if key == k {
			if len(p.path) < int(e.valueEnd) {
				return ""
			}

			return p.path[e.valueStart:e.valueEnd]
		}
	}

	return ""
}

func (p *Params) set(key paramID, valueStart, valueEnd int32) {
	p.entries[p.idx] = param{valueStart, valueEnd, key}
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
