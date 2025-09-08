node1:
	export NODE_ID=1
	export HTTP_BIND=:9001
	go run main.go -http-bind=:9001 -peers=http://127.0.0.1:9002,http://127.0.0.1:9003

node2:
	export NODE_ID=2
	export HTTP_BIND=:9002
	go run main.go -http-bind=:9002 -peers=http://127.0.0.1:9001,http://127.0.0.1:9003

node3:
	export NODE_ID=3
	export HTTP_BIND=:9003
	go run main.go -http-bind=:9003 -peers=http://127.0.0.1:9001,http://127.0.0.1:9002