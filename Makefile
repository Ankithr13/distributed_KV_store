.PHONY: tidy build

tidy:
	go mod tidy

build:
	go build ./...

run1:
	go run ./cmd/node -id n1 -addr 127.0.0.1:8001 -peers 127.0.0.1:8002,127.0.0.1:8003 -data ./data/n1

run2:
	go run ./cmd/node -id n2 -addr 127.0.0.1:8002 -peers 127.0.0.1:8001,127.0.0.1:8003 -data ./data/n2

run3:
	go run ./cmd/node -id n3 -addr 127.0.0.1:8003 -peers 127.0.0.1:8001,127.0.0.1:8002 -data ./data/n3