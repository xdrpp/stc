DESTDIR =
PREFIX = /usr/local
MANDIR = $(PREFIX)/share/man

BUILT_SOURCES = header.go parse.go
EXTRA_CLEAN = y.output

all: $(BUILT_SOURCES)
	go build

install: goxdr goxdr.1
	mkdir -p $(DESTDIR)$(PREFIX)/bin
	cp goxdr $(DESTDIR)$(PREFIX)/bin/goxdr
	mkdir -p $(DESTDIR)$(MANDIR)/man1
	cp goxdr.1 $(DESTDIR)$(MANDIR)/man1/goxdr.1

uninstall:
	rm -f $(DESTDIR)$(PREFIX)/bin/goxdr $(DESTDIR)$(MANDIR)/man1/goxdr.1

$(BUILT_SOURCES): header.go.in parse.y goxdr.go
	PATH="$$PATH:$$(go env GOPATH)/bin" go generate
	sed -e 's|^[ 	]*//line|//line|' parse.go > parse.go~ \
		&& mv -f parse.go~ parse.go

clean:
	go clean
	rm -f *~ .*~ y.output debug

maintainer-clean: clean
	rm -f $(BUILT_SOURCES) goxdr.1

goxdr.1: goxdr.1.md
	pandoc -s -w man goxdr.1.md -o goxdr.1 || \
		git show $$(git for-each-ref --count 1 --format '%(refname)' 'refs/remotes/*/go1'):./$@ > $@

.PHONY: all install uninstall clean maintainer-clean
.NOTPARALLEL:
