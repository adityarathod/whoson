BIN := bin

.PHONY: all clean

all: $(BIN)/whoson-cli $(BIN)/whoson-server

$(BIN):
	mkdir -p $(BIN)

$(BIN)/whoson-cli: $(BIN) $(shell find cmd/cli -name '*.go')
	go build -o $@ ./cmd/cli

$(BIN)/whoson-server: $(BIN) $(shell find cmd/server -name '*.go') $(shell find cmd/server/templates -name '*.html')
	go build -o $@ ./cmd/server

clean:
	rm -rf $(BIN)
