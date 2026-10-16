BIN := bin/hashiru-tui

.PHONY: build test race fmt run

build:
	go build -o $(BIN) ./cmd/hashiru-tui

test:
	go test ./...

race:
	go test -race -cover ./...

fmt:
	gofmt -l -w .
	go vet ./...

run: build
	./$(BIN)
