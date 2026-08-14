package gohttprouter

const maxParams = 32

// cannot have more than 32 params
// paths are validated on router build to not have more than 32 params
type Params struct {
	entries [maxParams]param
	idx     paramsIndex
}

type paramsIndex int

type param struct {
	key   string
	value string
}

func (p *Params) Get(key string) string {
	for idx := range p.idx {
		e := p.entries[idx]

		if key[0] == e.key[0] && key == e.key {
			return e.value
		}
	}

	return ""
}

func (p *Params) set(key string, value string) {
	if p.idx >= 32 {
		return
	}

	p.entries[p.idx] = param{key, value}
	p.idx++
}

// func (p *Params) clear() {
// 	p.idx = 0
// }

func (p *Params) save() paramsIndex {
	return p.idx
}

func (p *Params) restore(idx paramsIndex) {
	// clear(p.entries[idx:p.idx])
	p.idx = idx
}

func (p *Params) reset() {
	if p.idx > 0 {
		clear(p.entries[:p.idx])
		p.idx = 0
	}
}
