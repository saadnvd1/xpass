PREFIX ?= /usr/local/bin

build:
	go build -o xpass .

install: build
	cp xpass $(PREFIX)/xpass
	@echo "Installed xpass to $(PREFIX)/xpass"

uninstall:
	rm -f $(PREFIX)/xpass

clean:
	rm -f xpass

.PHONY: build install uninstall clean
