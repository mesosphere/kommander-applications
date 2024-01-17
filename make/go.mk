.PHONY: go-test
go-test: go-lint
	devbox run -- "cd hack/release && go test -v -race -covermode=atomic -coverprofile=coverage.out ./..."

.PHONY: mod-tidy
mod-tidy: install-tool.golang
	devbox run -- "cd hack/release && go mod tidy"

.PHONY: go-lint
go-lint:
	devbox run -- "cd hack/release && golangci-lint run ./..."
