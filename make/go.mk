.PHONY: go-test
go-test: go-lint
go-test: install-tool.golang
	cd apptests && go test -v -race -covermode=atomic -coverprofile=coverage.out ./...
	cd ../hack/release && go test -v -race -covermode=atomic -coverprofile=coverage.out ./...

.PHONY: mod-tidy
mod-tidy: install-tool.golang
	cd hack/release && go mod tidy

.PHONY: go-lint
go-lint: install-tool.golang install-tool.golangci-lint
	cd hack/release && golangci-lint run ./...
