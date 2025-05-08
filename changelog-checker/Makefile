.PHONY: build install test clean

build:
	go build -o bin/changelog-checker cmd/main.go

install:
	go install ./cmd/changelog-checker/

test:
	go test ./...

clean:
	rm -rf bin
