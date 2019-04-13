DESTDIR =
PREFIX = /usr/local
MANDIR = $(PREFIX)/share/man

BUILT_SOURCES = stx/xdr_generated.go uhelper.go
XDRS = xdr/Stellar-SCP.x xdr/Stellar-ledger-entries.x			\
xdr/Stellar-ledger.x xdr/Stellar-overlay.x xdr/Stellar-transaction.x	\
xdr/Stellar-types.x

GO_DEPENDS = golang.org/x/crypto/... golang.org/x/tools/cmd/goyacc	\
golang.org/x/tools/cmd/stringer

all: cmd/stc/stc

always:
	@:

install uninstall:
	cd cmd/stc && $(MAKE) $@

build-depend:
	go get $(GO_DEPENDS)

update-depend:
	go get -u $(GO_DEPENDS)

xdr:
	git fetch --depth=1 https://github.com/stellar/stellar-core.git master
	git archive --prefix=xdr/ FETCH_HEAD:src/xdr | tar xf -

$(XDRS): xdr

cmd/goxdr/goxdr:
	cd cmd/goxdr && GOARCH=$$(go env GOHOSTARCH) $(MAKE)

cmd/stc/stc: $(BUILT_SOURCES) always
	cd cmd/stc && $(MAKE)

stx/xdr_generated.go: cmd/goxdr/goxdr $(XDRS)
	cmd/goxdr/goxdr -p stx -o $@~ $(XDRS)
	@if cmp $@ $@~ > /dev/null 2>/dev/null; then \
		rm -f $@~; \
	else \
		echo mv -f $@~ $@; \
		mv -f $@~ $@; \
	fi

uhelper.go: stx/xdr_generated.go uniontool/uniontool.go
	go run uniontool/uniontool.go > $@~
	mv -f $@~ $@

clean:
	for dir in cmd/goxdr cmd/stc; do \
		(cd $$dir && $(MAKE) $@); \
	done
	go clean
	rm -f *~ .*~ */*~

maintainer-clean:
	for dir in cmd/goxdr cmd/stc; do \
		(cd $$dir && $(MAKE) $@); \
	done
	go clean
	rm -f *~ .*~ */*~ go.sum $(BUILT_SOURCES)
	# Git clean avoids removing xdr if it's a git repository
	git clean -fxd xdr

go1:
	./make-go1

.PHONY: all install clean maintainer-clean go1
.PHONY: build-depend update-depend always
.PHONY: cmd/goxdr/goxdr cmd/goxdr/stc
