.PHONY: lint

lint:
	go mod tidy
	gofmt -w .
	goimports -w .

.PHONY: lint
build:
	go build -o shallbot cmd/runbot/main.go
