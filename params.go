package gohttprouter

type Params struct {
	entries [32]param
	idx     int
}

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

func (p *Params) clear() {
	p.idx = 0
}

func (p *Params) reset() {
	clear(p.entries[:p.idx])
	p.idx = 0
}
