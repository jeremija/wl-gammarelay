prefix=/usr/local

all: protocol build

build: bin
	gcc \
		-lm \
		-lwayland-client \
		-Iprotocol/ \
		protocol/wlr-gamma-control-unstable-v1-protocol.c \
		src/wl-gammarelay.c \
		-o bin/wl-gammarelay

bin:
	mkdir -p bin/

.PHONY: protocol
protocol:
	$(MAKE) -C protocol/

.PHONY: clean
clean:
	$(MAKE) -C protocol/ clean
	rm -rf bin/

.PHONY: install
install:
	install bin/wl-gammarelay wl-gammarelay.sh $(prefix)/bin
