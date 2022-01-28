PREFIX ?= /usr/local
VERSION ?= $(shell git describe --always --tags --dirty)
COMMIT_HASH ?= $(shell git rev-parse HEAD)

BUILD_FLAGS := -ldflags "\
							 -X main.Version=$(VERSION) \
							 -X main.CommitHash=$(COMMIT_HASH)"

.PHONY: all
all: protocol build

.PHONY: build
build: bin
	go build $(BUILD_FLAGS) -o bin/wl-gammarelay ./

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
