BUILT_SOURCES = header.go parse.go qual_t_string.go
EXTRA_CLEAN = y.output

all: $(BUILT_SOURCES)
	go build

$(BUILT_SOURCES): header.go.in parse.y syms.go goxdr.go
	go generate

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

.PHONY: all clean maintainer-clean
.NOTPARALLEL:
