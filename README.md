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
BenchmarkBuildGithubAPI-12              	  19532	    60600 ns/op	 138162 B/op	   1061 allocs/op
BenchmarkBuildGithubAPIInsertOnly-12    	  19978	    60100 ns/op	 138161 B/op	   1061 allocs/op
BenchmarkRouterGithub-12                	37497753	       31.61 ns/op
BenchmarkRouterLarge-12                 	44853246	       24.97 ns/op
BenchmarkRouterGithubParallel-12        	1425466	      842.7 ns/op	      0 B/op	      0 allocs/op
BenchmarkRouterGithubAll-12             	 155446	     7562 ns/op
BenchmarkRouterLargeAll-12              	  56522	    20444 ns/op
BenchmarkRouterGithubRandom-12          	 159190	     7540 ns/op
BenchmarkRouterGithubParams-12          	32651173	       36.36 ns/op	      0 B/op	      0 allocs/op
BenchmarkParamMissSingle-12             	73130598	       16.16 ns/op	      0 B/op	      0 allocs/op
PASS
ok  	github.com/hehaowen00/go-http-router	13.112s
```
