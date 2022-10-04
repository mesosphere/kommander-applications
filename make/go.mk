.PHONY: go-test
go-test: go-lint
go-test: install-tool.golang install-tool.go.goveralls
	cd hack/release && go test -v -race -covermode=atomic -coverprofile=coverage.out ./...
	cd hack/release && goveralls -coverprofile=coverage.out -service=github

.PHONY: mod-tidy
mod-tidy: install-tool.golang
	cd hack/release && go mod tidy

.PHONY: go-lint
go-lint: install-tool.golang install-tool.golangci-lint
	cd hack/release && golangci-lint run ./...
