DESTDIR =
PREFIX = /usr/local
MANDIR = $(PREFIX)/share/man

TARGET = stc
GENERATE_INPUTS =
BUILT_SOURCES =
EXTRA_CLEAN =

all: $(TARGET)

$(TARGET): $(BUILT_SOURCES) always
	go build

$(BUILT_SOURCES): generate-stamp

generate-stamp: $(GENERATE_INPUTS)
	PATH="$$PATH:$$(go env GOPATH)/bin" go generate
	sed -e 's|^[ 	]*//line|//line|' parse.go > parse.go~ \
		&& mv -f parse.go~ parse.go
	touch $@

test: all
	go test -v

man: $(TARGET).1

$(TARGET).1: $(TARGET).1.md
	pandoc -s -w man $@.md -o $@ || \
	    git show --output=$@ \
	    $$(git for-each-ref --count 1 --format '%(refname)' 'refs/remotes/*/go1'):./$@

install: $(TARGET) $(TARGET).1 always
	mkdir -p $(DESTDIR)$(PREFIX)/bin
	cp $(TARGET) $(DESTDIR)$(PREFIX)/bin/$(TARGET)
	mkdir -p $(DESTDIR)$(MANDIR)/man1
	cp $(TARGET).1 $(DESTDIR)$(MANDIR)/man1/$(TARGET).1

uninstall: always
	rm -f $(DESTDIR)$(PREFIX)/bin/$(TARGET) \
		$(DESTDIR)$(MANDIR)/man1/$(TARGET).1

clean: always
	go clean
	rm -f *~ .*~ debug testdata/*~ $(EXTRA_CLEAN)

maintainer-clean: clean
	rm -f $(BUILT_SOURCES) $(TARGET).1 generate-stamp

built_sources: $(BUILT_SOURCES) $(TARGET).1 always
	for file in $(BUILT_SOURCES) $(TARGET).1; do \
		echo cmd/$$(basename $$PWD)/$$file >> ../../built_sources; \
	done

always:
	@:

.PHONY: always
.NOTPARALLEL:
