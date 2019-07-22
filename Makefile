CMDS = stc
CLEANFILES = .*~ *~ */*~ goxdr
BUILT_SOURCES = stx/xdr_generated.go uhelper.go stcdetail/stcxdr.go
XDRS = xdr/Stellar-SCP.x xdr/Stellar-ledger-entries.x			\
xdr/Stellar-ledger.x xdr/Stellar-overlay.x xdr/Stellar-transaction.x	\
xdr/Stellar-types.x

all: build man

build: $(BUILT_SOURCES) always
	go build

stx/xdr_generated.go: goxdr $(XDRS)
	./goxdr -B -p stx -enum-comments -o $@~ $(XDRS)
	cmp $@~ $@ 2> /dev/null || mv -f $@~ $@

stcdetail/stcxdr.go: goxdr stcdetail/stcxdr.x
	./goxdr -b -i github.com/xdrpp/stc/stx -p stcdetail -o $@~ \
		stcdetail/stcxdr.x
	cmp $@~ $@ 2> /dev/null || mv -f $@~ $@

uhelper.go: stx/xdr_generated.go uniontool/uniontool.go
	go run uniontool/uniontool.go > $@~
	mv -f $@~ $@

$(XDRS): xdr

xdr:
	git fetch --depth=1 https://github.com/stellar/stellar-core.git master
	git archive --prefix=xdr/ FETCH_HEAD:src/xdr | tar xf -

goxdr: always
	@set -e; if test -d cmd/goxdr; then \
	    (set -x; cd cmd/goxdr && $(MAKE)); \
	    goxdr=cmd/goxdr/goxdr; \
	else \
	    goxdr=$$(set -x; PATH="$$PATH:$$(go env GOPATH)/bin" command -v goxdr); \
	fi; \
	cmp "$$goxdr" $@ 2> /dev/null || set -x; cp "$$goxdr" $@

RECURSE = @set -e; for dir in $(CMDS); do \
	if test -d cmd/$$dir; then (set -x; cd cmd/$$dir && $(MAKE) $@); fi; \
	done

test: always
	go test -v . ./stcdetail
	$(RECURSE)

clean: always
	rm -f $(CLEANFILES)
	rm -rf goroot gh-pages
	$(RECURSE)

maintainer-clean: always
	rm -f $(CLEANFILES) $(BUILT_SOURCES) go.sum
	rm -rf goroot gh-pages
	$(RECURSE)

install uninstall man: always
	$(RECURSE)

built_sources: $(BUILT_SOURCES)
	rm -f $@
	for file in $(BUILT_SOURCES); do \
		echo $$file >> $@; \
	done
	$(RECURSE)

depend: always
	go get -u

go1: always
	./make-go1

gh-pages: always
	./make-gh-pages

always:
	@:
.PHONY: always
