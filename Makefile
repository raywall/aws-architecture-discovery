.PHONY: build test vet lambda clean
build:
	go build -trimpath -o bin/discovery ./cmd/discovery
test:
	go test -race -coverprofile=coverage.out ./...
vet:
	go vet ./...
lambda:
	mkdir -p build
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -trimpath -ldflags='-s -w' -o build/bootstrap ./cmd/discovery
clean:
	rm -rf bin build coverage.out
