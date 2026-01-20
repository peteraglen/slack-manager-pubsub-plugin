init: modules

modules:
	go mod tidy

test:
	gosec ./...
	go fmt ./...
	go test  -timeout 5s -cover -race ./...
	go vet ./...

lint:
	golangci-lint run ./...

lint-fix:
	golangci-lint run --fix ./...

bump-common-lib:
	go get github.com/peteraglen/slack-manager-common@latest
	go mod tidy
