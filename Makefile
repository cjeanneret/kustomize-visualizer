# Kustomap - Makefile for build and user-level install (no sudo)
BINARY  := kustomap
DESTDIR ?= $(HOME)
BINDIR  ?= $(DESTDIR)/.bin
UNITDIR ?= $(DESTDIR)/.config/systemd/user

.PHONY: build install install-binary install-systemd uninstall
.DEFAULT_GOAL := build

build:
	CGO_ENABLED=0 go build -ldflags="-s -w" -o $(BINARY) .

install: build install-binary install-systemd
	@echo ""
	@echo "Installation complete (no sudo required):"
	@echo "  Binary:     $(BINDIR)/$(BINARY)"
	@echo "  Unit file:  $(UNITDIR)/$(BINARY).service"
	@echo ""
	@echo "Enable and start the user service:"
	@echo "  systemctl --user daemon-reload"
	@echo "  systemctl --user enable --now $(BINARY).service"
	@echo ""
	@echo "Then open http://localhost:3000"

install-binary: build
	@mkdir -p "$(BINDIR)"
	@cp "$(BINARY)" "$(BINDIR)/$(BINARY)"
	@echo "Installed $(BINARY) to $(BINDIR)/$(BINARY)"

install-systemd:
	@mkdir -p "$(UNITDIR)"
	@sed 's|%BINDIR%|$(BINDIR)|g' install/$(BINARY).service.in > "$(UNITDIR)/$(BINARY).service"
	@echo "Installed systemd user unit to $(UNITDIR)/$(BINARY).service"

uninstall:
	@rm -f "$(BINDIR)/$(BINARY)"
	@rm -f "$(UNITDIR)/$(BINARY).service"
	@echo "Removed $(BINARY) and systemd unit"
