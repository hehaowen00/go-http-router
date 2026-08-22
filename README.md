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
BenchmarkBuildGithubAPI
BenchmarkBuildGithubAPI-12              	  21348	    55432 ns/op	 115201 B/op	   1123 allocs/op
BenchmarkBuildGithubAPIInsertOnly
BenchmarkBuildGithubAPIInsertOnly-12    	  21876	    54913 ns/op	 115201 B/op	   1123 allocs/op
BenchmarkRouterGithub
BenchmarkRouterGithub-12                	36949509	       31.79 ns/op
BenchmarkRouterLarge
BenchmarkRouterLarge-12                 	48618098	       22.96 ns/op
BenchmarkRouterGithubParallel
BenchmarkRouterGithubParallel-12        	1423491	      841.8 ns/op	      0 B/op	      0 allocs/op
BenchmarkRouterGithubAll
BenchmarkRouterGithubAll-12             	 151797	     7545 ns/op
BenchmarkRouterLargeAll
BenchmarkRouterLargeAll-12              	  61024	    19008 ns/op
BenchmarkRouterGithubRandom
BenchmarkRouterGithubRandom-12          	 156024	     7520 ns/op
BenchmarkRouterGithubParams
BenchmarkRouterGithubParams-12          	31692825	       36.23 ns/op	      0 B/op	      0 allocs/op
BenchmarkParamMissSingle
BenchmarkParamMissSingle-12             	70695488	       16.36 ns/op	      0 B/op	      0 allocs/op
PASS
ok  	github.com/hehaowen00/go-http-router	12.969s
```
