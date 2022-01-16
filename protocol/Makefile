SRCS := $(wildcard *.xml)

all: \
	$(patsubst %.xml,%-client-protocol.h,$(SRCS)) \
	$(patsubst %.xml,%-protocol.h,$(SRCS)) \
	$(patsubst %.xml,%-protocol.c,$(SRCS)) \

%-protocol.c:
	wayland-scanner private-code "$*.xml" "$@"

%-protocol.h:
	wayland-scanner server-header "$*.xml" "$@"

%-client-protocol.h:
	wayland-scanner client-header "$*.xml" "$@"

%.o:
	@echo test: $@

.PHONY: clean
clean:
	rm -f *.h *.c

# Disable implicit rules.
.SUFFIXES:
