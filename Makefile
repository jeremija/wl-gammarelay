PREFIX ?= /usr/local
BUILD_FLAGS := -ldflags "-X main.GitDescribe=$(shell git describe --always --tags --dirty)" -o peer-calls

.PHONY: all
all: protocol build

.PHONY: build
build: bin
	go build $(BUILD_FLAGS) -o bin/wl-gammarelay main.go

bin:
	mkdir -p bin/

.PHONY: test
test:
	go test ./...

.PHONY: protocol
protocol:
	$(MAKE) -C protocol/

.PHONY: clean
clean:
	$(MAKE) -C protocol/ clean
	rm -rf bin/

.PHONY: install
install:
	install bin/wl-gammarelay "$(PREFIX)/bin/wl-gammarelay"
