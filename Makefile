DESTDIR =
PREFIX = /usr/local
MANDIR = $(PREFIX)/share/man

BUILT_SOURCES = xdr_generated.go
EXTRA_CLEAN =
XDRS = xdr/Stellar-SCP.x xdr/Stellar-ledger-entries.x			\
xdr/Stellar-ledger.x xdr/Stellar-overlay.x xdr/Stellar-transaction.x	\
xdr/Stellar-types.x

GO_DEPENDS = golang.org/x/crypto/... golang.org/x/tools/cmd/goyacc	\
golang.org/x/tools/cmd/stringer

all: $(BUILT_SOURCES)
	go build -o stc

install: stc stc.1
	mkdir -p $(DESTDIR)$(PREFIX)/bin
	cp stc $(DESTDIR)$(PREFIX)/bin/stc
	mkdir -p $(DESTDIR)$(MANDIR)/man1
	cp stc.1 $(DESTDIR)$(MANDIR)/man1/stc.1

build-depend:
	go get $(GO_DEPENDS)

update-depend:
	go get -u $(GO_DEPENDS)

$(XDRS):
	git fetch --depth=1 https://github.com/stellar/stellar-core.git master
	git archive --prefix=xdr/ FETCH_HEAD:src/xdr | tar xf -

goxdr/goxdr:
	GOARCH=$$(go env GOHOSTARCH) $(MAKE) -C goxdr

xdr_generated.go: goxdr/goxdr $(XDRS)
	goxdr/goxdr -o $@ $(XDRS)

clean:
	$(MAKE) -C goxdr $@
	go clean
	rm -f *~ .*~ $(EXTRA_CLEAN)

maintainer-clean: clean
	$(MAKE) -C goxdr $@
	rm -rf $(BUILT_SOURCES) xdr

.gitignore: Makefile
	@rm -f .gitignore~
	for f in '*~' $(BUILT_SOURCES) $(EXTRA_CLEAN) "`basename $$PWD`"; do \
		echo "$$f" >> .gitignore~; \
	done
	echo xdr >> .gitignore~
	mv -f .gitignore~ .gitignore

stc.1: stc.1.md
	pandoc -s -w man stc.1.md -o stc.1

go1:
	./make-go1

.PHONY: all install clean maintainer-clean go1
.PHONY: build-depend update-depend goxdr/goxdr
