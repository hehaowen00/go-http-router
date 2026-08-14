package gohttprouter

import (
	"slices"
	"strings"
)

type nodePtr int32

type handlerPtr int32

type node[T any] struct {
	prefix       string
	handlerIdx   handlerPtr
	hasParams    bool
	hasCatchAll  bool
	catchAllNode nodePtr
	fingerprint  []byte
	children     []nodePtr
	wildcard     []wildcard
	catchAllName string
}

func (n *node[T]) appendFingerprint(b byte) {
	if cap(n.fingerprint) == 0 {
		n.fingerprint = make([]byte, 0, 4)
	}

	n.fingerprint = append(n.fingerprint, b)
}

func (n *node[T]) isEmpty() bool {
	return n.handlerIdx < 0 && len(n.children) == 0 && len(n.wildcard) == 0 &&
		!n.hasCatchAll
}

func (n *node[T]) recomputeHasParams(nodes []node[T]) bool {
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

func newNode[T any](nodes *[]node[T]) nodePtr {
	// handlerIdx defaults to -1: a zero value of 0 would alias a real
	// handler in the arena.
	*nodes = append(*nodes, node[T]{handlerIdx: -1})
	return nodePtr(len(*nodes) - 1)
}

func getWildcard[T any](nodes *[]node[T], idx nodePtr, name string) (int32, bool) {
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

func search[T any](
	n *node[T],
	nodes []node[T],
	path string,
	idx int,
	params *Params,
) handlerPtr {
	l := len(path)

	if idx == l || (idx == l-1 && path[idx] == '/') {
		if n.handlerIdx >= 0 {
			return n.handlerIdx
		}

		for _, c := range n.children {
			if nodes[c].prefix == "/" {
				return nodes[c].handlerIdx
			}
		}

		return -1
	}

	b := path[idx]
	rem := l - idx

	for j, c := range n.children {
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

	segmentStart := idx
	if path[segmentStart] == '/' {
		segmentStart++
	}

	segmentEnd := nextSlash(path, segmentStart, len(path))
	value := path[segmentStart:segmentEnd]

	if len(n.wildcard) == 1 && !n.hasCatchAll {
		wc := &n.wildcard[0]
		params.set(wc.name, value)

		h := search(
			&nodes[wc.node],
			nodes,
			path,
			segmentEnd,
			params,
		)
		return h
	}

	for wi := 0; wi < len(n.wildcard); wi++ {
		paramsIdx := params.save()

		wc := &n.wildcard[wi]
		params.set(wc.name, value)

		h := search(
			&nodes[wc.node],
			nodes,
			path,
			segmentEnd,
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

func insert[T any](
	nodes *[]node[T],
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
		name := paramName(currentSegment)

		wildcardIdx, created := getWildcard(nodes, nodeIdx, name)
		n = &(*nodes)[nodeIdx]

		pathSeq = pathSeq[1:]
		newParam = insert(
			nodes,
			n.wildcard[wildcardIdx].node,
			pathSeq,
			handlerIdx,
		)

		n = &(*nodes)[nodeIdx]

		if created || newParam {
			n.hasParams = true
		}

		return created || newParam
	}

	closestIdx := -1
	best := 0
	b := currentSegment[0]

	for i := range n.children {
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
		n.children = append(n.children, childIdx)
		n.appendFingerprint((*nodes)[childIdx].prefix[0])

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

func remove[T any](nodes []node[T], nodeIdx nodePtr, pathSeq []string) bool {
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
		name := paramName(currentSegment)

		for i := range n.wildcard {
			if n.wildcard[i].name != name {
				continue
			}

			childIdx := n.wildcard[i].node

			removed := remove(nodes, childIdx, pathSeq[1:])
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

	closestIdx := -1
	best := 0
	b := currentSegment[0]

	for i := range n.children {
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
	}

	n.hasParams = n.recomputeHasParams(nodes)

	return true
}
