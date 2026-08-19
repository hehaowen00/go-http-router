bench:
	go test -v -bench=. -cpuprofile=cpu.out

web:
	go tool pprof -http=:8080 cpu.out
