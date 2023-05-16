BIN := memcached-mysql

.PHONY: all
all: $(BIN)

.PHONY: test
test:
	go test ./... -cover -race -v

$(BIN): *.go **/*.go
	go build -o $(BIN) .

.PHONY: clean
clean:
	$(RM) $(BIN)
