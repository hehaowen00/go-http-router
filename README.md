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
BenchmarkBuildGithubAPI-12              	  19292	    60906 ns/op	 138161 B/op	   1061 allocs/op
BenchmarkBuildGithubAPIInsertOnly-12    	  20091	    59668 ns/op	 138161 B/op	   1061 allocs/op
BenchmarkRouterGithub-12                	37040942	       31.97 ns/op
BenchmarkRouterLarge-12                 	44972312	       24.60 ns/op
BenchmarkRouterGithubParallel-12        	1382958	      869.1 ns/op	      0 B/op	      0 allocs/op
BenchmarkRouterGithubAll-12             	 154622	     7641 ns/op
BenchmarkRouterLargeAll-12              	  57463	    20172 ns/op
BenchmarkRouterGithubRandom-12          	 157362	     7690 ns/op
BenchmarkRouterGithubParams-12          	31949661	       37.09 ns/op	      0 B/op	      0 allocs/op
BenchmarkParamMissSingle-12             	75809426	       15.99 ns/op	      0 B/op	      0 allocs/op
PASS
ok  	github.com/hehaowen00/go-http-router	12.947s
```
