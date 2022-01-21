PREFIX ?= /usr/local

.PHONY: all
all: protocol build

.PHONY: build
build: bin
	go build -o bin/wl-gammarelay main.go

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
