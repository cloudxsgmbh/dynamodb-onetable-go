.PHONY: test lint fmt vet tidy clean

## test: run all tests
test:
	go test ./...

## lint: run golangci-lint
lint:
	~/go/bin/golangci-lint run ./...

## fmt: format all Go source
fmt:
	gofmt -w .

## vet: run go vet
vet:
	go vet ./...

## tidy: tidy go modules
tidy:
	go mod tidy

## clean: remove build artefacts
clean:
	rm -rf ./bin