package gohttprouter

import (
	"slices"
	"strings"
)

type nodePtr int32

type handlerPtr int32

type node struct {
	prefix      string
	handler     handlerPtr
	fingerprint []byte
	children    []nodePtr
	wildcard    []wildcard
}

func (n *node) rebuildFingerprint(r *Router) {
	n.fingerprint = n.fingerprint[:0]

	for _, c := range n.children {
		n.fingerprint = append(n.fingerprint, r.nodes[c].prefix[0])
	}
}

type wildcard struct {
	name string
	node nodePtr
}

func (r *Router) newNode() nodePtr {
	r.nodes = append(r.nodes, node{
		handler: r.newHandler(),
	})

	return nodePtr(len(r.nodes) - 1)
}

func (r *Router) newHandler() handlerPtr {
	r.handlers = append(r.handlers, methodHandler{})
	return handlerPtr(len(r.handlers) - 1)
}

func (r *Router) getWildcard(idx nodePtr, name string) int32 {
	n := &r.nodes[idx]

	for i := range n.wildcard {
		if n.wildcard[i].name == name {
			return int32(i)
		}
	}

	childIdx := r.newNode()
	n = &r.nodes[idx]

	n.wildcard = append(n.wildcard, wildcard{
		name: name,
		node: childIdx,
	})

	return int32(len(n.wildcard) - 1)
}

func (r *Router) search(
	nodeIdx nodePtr,
	method methodEnum,
	path string,
	idx int,
	params *Params,
) Handler {
	n := &r.nodes[nodeIdx]

	if idx == len(path) || (idx == len(path)-1 && path[idx] == '/') {
		if h := r.handlers[n.handler].Get(method); h != nil {
			return h
		}

		for _, c := range n.children {
			if r.nodes[c].prefix == "/" {
				return r.handlers[r.nodes[c].handler].Get(method)
			}
		}

		return nil
	}

	if idx < len(path) {
		b := path[idx]
		rem := len(path) - idx

		for j, c := range n.children {
			if b != n.fingerprint[j] {
				continue
			}

			child := &r.nodes[c]
			pLen := len(child.prefix)

			if pLen <= rem {
				if path[idx:idx+pLen] == child.prefix {
					paramsIdx := params.save()

					h := r.search(c, method, path, idx+pLen, params)
					if h != nil {
						return h
					}

					params.restore(paramsIdx)
				}

				break
			}

			if pLen == rem+1 && child.prefix[pLen-1] == '/' &&
				path[idx:] == child.prefix[:rem] {
				return r.handlers[child.handler].Get(method)
			}

			break
		}
	}

	if len(n.wildcard) == 0 {
		return nil
	}

	segmentStart := idx
	if path[segmentStart] == '/' {
		segmentStart++
	}

	segmentEnd := strings.IndexByte(path[segmentStart:], '/')
	if segmentEnd == -1 {
		segmentEnd = len(path) - segmentStart
	}

	value := path[segmentStart : segmentStart+segmentEnd]

	for wi := range n.wildcard {
		paramsIdx := params.save()

		wc := &n.wildcard[wi]
		params.set(wc.name, value)

		h := r.search(
			nodePtr(wc.node),
			method,
			path,
			segmentStart+segmentEnd,
			params,
		)
		if h != nil {
			return h
		}

		params.restore(paramsIdx)
	}

	return nil
}

func (r *Router) insert(
	nodeIdx nodePtr,
	method methodEnum,
	pathSeq []string,
	handler Handler,
) {
	if len(pathSeq) == 0 {
		r.handlers[r.nodes[nodeIdx].handler].Insert(method, handler)
		return
	}

	currentSegment := pathSeq[0]
	n := &r.nodes[nodeIdx]

	if isParam(currentSegment) {
		name := paramName(currentSegment)

		wildcardIdx := r.getWildcard(nodeIdx, name)
		n = &r.nodes[nodeIdx]

		pathSeq = slices.Delete(pathSeq, 0, 1)
		r.insert(n.wildcard[wildcardIdx].node, method, pathSeq, handler)

		return
	}

	closestIdx := -1
	best := 0

	for i := range n.children {
		score := longestMatch(currentSegment, r.nodes[n.children[i]].prefix)
		if score > best {
			best = score
			closestIdx = i
		}
	}

	if closestIdx < 0 {
		childIdx := r.newNode()
		r.nodes[childIdx].prefix = currentSegment
		r.insert(childIdx, method, slices.Delete(pathSeq, 0, 1), handler)

		n = &r.nodes[nodeIdx]
		n.children = append(n.children, childIdx)
		n.rebuildFingerprint(r)

		return
	}

	closest := &r.nodes[n.children[closestIdx]]
	if len(closest.prefix) == best {
		if best == len(pathSeq[0]) {
			pathSeq = slices.Delete(pathSeq, 0, 1)
		} else {
			pathSeq[0] = pathSeq[0][best:]
		}

		r.insert(n.children[closestIdx], method, pathSeq, handler)

		return
	}

	if len(closest.prefix) > best {
		newChildIdx := r.newNode()
		n = &r.nodes[nodeIdx]
		closest = &r.nodes[n.children[closestIdx]]

		newChild := &r.nodes[newChildIdx]
		newChild.prefix = closest.prefix[:best]
		closest.prefix = closest.prefix[best:]
		newChild.children = append(newChild.children, n.children[closestIdx])

		n.children[closestIdx] = newChildIdx
		n.rebuildFingerprint(r)

		if best >= len(currentSegment) {
			pathSeq = slices.Delete(pathSeq, 0, 1)
		} else {
			pathSeq[0] = pathSeq[0][best:]
		}

		r.insert(newChildIdx, method, pathSeq, handler)
	}
}
