package gohttprouter

import (
	"slices"
	"unsafe"
)

const (
	maxTreeNodes = 1<<15 - 1
	inlineFrames = 4
)

type nodePtr int32

type handlerPtr int32

const (
	flagHasParams uint8 = 1 << iota
	flagHasCatchAll
	flagHasWildcard
	flagPrefixEndsSlash
)

func setFlag(flags *uint8, bit uint8, v bool) {
	if v {
		*flags |= bit
	} else {
		*flags &^= bit
	}
}

type node struct {
	prefix     string
	prefixWord uint64
	children   []childRef
	cold       *nodeCold
	handlerIdx handlerPtr
	slashChild int16
	flags      uint8
}

// search reads only the fields before inline, so their offsets must not move.
type nodeCold struct {
	wildcard     []wildcard
	catchAllNode nodePtr
	catchAllName paramID
	// Backing store for the first wildcard, so a node's cold data and its
	// usual single wildcard are one allocation.
	inline [1]wildcard
}

func (n *node) ensureCold() *nodeCold {
	if n.cold == nil {
		c := &nodeCold{catchAllNode: -1}
		c.wildcard = c.inline[:0]
		n.cold = c
	}
	return n.cold
}

// wildcardSpill reports whether c.wildcard has outgrown inline and lives in
// its own allocation.
func (c *nodeCold) wildcardSpill() bool {
	return cap(c.wildcard) > 0 && &c.wildcard[:1][0] != &c.inline[0]
}

func (n *node) numWildcards() int {
	if n.cold == nil {
		return 0
	}

	return len(n.cold.wildcard)
}

type childRef struct {
	n int16
	b byte
}

type wildcard struct {
	params []paramID
	node   nodePtr
	// Where search resumes after matching this wildcard; see
	// refreshSearchTargets. insert and remove only ever use node.
	searchNode nodePtr
	minRun     uint8
	skip       uint8
}

const maskLen = 9

var wordMask [maskLen]uint64 = (func() [maskLen]uint64 {
	var res [maskLen]uint64

	for i := range maskLen {
		res[i] = ^uint64(0) >> (64 - 8*i)
	}

	return res
})()

func setPrefix(n *node, p string) {
	n.prefix = p

	setFlag(&n.flags, flagPrefixEndsSlash, len(p) > 0 && p[len(p)-1] == '/')

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
		n.children = make([]childRef, 0, 4)
	}

	i, _ := slices.BinarySearchFunc(n.children, b, func(c childRef, t byte) int {
		return int(c.b) - int(t)
	})

	n.children = slices.Insert(n.children, i, childRef{
		n: int16(childIdx),
		b: b,
	})
}

func (n *node) appendChild(childIdx nodePtr, b byte) {
	n.children = append(n.children, childRef{
		n: int16(childIdx),
		b: b,
	})
}

func (n *node) isEmpty() bool {
	return n.handlerIdx < 0 && len(n.children) == 0 &&
		n.flags&(flagHasWildcard|flagHasCatchAll) == 0
}

func (c *nodeCold) recomputeWildcardMinRuns() {
	m := uint8(0)

	for i, v := range slices.Backward(c.wildcard) {
		p := uint8(len(v.params))

		if m == 0 || p < m {
			m = p
		}

		c.wildcard[i].minRun = m
	}
}

func (n *node) recomputeHasParams(nodes []node) bool {
	if n.numWildcards() > 0 || n.flags&flagHasCatchAll != 0 {
		return true
	}

	for i := range len(n.children) {
		c := n.children[i]
		if nodes[nodePtr(c.n)].flags&flagHasParams != 0 {
			return true
		}
	}

	return false
}

func newNode(nodes *[]node) nodePtr {
	*nodes = append(*nodes, node{handlerIdx: -1, slashChild: -1})
	return nodePtr(len(*nodes) - 1)
}

func compactNodes(nodes []node, roots *[methodCount]nodePtr) []node {
	pinned := make([]bool, len(nodes))
	for _, r := range roots {
		if r >= 0 {
			pinned[r] = true
		}
	}

	empty := 0
	for i := range nodes {
		if !pinned[i] && nodes[i].isEmpty() {
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
	for i := range len(nodes) {
		if pinned[i] || !nodes[i].isEmpty() {
			mapping[i] = nodePtr(len(compacted))
			compacted = append(compacted, nodes[i])
		}
	}

	for m := range roots {
		if roots[m] >= 0 {
			roots[m] = mapping[roots[m]]

			if compacted[roots[m]].isEmpty() {
				compacted[roots[m]] = node{handlerIdx: -1, slashChild: -1}
			}
		}
	}

	for i := range len(compacted) {
		n := &compacted[i]

		if n.slashChild >= 0 {
			n.slashChild = int16(mapping[nodePtr(n.slashChild)])
		}

		if n.cold != nil && n.cold.catchAllNode >= 0 {
			n.cold.catchAllNode = mapping[n.cold.catchAllNode]
		}

		for j := range len(n.children) {
			n.children[j].n = int16(mapping[nodePtr(n.children[j].n)])
		}

		for j := range n.numWildcards() {
			n.cold.wildcard[j].node = mapping[n.cold.wildcard[j].node]
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

func commonPrefixLen[T comparable](a, b []T) int {
	i := 0
	for i < len(a) && i < len(b) && a[i] == b[i] {
		i++
	}
	return i
}

func insertParamRun(
	nodes *[]node,
	nodeIdx nodePtr,
	names []paramID,
	rest []string,
	restIDs []paramID,
	handlerIdx handlerPtr,
) bool {
	n := &(*nodes)[nodeIdx]

	for i := range n.numWildcards() {
		if n.cold.wildcard[i].params[0] != names[0] {
			continue
		}

		newParam := insertWildcardRun(nodes, nodeIdx, i, names, rest, restIDs, handlerIdx)

		n = &(*nodes)[nodeIdx]
		if newParam {
			n.flags |= flagHasParams
		}

		return newParam
	}

	childIdx := newNode(nodes)
	n = &(*nodes)[nodeIdx]

	c := n.ensureCold()
	// names may alias the caller's stack buffer, so the stored copy is
	// taken here, where it is kept.
	c.wildcard = append(c.wildcard, wildcard{
		params: slices.Clone(names),
		node:   childIdx,
	})
	c.recomputeWildcardMinRuns()
	n.flags |= flagHasWildcard | flagHasParams
	wcIdx := len(c.wildcard) - 1

	insert(nodes, childIdx, rest, restIDs, handlerIdx)
	refreshSearchTarget(*nodes, nodeIdx, wcIdx)

	return true
}

func insertWildcardRun(
	nodes *[]node,
	parentIdx nodePtr,
	wcIdx int,
	names []paramID,
	rest []string,
	restIDs []paramID,
	handlerIdx handlerPtr,
) bool {
	wc := &(*nodes)[parentIdx].cold.wildcard[wcIdx]

	cp := commonPrefixLen(wc.params, names)

	if cp < len(wc.params) {
		splitWildcard(nodes, parentIdx, wcIdx, cp)
		wc = &(*nodes)[parentIdx].cold.wildcard[wcIdx]
	}

	names = names[cp:]

	var newParam bool
	if len(names) == 0 {
		newParam = insert(nodes, wc.node, rest, restIDs, handlerIdx)
	} else {
		newParam = insertParamRun(nodes, wc.node, names, rest, restIDs, handlerIdx)
	}

	// Anything this insert changed lies under wc.node, so its search target
	// is the only one that can have moved.
	refreshSearchTarget(*nodes, parentIdx, wcIdx)

	return newParam
}

func splitWildcard(nodes *[]node, parentIdx nodePtr, wcIdx int, cp int) {
	wc := &(*nodes)[parentIdx].cold.wildcard[wcIdx]

	oldNode := wc.node
	remainder := wc.params[cp:]
	wc.params = wc.params[:cp]

	newIdx := newNode(nodes)
	moved := &(*nodes)[newIdx]

	moved.flags |= flagHasParams | flagHasWildcard
	mc := moved.ensureCold()
	mc.wildcard = []wildcard{{params: remainder, node: oldNode}}
	mc.recomputeWildcardMinRuns()
	refreshSearchTarget(*nodes, newIdx, 0)

	wc.node = newIdx
	(*nodes)[parentIdx].cold.recomputeWildcardMinRuns()
}

func removeParamRun(
	nodes []node,
	nodeIdx nodePtr,
	names []paramID,
	rest []string,
) bool {
	n := &nodes[nodeIdx]

	for i := range n.numWildcards() {
		if n.cold.wildcard[i].params[0] != names[0] {
			continue
		}

		removed := removeWildcardRun(nodes, nodeIdx, i, names, rest)
		if !removed {
			return false
		}

		n = &nodes[nodeIdx]
		if nodes[n.cold.wildcard[i].node].isEmpty() {
			n.cold.wildcard = slices.Delete(n.cold.wildcard, i, i+1)
			n.cold.recomputeWildcardMinRuns()
			setFlag(&n.flags, flagHasWildcard, n.numWildcards() > 0)
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
	names []paramID,
	rest []string,
) bool {
	wc := &nodes[parentIdx].cold.wildcard[wcIdx]

	cp := commonPrefixLen(wc.params, names)

	if cp < len(wc.params) {
		return false
	}

	names = names[cp:]

	if len(names) == 0 {
		return remove(nodes, wc.node, rest)
	}

	cont := &nodes[wc.node]
	for i := range cont.numWildcards() {
		if cont.cold.wildcard[i].params[0] != names[0] {
			continue
		}

		removed := removeWildcardRun(nodes, wc.node, i, names, rest)
		if !removed {
			return false
		}

		cont = &nodes[wc.node]
		if nodes[cont.cold.wildcard[i].node].isEmpty() {
			cont.cold.wildcard = slices.Delete(cont.cold.wildcard, i, i+1)
			cont.cold.recomputeWildcardMinRuns()
			setFlag(&cont.flags, flagHasWildcard, cont.numWildcards() > 0)
		}

		setFlag(&cont.flags, flagHasParams, cont.recomputeHasParams(nodes))

		return true
	}

	return false
}

// Fields are int32 so a frame is 16 bytes: search zeroes the inline frames
// on every call, and a push is two stores instead of four.
type searchFrame struct {
	n         nodePtr
	idx       int32
	paramsIdx int32
	wi        int32
}

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

	if wi >= n.numWildcards() {
		return false
	}

	need := int(n.cold.wildcard[wi].minRun)
	if need == 0 {
		return false
	}

	next := idx
	for range need {
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

func search(nodes []node, root nodePtr, start int, path string, params *Params) handlerPtr {
	l := len(path)
	n := root
	idx := start
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
			hasWild := nn.flags&(flagHasWildcard|flagHasCatchAll) != 0

			if b == '/' {
				if sc := nn.slashChild; sc >= 0 {
					if hasWild {
						stack.push(searchFrame{n, int32(idx), int32(params.save()), 0})
					}

					n = nodePtr(sc)
					idx++
					nn = &nodes[n]
					continue descent
				}
			}

			rem := l - idx

			nlen := len(nn.children)
			for i := range nlen {
				c := nn.children[i]

				if b != c.b {
					continue
				}

				cnode := nodePtr(c.n)
				child := &nodes[cnode]
				pLen := len(child.prefix)

				if pLen == 1 {
					if hasWild {
						stack.push(searchFrame{n, int32(idx), int32(params.save()), 0})
					}

					n = cnode
					idx++
					nn = child
					continue descent
				}

				if pLen <= rem {
					if rem >= 8 && pLen <= 8 {
						sd := unsafe.StringData(path)
						if *(*uint64)(unsafe.Add(unsafe.Pointer(sd), idx))&wordMask[pLen] == child.prefixWord {
							if hasWild {
								stack.push(searchFrame{n, int32(idx), int32(params.save()), 0})
							}

							n = cnode
							idx += pLen
							nn = child
							continue descent
						}
					} else if pLen <= 8 && idx+pLen >= 8 {
						// Short tail: load the 8 bytes ending at the prefix's
						// end and shift the prefix down into the low bytes.
						sd := unsafe.StringData(path)
						w := *(*uint64)(unsafe.Add(unsafe.Pointer(sd), idx+pLen-8))
						if w>>(8*(8-pLen)) == child.prefixWord {
							if hasWild {
								stack.push(searchFrame{n, int32(idx), int32(params.save()), 0})
							}

							n = cnode
							idx += pLen
							nn = child
							continue descent
						}
					} else if path[idx:idx+pLen] == child.prefix {
						if hasWild {
							stack.push(searchFrame{n, int32(idx), int32(params.save()), 0})
						}

						n = cnode
						idx += pLen
						nn = child
						continue descent
					}
				}

				if pLen == rem+1 && child.flags&flagPrefixEndsSlash != 0 {
					var ok bool
					if rem <= 8 && l >= 8 {
						sd := unsafe.StringData(path)
						w := *(*uint64)(unsafe.Add(unsafe.Pointer(sd), l-8))
						ok = w>>(8*(8-rem)) == child.prefixWord&wordMask[rem]
					} else {
						ok = path[idx:] == child.prefix[:rem]
					}

					if ok && child.handlerIdx >= 0 {
						return child.handlerIdx
					}
				}

				break
			}

			wi = 0
			goto wildcards
		}

	backtrack:
		if stack.empty() {
			return -1
		}

		{
			f := stack.pop()
			params.restore(paramsIndex(f.paramsIdx))
			n = f.n
			idx = int(f.idx)
			wi = int(f.wi)
			nn = &nodes[n]
		}

	wildcards:
		if nn.flags&(flagHasWildcard|flagHasCatchAll) == 0 {
			goto backtrack
		}

		for ; wi < len(nn.cold.wildcard); wi++ {
			wc := &nn.cold.wildcard[wi]
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
				params.set(name, int32(segStart), int32(segEnd))
				next = segEnd
			}

			if !ok {
				params.restore(saved)
				continue
			}

			if wi == len(nn.cold.wildcard)-1 && nn.flags&flagHasCatchAll == 0 {
				n = wc.searchNode
				idx = next + int(wc.skip)
				nn = &nodes[n]
				continue descent
			}

			if !nn.canBacktrack(path, idx, l, wi+1) {
				n = wc.searchNode
				idx = next + int(wc.skip)
				nn = &nodes[n]
				continue descent
			}

			stack.push(searchFrame{n, int32(idx), int32(saved), int32(wi + 1)})
			n = wc.searchNode
			idx = next + int(wc.skip)
			nn = &nodes[n]
			continue descent
		}

		if nn.flags&flagHasCatchAll != 0 {
			start := idx
			if start < l && path[start] == '/' {
				start++
			}

			params.set(nn.cold.catchAllName, int32(start), int32(l))
			n = nn.cold.catchAllNode
			idx = l
			nn = &nodes[n]
			continue descent
		}

		goto backtrack
	}
}

// ids[i] is the interned name of pathSeq[i] when it is a param or catch-all,
// resolved once by Add, and is unused for static segments.
func insert(
	nodes *[]node,
	nodeIdx nodePtr,
	pathSeq []string,
	ids []paramID,
	handlerIdx handlerPtr,
) (newParam bool) {
	if len(pathSeq) == 0 {
		(*nodes)[nodeIdx].handlerIdx = handlerIdx
		return false
	}

	currentSegment := pathSeq[0]
	n := &(*nodes)[nodeIdx]

	if isCatchAll(currentSegment) {
		name := ids[0]

		if n.flags&flagHasCatchAll == 0 {
			childIdx := newNode(nodes)

			n = &(*nodes)[nodeIdx]
			c := n.ensureCold()
			c.catchAllName = name
			c.catchAllNode = childIdx
			n.flags |= flagHasCatchAll
		} else {
			n = &(*nodes)[nodeIdx]
			n.cold.catchAllName = name
		}

		insert(nodes, (*nodes)[nodeIdx].cold.catchAllNode, pathSeq[1:], ids[1:], handlerIdx)

		n = &(*nodes)[nodeIdx]
		n.flags |= flagHasParams

		return true
	}

	if isParam(currentSegment) {
		k := len(collectParamRun(pathSeq))

		return insertParamRun(
			nodes, nodeIdx, ids[:k], pathSeq[k:], ids[k:], handlerIdx,
		)
	}

	closestIdx := -1
	best := 0
	b := currentSegment[0]

	for i := range n.children {
		if b < n.children[i].b {
			break
		}

		if b != n.children[i].b {
			continue
		}

		score := longestMatch(currentSegment, (*nodes)[n.children[i].n].prefix)
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
			ids[1:],
			handlerIdx,
		)

		n = &(*nodes)[nodeIdx]
		n.addChild(childIdx, (*nodes)[childIdx].prefix[0])

		if currentSegment == "/" {
			n.slashChild = int16(childIdx)
		}

		if newParam {
			n.flags |= flagHasParams
		}

		return newParam
	}

	closest := &(*nodes)[n.children[closestIdx].n]
	if len(closest.prefix) == best {
		if best == len(pathSeq[0]) {
			pathSeq = pathSeq[1:]
			ids = ids[1:]
		} else {
			pathSeq[0] = pathSeq[0][best:]
		}

		newParam = insert(
			nodes,
			nodePtr(n.children[closestIdx].n),
			pathSeq,
			ids,
			handlerIdx,
		)

		if newParam {
			(*nodes)[nodeIdx].flags |= flagHasParams
		}

		return newParam
	}

	if len(closest.prefix) > best {
		oldChildIdx := nodePtr(n.children[closestIdx].n)
		newChildIdx := newNode(nodes)
		n = &(*nodes)[nodeIdx]
		closest = &(*nodes)[n.children[closestIdx].n]

		newChild := &(*nodes)[newChildIdx]
		setPrefix(newChild, closest.prefix[:best])
		newChild.flags = closest.flags & flagHasParams
		setPrefix(closest, closest.prefix[best:])
		newChild.appendChild(oldChildIdx, closest.prefix[0])

		n.children[closestIdx].n = int16(newChildIdx)

		if newChild.prefix == "/" {
			n.slashChild = int16(newChildIdx)
		} else if closest.prefix == "/" {
			newChild.slashChild = int16(oldChildIdx)
		}

		if best >= len(currentSegment) {
			pathSeq = pathSeq[1:]
			ids = ids[1:]
		} else {
			pathSeq[0] = pathSeq[0][best:]
		}

		newParam = insert(nodes, newChildIdx, pathSeq, ids, handlerIdx)

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
		if n.flags&flagHasCatchAll == 0 || n.cold == nil ||
			n.cold.catchAllName != mustInternName(catchAllName(currentSegment)) {
			return false
		}

		removed := remove(nodes, n.cold.catchAllNode, pathSeq[1:])
		if removed && nodes[n.cold.catchAllNode].isEmpty() {
			n.flags &^= flagHasCatchAll
			n.cold.catchAllName = 0
		}

		setFlag(&n.flags, flagHasParams, n.recomputeHasParams(nodes))

		return removed
	}

	if isParam(currentSegment) {
		run := collectParamRun(pathSeq)
		rest := pathSeq[len(run):]

		names := make([]paramID, len(run))
		for i := range len(run) {
			names[i] = mustInternName(paramName(run[i]))
		}

		return removeParamRun(nodes, nodeIdx, names, rest)
	}

	closestIdx := -1
	best := 0
	b := currentSegment[0]

	for i := range len(n.children) {
		if b < n.children[i].b {
			break
		}

		if b != n.children[i].b {
			continue
		}

		score := longestMatch(currentSegment, nodes[n.children[i].n].prefix)
		if score > best {
			best = score
			closestIdx = i
		}
	}

	if closestIdx < 0 {
		return false
	}

	childIdx := nodePtr(n.children[closestIdx].n)

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

		if nodePtr(n.slashChild) == childIdx {
			n.slashChild = -1
		}
	}

	setFlag(&n.flags, flagHasParams, n.recomputeHasParams(nodes))

	return true
}

// slashOnlyChild reports the '/' child of a node whose only way forward is
// that child: no handler, no wildcard or catch-all, no other children.
func slashOnlyChild(n *node) (nodePtr, bool) {
	if n.handlerIdx < 0 && n.slashChild >= 0 && len(n.children) == 1 &&
		n.flags&(flagHasWildcard|flagHasCatchAll) == 0 {
		return nodePtr(n.slashChild), true
	}

	return -1, false
}

// refreshSearchTargets points every wildcard's searchNode past a slash-only
// target node. A param value always ends at '/' or the end of the path, so
// the skipped node could only ever step into its '/' child, and past the end
// the terminal check still sees the same handler.
//
// Insert keeps targets current one wildcard at a time; this full pass is only
// needed after compactNodes renumbers the arena.
func refreshSearchTargets(nodes []node) {
	for i := range nodes {
		for j := range nodes[i].numWildcards() {
			refreshSearchTarget(nodes, nodePtr(i), j)
		}
	}
}

func refreshSearchTarget(nodes []node, parent nodePtr, wcIdx int) {
	wc := &nodes[parent].cold.wildcard[wcIdx]
	wc.searchNode, wc.skip = wc.node, 0

	if sc, ok := slashOnlyChild(&nodes[wc.node]); ok {
		wc.searchNode, wc.skip = sc, 1
	}
}
