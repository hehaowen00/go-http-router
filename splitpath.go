package gohttprouter

import "strings"

func splitPath(path string) []string {
	return splitPathInto(make([]string, 0, 4), path)
}

// splitPathInto appends path's segments to dst, so a caller can pass a stack
// buffer: insert keeps the strings but never the slice.
func splitPathInto(dst []string, path string) []string {
	if len(path) == 0 {
		return append(dst, "/")
	}

	if res, ok := splitPathFast(dst, path); ok {
		return res
	}

	return splitPathSlow(dst, path)
}

func normalizeStaticPath(path string) string {
	if len(path) == 0 {
		return "/"
	}

	if path[0] == '/' {
		i := 1
		for i < len(path) && path[i] == '/' {
			i++
		}

		end := len(path)
		for end > i && path[end-1] == '/' {
			end--
		}

		if !strings.Contains(path[i:end], "//") {
			if i == end {
				return "/"
			}

			// With a single leading '/', "/" + path[i:end] is path[:end].
			if i == 1 {
				return path[:end]
			}

			return "/" + path[i:end]
		}
	}

	path = strings.TrimSpace(path)
	if len(path) == 0 {
		return "/"
	}

	if path[0] != '/' {
		path = "/" + path
	}

	if strings.Contains(path, "//") {
		path = collapseSlashes(path)
	}

	i := 1
	end := len(path)

	for end > i && path[end-1] == '/' {
		end--
	}

	if i == end {
		return "/"
	}

	return "/" + path[i:end]
}

func splitPathFast(dst []string, path string) ([]string, bool) {
	if path[0] != '/' {
		return nil, false
	}

	i := 1
	for i < len(path) && path[i] == '/' {
		i++
	}

	end := len(path)
	for end > i && path[end-1] == '/' {
		end--
	}

	if i == end {
		return append(dst, "/"), true
	}

	res := dst
	staticStart := -1

	for i < end {
		if path[i] == '/' {
			if i+1 < end && path[i+1] == '/' {
				return nil, false
			}

			i++
			continue
		}

		segStart := i
		i = nextSlash(path, i, end)
		seg := path[segStart:i]

		if isParam(seg) || isCatchAll(seg) {
			if staticStart >= 0 {
				res = append(res, path[staticStart:segStart])
				staticStart = -1
			}

			res = append(res, seg)
			continue
		}

		if staticStart < 0 {
			staticStart = segStart - 1
		}
	}

	if staticStart >= 0 {
		res = append(res, path[staticStart:end]+"/")
	}

	return res, true
}

func splitPathSlow(dst []string, path string) []string {
	path = strings.TrimSpace(path)

	if len(path) == 0 {
		return append(dst, "/")
	}

	if path[0] != '/' {
		path = "/" + path
	}

	if strings.Contains(path, "//") {
		path = collapseSlashes(path)
	}

	i := 1
	end := len(path)

	for end > i && path[end-1] == '/' {
		end--
	}

	if i == end {
		return append(dst, "/")
	}

	res := dst
	staticStart := -1

	for i < end {
		if path[i] == '/' {
			i++
			continue
		}

		segStart := i
		i = nextSlash(path, i, end)
		seg := path[segStart:i]

		if isParam(seg) || isCatchAll(seg) {
			if staticStart >= 0 {
				res = append(res, path[staticStart:segStart])
				staticStart = -1
			}

			res = append(res, seg)
			continue
		}

		if staticStart < 0 {
			staticStart = segStart - 1
		}
	}

	if staticStart >= 0 {
		res = append(res, path[staticStart:end]+"/")
	}

	return res
}

func collapseSlashes(s string) string {
	b := make([]byte, 0, len(s))

	last := false

	for i := 0; i < len(s); i++ {
		if s[i] == '/' {
			if last {
				continue
			}

			last = true
		} else {
			last = false
		}

		b = append(b, s[i])
	}

	return string(b)
}
