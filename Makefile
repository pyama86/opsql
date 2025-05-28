.PHONY: build test lint fmt ci devdeps clean
LINTER := golangci-lint
BINARY_NAME := opsql

build:
	go build -o bin/$(BINARY_NAME) .

test:
	go test -v ./...

run:
	go run .

lint:
	@echo ">> Running linter ($(LINTER))"
	$(LINTER) run

fmt:
	@echo ">> Formatting code"
	gofmt -w .
	goimports -w .

ci: devdeps lint test build

devdeps:
	@echo ">> Installing development dependencies"
	which goimports > /dev/null || go install golang.org/x/tools/cmd/goimports@latest
	which golangci-lint > /dev/null || go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest

clean:
	rm -rf bin/
