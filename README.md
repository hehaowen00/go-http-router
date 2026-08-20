# go-http-router

## Benchmarks

Github benchmark taken from `go-http-routing-benchmark`.

Large is a 901 route production API with mostly GET and POST static routes with
parameters.

```
goos: darwin
goarch: arm64
pkg: github.com/hehaowen00/go-http-router
cpu: Apple M2 Max
BenchmarkBuildGithubAPI-12              	  19153	    61266 ns/op	 138161 B/op	   1061 allocs/op
BenchmarkBuildGithubAPIInsertOnly-12    	  19953	    60128 ns/op	 138161 B/op	   1061 allocs/op
BenchmarkRouterGithub-12                	37148827	       31.40 ns/op
BenchmarkRouterLarge-12                 	45800360	       24.75 ns/op
BenchmarkRouterGithubParallel-12        	1437668	      840.1 ns/op	      0 B/op	      0 allocs/op
BenchmarkRouterGithubAll-12             	 156174	     7558 ns/op
BenchmarkRouterLargeAll-12              	  57973	    19873 ns/op
BenchmarkRouterGithubRandom-12          	 157268	     7478 ns/op
BenchmarkRouterGithubParams-12          	32888162	       36.48 ns/op	      0 B/op	      0 allocs/op
BenchmarkParamMissSingle-12             	74055404	       16.09 ns/op	      0 B/op	      0 allocs/op
PASS
ok  	github.com/hehaowen00/go-http-router	13.166s
```
