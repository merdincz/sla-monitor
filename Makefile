.PHONY: build test lint clean docker

build:
	go build -o bin/sla-monitor ./cmd/sla-monitor

test:
	go test -v -cover ./...

lint:
	golangci-lint run

clean:
	rm -rf bin/

docker:
	docker build -t sla-monitor:latest .

release:
	goreleaser release --rm-dist