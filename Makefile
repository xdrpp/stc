BUILT_SOURCES = xdr_generated.go
EXTRA_CLEAN =
XDRS = xdr/Stellar-SCP.x xdr/Stellar-ledger-entries.x			\
xdr/Stellar-ledger.x xdr/Stellar-overlay.x xdr/Stellar-transaction.x	\
xdr/Stellar-types.x

GO_DEPENDS = golang.org/x/crypto/... golang.org/x/tools/cmd/goyacc	\
golang.org/x/tools/cmd/stringer

all: $(BUILT_SOURCES)
	go build -o stc

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
	goxdr/goxdr -o $@~ $(XDRS)
	mv -f $@~ $@

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

.PHONY: all build-depend update-depend clean maintainer-clean goxdr/goxdr
