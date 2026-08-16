# go-http-router

## Benchmarks

```
goos: darwin
goarch: arm64
pkg: github.com/hehaowen00/go-http-router
cpu: Apple M2 Max
BenchmarkBuildGithubAPI
BenchmarkBuildGithubAPI-12              	   37561	     61717 ns/op	  138161 B/op	    1061 allocs/op
BenchmarkBuildGithubAPIInsertOnly
BenchmarkBuildGithubAPIInsertOnly-12    	   38223	     63995 ns/op	  138161 B/op	    1061 allocs/op
BenchmarkRouterGithub
BenchmarkRouterGithub-12                	71439554	        33.64 ns/op
BenchmarkRouterGithubParallel
BenchmarkRouterGithubParallel-12        	 2652810	       912.9 ns/op	       0 B/op	       0 allocs/op
BenchmarkRouterGithubAll
BenchmarkRouterGithubAll-12             	  297973	      7992 ns/op
BenchmarkRouterGithubRandom
BenchmarkRouterGithubRandom-12          	  300012	      7949 ns/op
BenchmarkRouterGithubParams
BenchmarkRouterGithubParams-12          	61996814	        38.42 ns/op	       0 B/op	       0 allocs/op
BenchmarkParamMissSingle
BenchmarkParamMissSingle-12             	142608814	        16.78 ns/op	       0 B/op	       0 allocs/op
```
