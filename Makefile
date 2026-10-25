# stream - a tiling window manager for river.
# See LICENSE file for copyright and license details.

include config.mk

export CGO_ENABLED = 0

GEN = internal/proto/gen.go internal/keysym/table.go
SRC = $(shell find cmd internal -type f -name '*.go' 2>/dev/null) ${GEN} go.mod go.sum config.mk
BIN = bin/swm bin/streamctl

all: build

build: ${BIN}

# Real file targets, so `sudo make install` after `make` only copies and
# doesn't rebuild as root.
bin/%: ${SRC}
	@mkdir -p bin
	${GO} build ${GOFLAGS} -ldflags "${LDFLAGS}" -o $@ ./cmd/$*

generate: ${GEN}

internal/proto/gen.go: internal/proto/proto.go $(wildcard protocol/*.xml)
	${GO} generate ./internal/proto

internal/keysym/table.go: internal/keysym/gen/main.go ${KEYSYMS}
	${GO} run ./internal/keysym/gen -o $@ ${KEYSYMS}

# Runs a development build of swm in river: nested in a window when started
# from a Wayland session, on the hardware when started from a TTY.
run: ${GEN}
	@mkdir -p bin/dev
	${GO} build -o bin/dev/swm ./cmd/swm
	${RIVER} -c '${CURDIR}/bin/dev/swm -log-level debug'

test: ${GEN}
	${GO} test -v ./...

# examples/h2o/wallpaper is a module of its own, so ./... leaves it out.
EXAMPLES = examples/h2o/wallpaper

fmt:
	${GO} fmt ./...
	${GO} -C ${EXAMPLES} fmt ./...

# staticcheck skips internal/proto: it is wlclgen output (SA4006 on unused fds
# values in the generator's templates), not code maintained here.
lint: ${GEN}
	${GO} vet ./...
	${GO} tool staticcheck $$(${GO} list ./... | grep -v '/internal/proto$$')
	${GO} -C ${EXAMPLES} vet ./...
	sc=$$(${GO} tool -n staticcheck) && cd ${EXAMPLES} && $$sc ./...

clean:
	rm -rf bin ${GEN}

install: ${BIN}
	@mkdir -p ${DESTDIR}${PREFIX}/bin
	cp -f ${BIN} ${DESTDIR}${PREFIX}/bin
	@chmod 755 ${DESTDIR}${PREFIX}/bin/swm ${DESTDIR}${PREFIX}/bin/streamctl
	@mkdir -p ${DESTDIR}${PREFIX}/share/backgrounds/stream
	cp -f examples/h2o/wallpaper.png ${DESTDIR}${PREFIX}/share/backgrounds/stream/h2o.png
	@chmod 644 ${DESTDIR}${PREFIX}/share/backgrounds/stream/h2o.png

uninstall:
	rm -f ${DESTDIR}${PREFIX}/bin/swm ${DESTDIR}${PREFIX}/bin/streamctl
	rm -f ${DESTDIR}${PREFIX}/share/backgrounds/stream/h2o.png
	-rmdir ${DESTDIR}${PREFIX}/share/backgrounds/stream

.PHONY: all build generate run test fmt lint clean install uninstall
