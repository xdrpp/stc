DESTDIR =
PREFIX = /usr/local
MANDIR = $(PREFIX)/share/man

BUILT_SOURCES = xdr_generated.go
XDRS = xdr/Stellar-SCP.x xdr/Stellar-ledger-entries.x			\
xdr/Stellar-ledger.x xdr/Stellar-overlay.x xdr/Stellar-transaction.x	\
xdr/Stellar-types.x

GO_DEPENDS = golang.org/x/crypto/... golang.org/x/tools/cmd/goyacc	\
golang.org/x/tools/cmd/stringer

all: $(BUILT_SOURCES) cmd/stc/stc

install: cmd/stc/stc cmd/stc/stc.1
	mkdir -p $(DESTDIR)$(PREFIX)/bin
	cp cmd/stc/stc $(DESTDIR)$(PREFIX)/bin/stc
	mkdir -p $(DESTDIR)$(MANDIR)/man1
	cp cmd/stc/stc.1 $(DESTDIR)$(MANDIR)/man1/stc.1

uninstall:
	rm -f $(DESTDIR)$(PREFIX)/bin/stc $(DESTDIR)$(MANDIR)/man1/stc.1

build-depend:
	go get $(GO_DEPENDS)

update-depend:
	go get -u $(GO_DEPENDS)

$(XDRS):
	git fetch --depth=1 https://github.com/stellar/stellar-core.git master
	git archive --prefix=xdr/ FETCH_HEAD:src/xdr | tar xf -

cmd/goxdr/goxdr:
	GOARCH=$$(go env GOHOSTARCH) $(MAKE) -C cmd/goxdr

cmd/stc/stc: $(BUILT_SOURCES)
	cd cmd/stc && go build

xdr_generated.go: cmd/goxdr/goxdr $(XDRS)
	cmd/goxdr/goxdr -p stc -o $@ $(XDRS)

clean:
	$(MAKE) -C cmd/goxdr $@
	go clean
	cd cmd/stc && go clean
	rm -f *~ .*~ cmd/stc/*~ cmd/stc/.*~

maintainer-clean: clean
	$(MAKE) -C cmd/goxdr $@
	rm -rf go.sum $(BUILT_SOURCES) xdr cmd/stc/stc.1

cmd/stc/stc.1: cmd/stc/stc.1.md
	pandoc -s -w man cmd/stc/stc.1.md -o cmd/stc/stc.1 || \
		git show $$(git for-each-ref --count 1 --format '%(refname)' 'refs/remotes/*/go1'):./$@ > $@

go1:
	./make-go1

.PHONY: all install clean maintainer-clean go1
.PHONY: build-depend update-depend goxdr/goxdr
