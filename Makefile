.PHONY: build test lint fmt ci devdeps clean e2e-up e2e-down e2e-test e2e
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

# E2Eテスト用のターゲット
e2e-up:
	@echo ">> Starting database containers"
	docker compose up -d
	@echo ">> Waiting for databases to be ready..."
	@sleep 10

e2e-down:
	@echo ">> Stopping database containers"
	docker compose down -v

e2e-test:
	@echo ">> Running E2E tests"
	MYSQL_DSN="root:root@tcp(localhost:3306)/opsql_test?parseTime=true" \
	POSTGRES_DSN="postgres://postgres:postgres@localhost:5432/opsql_test?sslmode=disable" \
	go test -v ./e2e/...

e2e: e2e-up e2e-test e2e-down
	@echo ">> E2E tests completed"
