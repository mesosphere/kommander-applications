set dotenv-load

mod-tidy:
    cd hack/release && go mod tidy
    cd apptests && go mod tidy
    cd apptests/appscenarios && go mod tidy

go-lint:
    cd hack/release && golangci-lint run --fix ./...
    cd hack/release && go fmt ./...
    cd hack/release && go fix ./...

go-test:
    cd hack/release && go test -v -race -covermode=atomic -coverprofile=coverage.out ./...

import 'just/test.just'
import 'just/tools.just'
import 'just/release.just'
import 'just/git-operator.just'
