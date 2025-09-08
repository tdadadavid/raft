bench:
	go run ./latency/*.go -url http://127.0.0.1:9002 -conns 50 -ops 2000 -writepct 90 -collect-latency

raft:
	go run ../bench/latency/*.go \
  -url http://127.0.0.1:8001 \
  -conns 200 \
  -ops 2000 \
  -writepct 90 \
  -collect-latency

staleness:
	go run ./stale/*.go -write-url http://127.0.0.1:8001 -read-url http://127.0.0.1:8002 -ops 2000



# go run ../bench/latency/*.go \
#   -url http://127.0.0.1:8001 \
#   -conns 200 \
#   -ops 2000 \
#   -writepct 50 \
#   -collect-latency