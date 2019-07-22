
TARGET = ini
EXTRA_CLEAN =

all: $(TARGET)

$(TARGET): always
	go build -tags ini

test man install uninstall built_sources:
	@:

clean: always
	go clean
	rm -f *~ .*~ debug testdata/*~ $(EXTRA_CLEAN)

maintainer-clean: clean
	rm -f $(BUILT_SOURCES)

always:
	@:

.PHONY: always
.NOTPARALLEL:
