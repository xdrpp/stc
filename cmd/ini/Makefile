DESTDIR =
PREFIX = /usr/local
MANDIR = $(PREFIX)/share/man

BUILT_SOURCES =
EXTRA_CLEAN =

all: $(BUILT_SOURCES)
	go build -tags ini

install: goxdr goxdr.1
	mkdir -p $(DESTDIR)$(PREFIX)/bin
	cp ini $(DESTDIR)$(PREFIX)/bin/ini

uninstall:
	rm -f $(DESTDIR)$(PREFIX)/bin/ini

test: all
	go test -v .

clean:
	go clean
	rm -f *~ .*~ debug

maintainer-clean: clean

.PHONY: all install uninstall clean maintainer-clean test
.NOTPARALLEL:
