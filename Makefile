.PHONY: build test vet lambda clean run

CONFIG ?= config.yaml

build:
	@go build -trimpath -o bin/discovery ./cmd/discovery

test:
	@go test -race -coverprofile=coverage.out ./...

vet:
	@go vet ./...

lambda:
	@mkdir -p build; \
	 CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -trimpath -ldflags='-s -w' -o build/bootstrap ./cmd/discovery;

clean:
	@rm -rf bin build coverage.out diagrams

run:
	@go run ./cmd/discovery -config "$(CONFIG)"
