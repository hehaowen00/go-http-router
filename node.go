package gohttprouter

import (
	"fmt"
	"slices"
	"strings"
)

type node struct {
	prefix   string
	handlers methodHandler
	children []*node
	wildcard []*wildcard
	param    string
}

type wildcard struct {
	name string
	node *node
}

func newNode() *node {
	return &node{}
}

func (n *node) getWildcard(name string) int {
	for i := range n.wildcard {
		if n.wildcard[i].name == name {
			return i
		}
	}

	n.wildcard = append(n.wildcard, &wildcard{
		name: name,
		node: newNode(),
	})

	return len(n.wildcard) - 1
}

func (n *node) search(method string, path string, params *Params) Handler {
	path = strings.TrimPrefix(path, n.prefix)

	if len(path) == 0 || (n.handlers.count() > 0 && path == "/") {
		return n.handlers.getHandler(method)
	}

	for _, child := range n.children {
		if strings.HasPrefix(path, child.prefix) {
			return child.search(method, path, params)
		}
	}

	if len(n.wildcard) == 0 {
		return nil
	}

	idx := strings.IndexAny(path[1:], "/")
	if idx == -1 {
		idx = len(path)
	}

	value := path[:idx]

	for _, v := range n.wildcard {
		params.set(v.name, value)
		path = path[idx:]

		idx := strings.IndexAny(path, "/")
		if idx == -1 {
			idx = len(path)
		}

		path = path[idx:]
		return v.node.search(method, path, params)
	}

	return nil
}

func (n *node) insert(method string, pathSeq []string, handler Handler) {
	if len(pathSeq) == 0 {
		n.handlers.insertHandler(method, handler)
		return
	}

	currentSegment := pathSeq[0]

	if isParam(currentSegment) {
		name := paramName(currentSegment)

		i := n.getWildcard(name)
		wildcardNode := n.wildcard[i]

		// if n.wildcard == nil {
		// 	n.wildcard = map[string]*node{}
		// }

		// wildcardNode, ok := n.wildcard[name]
		// if !ok {
		// 	wildcardNode = newNode()
		// 	n.wildcard[name] = wildcardNode
		// }

		pathSeq = slices.Delete(pathSeq, 0, 1)
		wildcardNode.node.insert(method, pathSeq, handler)
		return
	}

	shortestIdx := -1
	best := 0

	for i := range n.children {
		score := longestMatch(currentSegment, n.children[i].prefix)
		if score > best {
			best = score
			shortestIdx = i
		}
	}

	if shortestIdx < 0 {
		pathSeq = slices.Delete(pathSeq, 0, 1)

		childNode := newNode()
		childNode.prefix = currentSegment
		childNode.insert(method, pathSeq, handler)

		n.children = append(n.children, childNode)
		return
	}

	closest := n.children[shortestIdx]
	if len(closest.prefix) == best {
		if best == len(pathSeq[0]) {
			pathSeq = slices.Delete(pathSeq, 0, 1)
		} else {
			pathSeq[0] = pathSeq[0][best:]
		}

		closest.insert(method, pathSeq, handler)
	}

	if len(closest.prefix) > best {
		newChild := newNode()
		newChild.prefix = closest.prefix[:best]
		closest.prefix = closest.prefix[best:]
		newChild.children = append(newChild.children, closest)

		n.children[shortestIdx] = newChild

		if best >= len(currentSegment) {
			pathSeq = slices.Delete(pathSeq, 0, 1)
		} else {
			pathSeq[0] = pathSeq[0][best:]
		}

		newChild.insert(method, pathSeq, handler)
	}
}

func isParam(segment string) bool {
	return strings.HasPrefix(segment, ":")
}

func paramName(segment string) string {
	return strings.TrimPrefix(segment, ":")
}

func longestMatch(left, right string) int {
	var i int

	l := min(len(left), len(right))

	for i = range l {
		if left[i] != right[i] {
			return i
		}
	}

	return i + 1
}

func splitPath(path string) []string {
	path = strings.TrimSpace(path)

	for strings.HasPrefix(path, "/") {
		path = strings.TrimPrefix(path, "/")
	}

	for strings.HasSuffix(path, "/") {
		path = strings.TrimSuffix(path, "/")
	}

	path = strings.ReplaceAll(path, "//", "/")

	xs := strings.Split(path, "/")

	var buf string
	var res []string

	for _, v := range xs {
		if strings.HasPrefix(v, ":") {
			if len(buf) > 0 {
				res = append(res, buf+"/")
				buf = ""
				res = append(res, v)
			} else {
				res = append(res, v)
			}
		} else {
			// if buf == "" {
			// 	buf = v
			// } else {
			buf += "/" + v
			// }
		}
	}

	if buf != "" {
		res = append(res, buf+"/")
	}

	return res
}

func validateSeq(xs []string) error {
	set := map[string]struct{}{}

	for i := range xs {
		k := xs[i]
		if k[0] != ':' {
			continue
		}

		_, ok := set[k]
		if ok {
			return fmt.Errorf("duplicate param name - %s", k)
		}
		set[k] = struct{}{}
	}

	if len(set) > 32 {
		return fmt.Errorf("wildcard limit exceeded")
	}

	return nil
}
