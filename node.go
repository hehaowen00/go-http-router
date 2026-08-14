package gohttprouter

import (
	"slices"
	"strings"
)

type nodePtr int32

type handlerPtr int32

type node struct {
	prefix       string
	handlerIdx   handlerPtr
	hasParams    bool
	hasCatchAll  bool
	catchAllNode nodePtr
	slashChild   nodePtr
	fingerprint  []byte
	children     []nodePtr
	wildcard     []wildcard
	catchAllName string
	params       []string
}

func (n *node) appendFingerprint(b byte) {
	if cap(n.fingerprint) == 0 {
		n.fingerprint = make([]byte, 0, 4)
	}

	n.fingerprint = append(n.fingerprint, b)
}

func (n *node) addChild(childIdx nodePtr, b byte) {
	i, _ := slices.BinarySearch(n.fingerprint, b)
	n.children = slices.Insert(n.children, i, childIdx)
	n.fingerprint = slices.Insert(n.fingerprint, i, b)
}

func (n *node) isEmpty() bool {
	return n.handlerIdx < 0 && len(n.children) == 0 && len(n.wildcard) == 0 &&
		!n.hasCatchAll
}

func (n *node) recomputeHasParams(nodes []node) bool {
	if len(n.wildcard) > 0 || n.hasCatchAll {
		return true
	}

	for _, c := range n.children {
		if nodes[c].hasParams {
			return true
		}
	}

	return false
}

type wildcard struct {
	name string
	node nodePtr
}

func newNode(nodes *[]node) nodePtr {
	*nodes = append(*nodes, node{handlerIdx: -1, slashChild: -1})
	return nodePtr(len(*nodes) - 1)
}

func getWildcard(nodes *[]node, idx nodePtr, name string) (int32, bool) {
	n := &(*nodes)[idx]

	for i := range n.wildcard {
		if n.wildcard[i].name == name {
			return int32(i), false
		}
	}

	childIdx := newNode(nodes)
	n = &(*nodes)[idx]

	n.wildcard = append(n.wildcard, wildcard{
		name: name,
		node: childIdx,
	})

	return int32(len(n.wildcard) - 1), true
}

func (n *node) isFreshRun() bool {
	return len(n.params) == 0 && n.handlerIdx < 0 && len(n.children) == 0 &&
		len(n.wildcard) == 0 && !n.hasCatchAll
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

func insertParamRun(nodes *[]node, nodeIdx nodePtr, names []string, rest []string, handlerIdx handlerPtr) bool {
	n := &(*nodes)[nodeIdx]

	wcIdx, created := getWildcard(nodes, nodeIdx, names[0])
	n = &(*nodes)[nodeIdx]
	childIdx := n.wildcard[wcIdx].node

	newParam := insertRunNode(nodes, childIdx, names, rest, handlerIdx)

	n = &(*nodes)[nodeIdx]
	if created || newParam {
		n.hasParams = true
	}

	return created || newParam
}

func insertRunNode(nodes *[]node, nodeIdx nodePtr, names []string, rest []string, handlerIdx handlerPtr) bool {
	n := &(*nodes)[nodeIdx]

	if n.isFreshRun() {
		n.params = names
		return insert(nodes, nodeIdx, rest, handlerIdx)
	}

	cp := commonPrefixLen(n.params, names)

	if cp < len(n.params) {
		splitRunNode(nodes, nodeIdx, cp)
		n = &(*nodes)[nodeIdx]
	}

	names = names[cp:]

	if len(names) == 0 {
		return insert(nodes, nodeIdx, rest, handlerIdx)
	}

	wcIdx, created := getWildcard(nodes, nodeIdx, names[0])
	n = &(*nodes)[nodeIdx]
	childIdx := n.wildcard[wcIdx].node

	newParam := insertRunNode(nodes, childIdx, names, rest, handlerIdx)

	n = &(*nodes)[nodeIdx]
	if created || newParam {
		n.hasParams = true
	}

	return created || newParam
}

func splitRunNode(nodes *[]node, nodeIdx nodePtr, cp int) {
	old := &(*nodes)[nodeIdx]

	newIdx := newNode(nodes)
	moved := &(*nodes)[newIdx]

	moved.params = old.params[cp:]
	moved.handlerIdx = old.handlerIdx
	moved.hasParams = old.hasParams
	moved.hasCatchAll = old.hasCatchAll
	moved.catchAllNode = old.catchAllNode
	moved.slashChild = old.slashChild
	moved.fingerprint = old.fingerprint
	moved.children = old.children
	moved.wildcard = old.wildcard
	moved.catchAllName = old.catchAllName

	old.params = old.params[:cp]
	old.handlerIdx = -1
	old.hasParams = true
	old.hasCatchAll = false
	old.catchAllNode = -1
	old.slashChild = -1
	old.fingerprint = nil
	old.children = nil
	old.wildcard = []wildcard{{name: moved.params[0], node: newIdx}}
	old.catchAllName = ""
}

func removeParamRun(nodes []node, nodeIdx nodePtr, names []string, rest []string) bool {
	n := &nodes[nodeIdx]

	for i := range n.wildcard {
		if n.wildcard[i].name != names[0] {
			continue
		}

		childIdx := n.wildcard[i].node

		removed := removeRunNode(nodes, childIdx, names, rest)
		if !removed {
			return false
		}

		n = &nodes[nodeIdx]
		if nodes[childIdx].isEmpty() {
			n.wildcard = slices.Delete(n.wildcard, i, i+1)
		}
		n.hasParams = n.recomputeHasParams(nodes)

		return true
	}

	return false
}

func removeRunNode(nodes []node, nodeIdx nodePtr, names []string, rest []string) bool {
	n := &nodes[nodeIdx]

	cp := commonPrefixLen(n.params, names)

	if cp < len(n.params) {
		return false
	}

	names = names[cp:]

	if len(names) == 0 {
		return remove(nodes, nodeIdx, rest)
	}

	for i := range n.wildcard {
		if n.wildcard[i].name != names[0] {
			continue
		}

		childIdx := n.wildcard[i].node

		removed := removeRunNode(nodes, childIdx, names, rest)
		if !removed {
			return false
		}

		n = &nodes[nodeIdx]
		if nodes[childIdx].isEmpty() {
			n.wildcard = slices.Delete(n.wildcard, i, i+1)
		}
		n.hasParams = n.recomputeHasParams(nodes)

		return true
	}

	return false
}

func search(
	n *node,
	nodes []node,
	path string,
	idx int,
	params *Params,
) handlerPtr {
	l := len(path)

	for _, name := range n.params {
		if idx >= l {
			return -1
		}

		segStart := idx
		if path[segStart] == '/' {
			segStart++
		}

		if segStart >= l {
			return -1
		}

		segEnd := nextSlash(path, segStart, l)
		params.set(name, path[segStart:segEnd])
		idx = segEnd
	}

	if idx >= l {
		if n.handlerIdx >= 0 {
			return n.handlerIdx
		}

		if n.slashChild >= 0 {
			return nodes[n.slashChild].handlerIdx
		}

		return -1
	}

	if idx == l-1 && path[idx] == '/' {
		if n.handlerIdx >= 0 {
			return n.handlerIdx
		}

		if n.slashChild >= 0 {
			return nodes[n.slashChild].handlerIdx
		}

		return -1
	}

	b := path[idx]
	rem := l - idx

	for j, c := range n.children {
		if b < n.fingerprint[j] {
			break
		}

		if b != n.fingerprint[j] {
			continue
		}

		child := &nodes[c]
		pLen := len(child.prefix)

		if pLen <= rem && path[idx:idx+pLen] == child.prefix {
			var paramsIdx paramsIndex

			if child.hasParams {
				paramsIdx = params.save()
			}

			h := search(child, nodes, path, idx+pLen, params)
			if h >= 0 {
				return h
			}

			if child.hasParams {
				params.restore(paramsIdx)
			}
		} else if pLen == rem+1 && child.prefix[pLen-1] == '/' &&
			path[idx:] == child.prefix[:rem] {
			if child.handlerIdx >= 0 {
				return child.handlerIdx
			}
		}

		break
	}

	if !n.hasParams {
		return -1
	}

	if len(n.wildcard) == 0 && !n.hasCatchAll {
		return -1
	}

	if len(n.wildcard) == 1 && !n.hasCatchAll {
		wc := &n.wildcard[0]
		return search(
			&nodes[wc.node],
			nodes,
			path,
			idx,
			params,
		)
	}

	for wi := 0; wi < len(n.wildcard); wi++ {
		paramsIdx := params.save()

		wc := &n.wildcard[wi]

		h := search(
			&nodes[wc.node],
			nodes,
			path,
			idx,
			params,
		)
		if h >= 0 {
			return h
		}

		params.restore(paramsIdx)
	}

	if n.hasCatchAll {
		params.set(n.catchAllName, strings.TrimPrefix(path[idx:], "/"))
		return search(&nodes[n.catchAllNode], nodes, path, len(path), params)
	}

	return -1
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

		if !n.hasCatchAll {
			childIdx := newNode(nodes)

			n = &(*nodes)[nodeIdx]
			n.catchAllName = name
			n.catchAllNode = childIdx
			n.hasCatchAll = true
		} else {
			n = &(*nodes)[nodeIdx]
			n.catchAllName = name
		}

		insert(nodes, n.catchAllNode, pathSeq[1:], handlerIdx)

		n = &(*nodes)[nodeIdx]
		n.hasParams = true

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
		if b < n.fingerprint[i] {
			break
		}

		if b != n.fingerprint[i] {
			continue
		}

		score := longestMatch(currentSegment, (*nodes)[n.children[i]].prefix)
		if score > best {
			best = score
			closestIdx = i
		}
	}

	if closestIdx < 0 {
		childIdx := newNode(nodes)
		(*nodes)[childIdx].prefix = currentSegment
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
			n.hasParams = true
		}

		return newParam
	}

	closest := &(*nodes)[n.children[closestIdx]]
	if len(closest.prefix) == best {
		if best == len(pathSeq[0]) {
			pathSeq = pathSeq[1:]
		} else {
			pathSeq[0] = pathSeq[0][best:]
		}

		newParam = insert(
			nodes,
			n.children[closestIdx],
			pathSeq,
			handlerIdx,
		)

		if newParam {
			(*nodes)[nodeIdx].hasParams = true
		}

		return newParam
	}

	if len(closest.prefix) > best {
		oldChildIdx := n.children[closestIdx]
		newChildIdx := newNode(nodes)
		n = &(*nodes)[nodeIdx]
		closest = &(*nodes)[n.children[closestIdx]]

		newChild := &(*nodes)[newChildIdx]
		newChild.prefix = closest.prefix[:best]
		newChild.hasParams = closest.hasParams
		closest.prefix = closest.prefix[best:]
		newChild.children = append(newChild.children, n.children[closestIdx])
		newChild.appendFingerprint(closest.prefix[0])

		n.children[closestIdx] = newChildIdx

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
			(*nodes)[nodeIdx].hasParams = true
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
		if !n.hasCatchAll || n.catchAllName != catchAllName(currentSegment) {
			return false
		}

		removed := remove(nodes, n.catchAllNode, pathSeq[1:])
		if removed && nodes[n.catchAllNode].isEmpty() {
			n.hasCatchAll = false
			n.catchAllName = ""
		}

		n.hasParams = n.recomputeHasParams(nodes)

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
		if b < n.fingerprint[i] {
			break
		}

		if b != n.fingerprint[i] {
			continue
		}

		score := longestMatch(currentSegment, nodes[n.children[i]].prefix)
		if score > best {
			best = score
			closestIdx = i
		}
	}

	if closestIdx < 0 {
		return false
	}

	childIdx := n.children[closestIdx]

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
		n.fingerprint = slices.Delete(n.fingerprint, closestIdx, closestIdx+1)

		if n.slashChild == childIdx {
			n.slashChild = -1
		}
	}

	n.hasParams = n.recomputeHasParams(nodes)

	return true
}
