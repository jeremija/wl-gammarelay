prefix=/usr/local

.PHONY: all
all: protocol build

.PHONY: build
build: bin/wl-gammarelay.sh bin/wl-gammarelay

bin:
	mkdir -p bin/

bin/wl-gammarelay.sh: bin wl-gammarelay.sh
	cp wl-gammarelay.sh bin/wl-gammarelay.sh

bin/wl-gammarelay: bin src/wl-gammarelay.c protocol/wlr-gamma-control-unstable-v1-protocol.c
	gcc \
		-lm \
		-lwayland-client \
		-Iprotocol/ \
		protocol/wlr-gamma-control-unstable-v1-protocol.c \
		src/wl-gammarelay.c \
		-o bin/wl-gammarelay

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
