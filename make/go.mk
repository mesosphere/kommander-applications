.PHONY: go-test
go-test: go-lint
	cd hack/release && go test -v -race -covermode=atomic -coverprofile=coverage.out ./...

.PHONY: mod-tidy
mod-tidy:
	cd hack/release && go mod tidy
	cd magefiles && go mod tidy
	cd apptests && go mod tidy

.PHONY: go-lint
go-lint:
	cd hack/release && golangci-lint run ./...
