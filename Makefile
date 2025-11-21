.PHONY: build build-linux clean

build:
	go build -o bin/linkmaster-node ./cmd/agent

build-linux:
	GOOS=linux GOARCH=amd64 go build -o bin/linkmaster-node-linux ./cmd/agent

clean:
	rm -rf bin/

