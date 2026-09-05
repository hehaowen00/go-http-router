package gohttprouter

import (
	"sync"
	"sync/atomic"
)

type paramID = uint16

const maxParamNames = 1 << 16

var (
	nameMu    sync.Mutex
	nameIDs   = map[string]paramID{}
	nameTable atomic.Pointer[[]string]
)

func internName(s string) (paramID, bool) {
	nameMu.Lock()
	defer nameMu.Unlock()

	if id, ok := nameIDs[s]; ok {
		return id, true
	}

	var next []string
	if cur := nameTable.Load(); cur != nil {
		next = make([]string, len(*cur), len(*cur)+1)
		copy(next, *cur)
	}

	if len(next) >= maxParamNames {
		return 0, false
	}

	id := paramID(len(next))
	next = append(next, s)
	nameTable.Store(&next)
	nameIDs[s] = id

	return id, true
}

func mustInternName(s string) paramID {
	id, _ := internName(s)
	return id
}

func nameOf(id paramID) string {
	t := nameTable.Load()
	if t == nil || int(id) >= len(*t) {
		return ""
	}

	return (*t)[id]
}
