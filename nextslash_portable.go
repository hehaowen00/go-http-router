//go:build !amd64 && !arm64

package gohttprouter

func nextSlash(s string, i, end int) int {
	for i < end && s[i] != '/' {
		i++
	}

	return i
}
