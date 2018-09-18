BUILT_SOURCES = header.go parse.go
EXTRA_CLEAN = y.output

all: $(BUILT_SOURCES)
	go build

install: goxdr goxdr.1
	mkdir -p $(DESTDIR)$(PREFIX)/bin
	cp goxdr $(DESTDIR)$(PREFIX)/bin/goxdr
	mkdir -p $(DESTDIR)$(MANDIR)/man1
	cp goxdr.1 $(DESTDIR)$(MANDIR)/man1/goxdr.1

$(BUILT_SOURCES): header.go.in parse.y syms.go goxdr.go
#	PATH="$$PATH:$$(go env GOPATH)/bin" \
#		test -n "$$(command -v goyacc)" || \
#		go get golang.org/x/tools/cmd/goyacc
#	PATH="$$PATH:$$(go env GOPATH)/bin" \
#		test -n "$$(command -v stringer)" || \
#		go get golang.org/x/tools/cmd/stringer
	PATH="$$PATH:$$(go env GOPATH)/bin" go generate
	sed -e 's|^[ 	]*//line|//line|' parse.go > parse.go~ \
		&& mv -f parse.go~ parse.go

clean:
	go clean
	rm -f *~ .*~ $(EXTRA_CLEAN)

maintainer-clean: clean
	rm -f $(BUILT_SOURCES)

.gitignore: Makefile
	@rm -f .gitignore~
	for f in '*~' $(BUILT_SOURCES) $(EXTRA_CLEAN) "`basename $$PWD`"; do \
		echo "$$f" >> .gitignore~; \
	done
	mv -f .gitignore~ .gitignore

goxdr.1: goxdr.1.md
	pandoc -s -w man goxdr.1.md -o goxdr.1 || \
		git show $$(git for-each-ref --count 1 --format '%(refname)' 'refs/remotes/*/go1'):./$@ > $@

.PHONY: all install clean maintainer-clean
.NOTPARALLEL:
