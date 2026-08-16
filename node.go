package gohttprouter

import (
	"slices"
	"strings"
	"unsafe"
)

type nodePtr int32

type handlerPtr int32

const (
	flagHasParams uint8 = 1 << iota
	flagHasCatchAll
	flagHasWildcard
)

func setFlag(flags *uint8, bit uint8, v bool) {
	if v {
		*flags |= bit
	} else {
		*flags &^= bit
	}
}

type node struct {
	prefix       string
	prefixWord   uint64
	handlerIdx   handlerPtr
	slashChild   nodePtr
	catchAllNode nodePtr
	flags        uint8
	children     []child
	wildcard     []wildcard
	catchAllName string
}

type child struct {
	b    byte
	node nodePtr
}

type wildcard struct {
	params []string
	node   nodePtr
	minRun uint8
}

var wordMask [9]uint64

func init() {
	for i := 1; i < len(wordMask); i++ {
		wordMask[i] = ^uint64(0) >> (64 - 8*i)
	}
}

func setPrefix(n *node, p string) {
	n.prefix = p
	if len(p) > 8 {
		p = p[:8]
	}
	var w uint64
	for i := 0; i < len(p); i++ {
		w |= uint64(p[i]) << (8 * i)
	}
	n.prefixWord = w
}

func (n *node) addChild(childIdx nodePtr, b byte) {
	if cap(n.children) == 0 {
		n.children = make([]child, 0, 4)
	}

	i, _ := slices.BinarySearchFunc(n.children, b, func(c child, t byte) int {
		return int(c.b) - int(t)
	})
	n.children = slices.Insert(n.children, i, child{b: b, node: childIdx})
}

func (n *node) appendChild(childIdx nodePtr, b byte) {
	n.children = append(n.children, child{b: b, node: childIdx})
}

func (n *node) isEmpty() bool {
	return n.handlerIdx < 0 && len(n.children) == 0 &&
		n.flags&(flagHasWildcard|flagHasCatchAll) == 0
}

func (n *node) recomputeWildcardMinRuns() {
	m := uint8(0)
	for i := len(n.wildcard) - 1; i >= 0; i-- {
		p := uint8(len(n.wildcard[i].params))
		if m == 0 || p < m {
			m = p
		}
		n.wildcard[i].minRun = m
	}
}

func (n *node) recomputeHasParams(nodes []node) bool {
	if len(n.wildcard) > 0 || n.flags&flagHasCatchAll != 0 {
		return true
	}

	for _, c := range n.children {
		if nodes[c.node].flags&flagHasParams != 0 {
			return true
		}
	}

	return false
}

func newNode(nodes *[]node) nodePtr {
	*nodes = append(*nodes, node{handlerIdx: -1, slashChild: -1})
	return nodePtr(len(*nodes) - 1)
}

func compactNodes(nodes []node) []node {
	empty := 0
	for i := 1; i < len(nodes); i++ {
		if nodes[i].isEmpty() {
			empty++
		}
	}

	if empty == 0 {
		return nodes
	}

	mapping := make([]nodePtr, len(nodes))
	for i := range mapping {
		mapping[i] = -1
	}

	compacted := make([]node, 0, len(nodes)-empty)
	for i := range nodes {
		if i == 0 || !nodes[i].isEmpty() {
			mapping[i] = nodePtr(len(compacted))
			compacted = append(compacted, nodes[i])
		}
	}

	if compacted[0].isEmpty() {
		compacted[0] = node{handlerIdx: -1, slashChild: -1}
	}

	for i := range compacted {
		n := &compacted[i]

		if n.slashChild >= 0 {
			n.slashChild = mapping[n.slashChild]
		}

		if n.catchAllNode >= 0 {
			n.catchAllNode = mapping[n.catchAllNode]
		}

		for j := range n.children {
			n.children[j].node = mapping[n.children[j].node]
		}

		for j := range n.wildcard {
			n.wildcard[j].node = mapping[n.wildcard[j].node]
		}
	}

	return compacted
}

func collectParamRun(pathSeq []string) []string {
	i := 0
	for i < len(pathSeq) && isParam(pathSeq[i]) {
		i++
	}
	return pathSeq[:i]
}

func commonPrefixLen(a, b []string) int {
	i := 0
	for i < len(a) && i < len(b) && a[i] == b[i] {
		i++
	}
	return i
}

func insertParamRun(
	nodes *[]node,
	nodeIdx nodePtr,
	names []string,
	rest []string,
	handlerIdx handlerPtr,
) bool {
	n := &(*nodes)[nodeIdx]

	for i := range n.wildcard {
		if n.wildcard[i].params[0] != names[0] {
			continue
		}

		newParam := insertWildcardRun(nodes, nodeIdx, i, names, rest, handlerIdx)

		n = &(*nodes)[nodeIdx]
		if newParam {
			n.flags |= flagHasParams
		}

		return newParam
	}

	childIdx := newNode(nodes)
	n = &(*nodes)[nodeIdx]

	n.wildcard = append(n.wildcard, wildcard{
		params: names,
		node:   childIdx,
	})
	n.recomputeWildcardMinRuns()
	n.flags |= flagHasWildcard | flagHasParams

	insert(nodes, childIdx, rest, handlerIdx)

	return true
}

func insertWildcardRun(
	nodes *[]node,
	parentIdx nodePtr,
	wcIdx int,
	names []string,
	rest []string,
	handlerIdx handlerPtr,
) bool {
	wc := &(*nodes)[parentIdx].wildcard[wcIdx]

	cp := commonPrefixLen(wc.params, names)

	if cp < len(wc.params) {
		splitWildcard(nodes, parentIdx, wcIdx, cp)
		wc = &(*nodes)[parentIdx].wildcard[wcIdx]
	}

	names = names[cp:]

	if len(names) == 0 {
		return insert(nodes, wc.node, rest, handlerIdx)
	}

	return insertParamRun(nodes, wc.node, names, rest, handlerIdx)
}

func splitWildcard(nodes *[]node, parentIdx nodePtr, wcIdx int, cp int) {
	wc := &(*nodes)[parentIdx].wildcard[wcIdx]

	oldNode := wc.node
	remainder := wc.params[cp:]
	wc.params = wc.params[:cp]

	newIdx := newNode(nodes)
	moved := &(*nodes)[newIdx]

	moved.flags |= flagHasParams | flagHasWildcard
	moved.wildcard = []wildcard{{params: remainder, node: oldNode}}
	moved.recomputeWildcardMinRuns()

	wc.node = newIdx
	(*nodes)[parentIdx].recomputeWildcardMinRuns()
}

func removeParamRun(
	nodes []node,
	nodeIdx nodePtr,
	names []string,
	rest []string,
) bool {
	n := &nodes[nodeIdx]

	for i := range n.wildcard {
		if n.wildcard[i].params[0] != names[0] {
			continue
		}

		removed := removeWildcardRun(nodes, nodeIdx, i, names, rest)
		if !removed {
			return false
		}

		n = &nodes[nodeIdx]
		if nodes[n.wildcard[i].node].isEmpty() {
			n.wildcard = slices.Delete(n.wildcard, i, i+1)
			n.recomputeWildcardMinRuns()
			setFlag(&n.flags, flagHasWildcard, len(n.wildcard) > 0)
		}
		setFlag(&n.flags, flagHasParams, n.recomputeHasParams(nodes))

		return true
	}

	return false
}

func removeWildcardRun(
	nodes []node,
	parentIdx nodePtr,
	wcIdx int,
	names []string,
	rest []string,
) bool {
	wc := &nodes[parentIdx].wildcard[wcIdx]

	cp := commonPrefixLen(wc.params, names)

	if cp < len(wc.params) {
		return false
	}

	names = names[cp:]

	if len(names) == 0 {
		return remove(nodes, wc.node, rest)
	}

	cont := &nodes[wc.node]
	for i := range cont.wildcard {
		if cont.wildcard[i].params[0] != names[0] {
			continue
		}

		removed := removeWildcardRun(nodes, wc.node, i, names, rest)
		if !removed {
			return false
		}

		cont = &nodes[wc.node]
		if nodes[cont.wildcard[i].node].isEmpty() {
			cont.wildcard = slices.Delete(cont.wildcard, i, i+1)
			cont.recomputeWildcardMinRuns()
			setFlag(&cont.flags, flagHasWildcard, len(cont.wildcard) > 0)
		}
		setFlag(&cont.flags, flagHasParams, cont.recomputeHasParams(nodes))

		return true
	}

	return false
}

type searchFrame struct {
	n         nodePtr
	idx       int
	paramsIdx paramsIndex
	wi        int
}

const inlineFrames = 4

type frameStack struct {
	frames [inlineFrames]searchFrame
	sp     int
	spill  []searchFrame
}

func (s *frameStack) push(f searchFrame) {
	if s.sp < len(s.frames) {
		s.frames[s.sp] = f
		s.sp++
		return
	}
	s.spill = append(s.spill, f)
}

func (s *frameStack) pop() searchFrame {
	if len(s.spill) > 0 {
		f := s.spill[len(s.spill)-1]
		s.spill = s.spill[:len(s.spill)-1]
		return f
	}
	s.sp--
	return s.frames[s.sp]
}

func (s *frameStack) empty() bool {
	return s.sp == 0 && len(s.spill) == 0
}

func (n *node) canBacktrack(path string, idx, l, wi int) bool {
	if n.flags&flagHasCatchAll != 0 {
		return true
	}

	if wi >= len(n.wildcard) {
		return false
	}
	need := int(n.wildcard[wi].minRun)
	if need == 0 {
		return false
	}

	next := idx
	for c := 0; c < need; c++ {
		if next >= l {
			return false
		}

		segStart := next
		if path[segStart] == '/' {
			segStart++
		}

		if segStart >= l {
			return false
		}

		next = nextSlash(path, segStart, l)
	}

	return true
}

func search(nodes []node, path string, params *Params) handlerPtr {
	l := len(path)
	n := nodePtr(0)
	idx := 0
	wi := 0
	nn := &nodes[n]
	var stack frameStack

descent:
	for {
		if idx >= l || (idx == l-1 && path[idx] == '/') {
			if nn.handlerIdx >= 0 {
				return nn.handlerIdx
			}

			if sc := nn.slashChild; sc >= 0 {
				return nodes[sc].handlerIdx
			}

			goto backtrack
		}

		{
			b := path[idx]

			if b == '/' {
				if sc := nn.slashChild; sc >= 0 {
					if nn.flags&(flagHasWildcard|flagHasCatchAll) != 0 {
						stack.push(searchFrame{n, idx, params.save(), 0})
					}

					n = sc
					idx++
					nn = &nodes[n]
					continue descent
				}
			}

			rem := l - idx

			for i := range nn.children {
				c := nn.children[i]
				if b < c.b {
					break
				}

				if b != c.b {
					continue
				}

				child := &nodes[c.node]
				pLen := len(child.prefix)

				if pLen == 1 {
					if nn.flags&(flagHasWildcard|flagHasCatchAll) != 0 {
						stack.push(searchFrame{n, idx, params.save(), 0})
					}

					n = c.node
					idx++
					nn = child
					continue descent
				}

				if pLen <= rem {
					matched := false
					if rem >= 8 && pLen <= 8 {
						sd := unsafe.StringData(path)
						matched = *(*uint64)(unsafe.Add(unsafe.Pointer(sd), idx))&wordMask[pLen] == child.prefixWord
					} else {
						matched = path[idx:idx+pLen] == child.prefix
					}

					if matched {
						if nn.flags&(flagHasWildcard|flagHasCatchAll) != 0 {
							stack.push(searchFrame{n, idx, params.save(), 0})
						}

						n = c.node
						idx += pLen
						nn = child
						continue descent
					}
				}

				if pLen == rem+1 && child.prefix[pLen-1] == '/' &&
					path[idx:] == child.prefix[:rem] {
					if child.handlerIdx >= 0 {
						return child.handlerIdx
					}
				}

				break
			}
		}

		wi = 0
		goto wildcards

	backtrack:
		if stack.empty() {
			return -1
		}

		{
			f := stack.pop()
			params.restore(f.paramsIdx)
			n = f.n
			idx = f.idx
			wi = f.wi
			nn = &nodes[n]
		}

	wildcards:
		for ; wi < len(nn.wildcard); wi++ {
			wc := &nn.wildcard[wi]
			saved := params.save()

			next := idx
			ok := true

			for i := range wc.params {
				name := wc.params[i]

				if next >= l {
					ok = false
					break
				}

				segStart := next
				if path[segStart] == '/' {
					segStart++
				}

				if segStart >= l {
					ok = false
					break
				}

				segEnd := nextSlash(path, segStart, l)
				params.set(name, path[segStart:segEnd])
				next = segEnd
			}

			if !ok {
				params.restore(saved)
				continue
			}

			if wi == len(nn.wildcard)-1 && nn.flags&flagHasCatchAll == 0 {
				n = wc.node
				idx = next
				nn = &nodes[n]
				continue descent
			}

			if !nn.canBacktrack(path, idx, l, wi+1) {
				n = wc.node
				idx = next
				nn = &nodes[n]
				continue descent
			}

			stack.push(searchFrame{n, idx, saved, wi + 1})
			n = wc.node
			idx = next
			nn = &nodes[n]
			continue descent
		}

		if nn.flags&flagHasCatchAll != 0 {
			params.set(nn.catchAllName, strings.TrimPrefix(path[idx:], "/"))
			n = nn.catchAllNode
			idx = l
			nn = &nodes[n]
			continue descent
		}

		goto backtrack
	}
}

func insert(
	nodes *[]node,
	nodeIdx nodePtr,
	pathSeq []string,
	handlerIdx handlerPtr,
) (newParam bool) {
	if len(pathSeq) == 0 {
		(*nodes)[nodeIdx].handlerIdx = handlerIdx
		return false
	}

	currentSegment := pathSeq[0]
	n := &(*nodes)[nodeIdx]

	if isCatchAll(currentSegment) {
		name := catchAllName(currentSegment)

		if n.flags&flagHasCatchAll == 0 {
			childIdx := newNode(nodes)

			n = &(*nodes)[nodeIdx]
			n.catchAllName = name
			n.catchAllNode = childIdx
			n.flags |= flagHasCatchAll
		} else {
			n = &(*nodes)[nodeIdx]
			n.catchAllName = name
		}

		insert(nodes, n.catchAllNode, pathSeq[1:], handlerIdx)

		n = &(*nodes)[nodeIdx]
		n.flags |= flagHasParams

		return true
	}

	if isParam(currentSegment) {
		run := collectParamRun(pathSeq)
		rest := pathSeq[len(run):]

		names := make([]string, len(run))
		for i := range run {
			names[i] = paramName(run[i])
		}

		return insertParamRun(nodes, nodeIdx, names, rest, handlerIdx)
	}

	closestIdx := -1
	best := 0
	b := currentSegment[0]

	for i := range n.children {
		c := &n.children[i]

		if b < c.b {
			break
		}

		if b != c.b {
			continue
		}

		score := longestMatch(currentSegment, (*nodes)[c.node].prefix)
		if score > best {
			best = score
			closestIdx = i
		}
	}

	if closestIdx < 0 {
		childIdx := newNode(nodes)
		setPrefix(&(*nodes)[childIdx], currentSegment)
		newParam = insert(
			nodes,
			childIdx,
			pathSeq[1:],
			handlerIdx,
		)

		n = &(*nodes)[nodeIdx]
		n.addChild(childIdx, (*nodes)[childIdx].prefix[0])

		if currentSegment == "/" {
			n.slashChild = childIdx
		}

		if newParam {
			n.flags |= flagHasParams
		}

		return newParam
	}

	closest := &(*nodes)[n.children[closestIdx].node]
	if len(closest.prefix) == best {
		if best == len(pathSeq[0]) {
			pathSeq = pathSeq[1:]
		} else {
			pathSeq[0] = pathSeq[0][best:]
		}

		newParam = insert(
			nodes,
			n.children[closestIdx].node,
			pathSeq,
			handlerIdx,
		)

		if newParam {
			(*nodes)[nodeIdx].flags |= flagHasParams
		}

		return newParam
	}

	if len(closest.prefix) > best {
		oldChildIdx := n.children[closestIdx].node
		newChildIdx := newNode(nodes)
		n = &(*nodes)[nodeIdx]
		closest = &(*nodes)[n.children[closestIdx].node]

		newChild := &(*nodes)[newChildIdx]
		setPrefix(newChild, closest.prefix[:best])
		newChild.flags = closest.flags & flagHasParams
		setPrefix(closest, closest.prefix[best:])
		newChild.appendChild(oldChildIdx, closest.prefix[0])

		n.children[closestIdx].node = newChildIdx

		if newChild.prefix == "/" {
			n.slashChild = newChildIdx
		} else if closest.prefix == "/" {
			newChild.slashChild = oldChildIdx
		}

		if best >= len(currentSegment) {
			pathSeq = pathSeq[1:]
		} else {
			pathSeq[0] = pathSeq[0][best:]
		}

		newParam = insert(nodes, newChildIdx, pathSeq, handlerIdx)

		if newParam {
			(*nodes)[nodeIdx].flags |= flagHasParams
		}

		return newParam
	}

	return false
}

func remove(nodes []node, nodeIdx nodePtr, pathSeq []string) bool {
	if len(pathSeq) == 0 {
		n := &nodes[nodeIdx]

		if n.handlerIdx < 0 {
			return false
		}

		n.handlerIdx = -1
		return true
	}

	currentSegment := pathSeq[0]
	n := &nodes[nodeIdx]

	if isCatchAll(currentSegment) {
		if n.flags&flagHasCatchAll == 0 ||
			n.catchAllName != catchAllName(currentSegment) {
			return false
		}

		removed := remove(nodes, n.catchAllNode, pathSeq[1:])
		if removed && nodes[n.catchAllNode].isEmpty() {
			n.flags &^= flagHasCatchAll
			n.catchAllName = ""
		}

		setFlag(&n.flags, flagHasParams, n.recomputeHasParams(nodes))

		return removed
	}

	if isParam(currentSegment) {
		run := collectParamRun(pathSeq)
		rest := pathSeq[len(run):]

		names := make([]string, len(run))
		for i := range run {
			names[i] = paramName(run[i])
		}

		return removeParamRun(nodes, nodeIdx, names, rest)
	}

	closestIdx := -1
	best := 0
	b := currentSegment[0]

	for i := range n.children {
		c := &n.children[i]

		if b < c.b {
			break
		}

		if b != c.b {
			continue
		}

		score := longestMatch(currentSegment, nodes[c.node].prefix)
		if score > best {
			best = score
			closestIdx = i
		}
	}

	if closestIdx < 0 {
		return false
	}

	childIdx := n.children[closestIdx].node

	if len(nodes[childIdx].prefix) > best {
		return false
	}

	if best < len(currentSegment) {
		pathSeq[0] = currentSegment[best:]
	} else {
		pathSeq = pathSeq[1:]
	}

	removed := remove(nodes, childIdx, pathSeq)
	if !removed {
		return false
	}

	n = &nodes[nodeIdx]

	if nodes[childIdx].isEmpty() {
		n.children = slices.Delete(n.children, closestIdx, closestIdx+1)

		if n.slashChild == childIdx {
			n.slashChild = -1
		}
	}

	setFlag(&n.flags, flagHasParams, n.recomputeHasParams(nodes))

	return true
}
