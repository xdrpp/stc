DESTDIR =
PREFIX = /usr/local
MANDIR = $(PREFIX)/share/man

all:
	go build

install: stc stc.1
	mkdir -p $(DESTDIR)$(PREFIX)/bin
	cp stc $(DESTDIR)$(PREFIX)/bin/stc
	mkdir -p $(DESTDIR)$(MANDIR)/man1
	cp stc.1 $(DESTDIR)$(MANDIR)/man1/stc.1

uninstall:
	rm -f $(DESTDIR)$(PREFIX)/bin/stc $(DESTDIR)$(MANDIR)/man1/stc.1

clean:
	go clean
	rm -f *~ .*~ debug

maintainer-clean: clean
	rm -f stc.1

stc.1: stc.1.md
	pandoc -s -w man stc.1.md -o stc.1 || \
		git show $$(git for-each-ref --count 1 --format '%(refname)' 'refs/remotes/*/go1'):./$@ > $@

.PHONY: all install uninstall clean maintainer-clean
.NOTPARALLEL:
