CMDS = stc
CLEANFILES = .*~ *~ */*~ goxdr
BUILT_SOURCES = stx/xdr_generated.go uhelper.go
XDRS = xdr/Stellar-SCP.x xdr/Stellar-ledger-entries.x			\
xdr/Stellar-ledger.x xdr/Stellar-overlay.x xdr/Stellar-transaction.x	\
xdr/Stellar-types.x

all: build man

build: $(BUILT_SOURCES) always go.mod
	go build
	cd cmd/stc && $(MAKE)

stx/xdr_generated.go: goxdr $(XDRS)
	./goxdr -p stx -enum-comments -o $@~ $(XDRS)
	printf "\nvar StellarCommit = \"%s\"\n" \
	    `cat xdr/Stellar-version` >> $@~
	cmp $@~ $@ 2> /dev/null || mv -f $@~ $@

uhelper.go: stx/xdr_generated.go uniontool/uniontool.go go.mod
	go run uniontool/uniontool.go > $@~
	mv -f $@~ $@

go.mod: $(MAKEFILE_LIST)
	echo 'module github.com/xdrpp/stc' > go.mod
	if test -d cmd/goxdr -a 1 != "$$BUILDING_GO1"; then \
	    echo 'replace github.com/xdrpp/goxdr => ./cmd/goxdr' >> go.mod; \
	else \
	    export GOPRIVATE='*'; \
	    go get -u github.com/xdrpp/goxdr/cmd/goxdr; \
	fi
	go mod tidy

$(XDRS): xdr

xdr:
	git fetch --depth=1 https://github.com/stellar/stellar-core.git master
	rm -f xdr/Stellar-version
	git archive --prefix=xdr/ FETCH_HEAD:src/xdr | tar xf -
	git rev-parse FETCH_HEAD > xdr/Stellar-version

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
	cd cmd/ini && $(MAKE)
	go test -v . ./stcdetail ./ini
	$(RECURSE)

clean: always
	rm -f $(CLEANFILES)
	rm -rf goroot gh-pages
	$(RECURSE)

maintainer-clean: always
	rm -f $(CLEANFILES) $(BUILT_SOURCES) go.sum go.mod
	git clean -fx xdr
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
	rm -f go.mod
	$(MAKE) go.mod

go1: always
	rm -f go.sum go.mod
	BUILDING_GO1=1 $(MAKE) build
	./make-go1
	rm -f go.mod

gh-pages: always
	./make-gh-pages

always:
	@:
.PHONY: always
