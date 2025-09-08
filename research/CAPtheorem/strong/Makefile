node1: 
	go run *.go -id=1 -http-bind=:8001 -peers=http://127.0.0.1:8002,http://127.0.0.1:8003,http://127.0.0.1:8004
node2:
	go run *.go -id=2 -http-bind=:8002 -peers=http://127.0.0.1:8001,http://127.0.0.1:8003,http://127.0.0.1:8004

node3:
	go run *.go -id=3 -http-bind=:8003 -peers=http://127.0.0.1:8001,http://127.0.0.1:8002,http://127.0.0.1:8004

node4:
	go run *.go -id=4 -http-bind=:8004 -peers=http://127.0.0.1:8001,http://127.0.0.1:8002,http://127.0.0.1:8003