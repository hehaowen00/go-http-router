# go-http-router

```
goos: darwin
goarch: arm64
pkg: github.com/hehaowen00/go-http-router
cpu: Apple M2 Max
BenchmarkRouterMiss
BenchmarkRouterMiss-12                  	1044326	     1131 ns/op
BenchmarkRouterHalfMiss
BenchmarkRouterHalfMiss-12              	 189094	     6279 ns/op
BenchmarkBuildGithubAPI
BenchmarkBuildGithubAPI-12              	  13341	    89945 ns/op	 166450 B/op	   1613 allocs/op
BenchmarkBuildGithubAPIInsertOnly
BenchmarkBuildGithubAPIInsertOnly-12    	  13329	    90141 ns/op	 166450 B/op	   1613 allocs/op
BenchmarkRouterGithub
BenchmarkRouterGithub-12                	 116432	    10398 ns/op
BenchmarkRouterGithubRandom
BenchmarkRouterGithubRandom-12          	 114249	    10355 ns/op
```
