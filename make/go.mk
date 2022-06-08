.PHONY: go-test
go-test: install-tool.golang
	cd hack/release && go test ./...


.PHONY: mod-tidy
mod-tidy: install-tool.golang
	cd hack/release && go mod tidy
