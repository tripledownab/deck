APP := deck

# Install location. Defaults to ~/.local/bin since that's already on PATH on
# most macOS / Linux setups and doesn't need sudo. Override with PREFIX:
#   make install PREFIX=/usr/local/bin
#
# Same convention as cathode, so both binaries land side by side.
PREFIX ?= $(HOME)/.local/bin

# Where `make demo` builds its throwaway workspace. Under /tmp because it is
# disposable: three git repositories, their worktrees and a state directory,
# none of which should end up anywhere a backup would find them.
DEMO_DIR ?= /tmp/deck-demo

.PHONY: help build run test race vet fmt tidy install uninstall reinstall watch clean demo

help: ## list the targets
	@grep -E '^[a-z-]+:.*?## ' $(MAKEFILE_LIST) \
	  | awk -F':.*?## ' '{printf "  %-11s %s\n", $$1, $$2}'

build: ## compile the single binary
	go build -o $(APP) .

run: build ## build then launch in the current repo
	./$(APP)

test: ## run the unit and smoke tests
	go test ./...

race: ## run the tests under the race detector
	go test -race -timeout 300s ./...

vet: ## run go vet
	go vet ./...

fmt: ## gofmt the tree
	gofmt -w .

tidy: ## resolve deps + write go.sum
	go mod tidy

install: ## build and copy to $(PREFIX), creating it if needed
	@mkdir -p $(PREFIX)
	go build -o $(PREFIX)/$(APP) .
	@echo "installed: $(PREFIX)/$(APP)"
	@case ":$$PATH:" in *":$(PREFIX):"*) ;; *) \
	  echo "warn: $(PREFIX) is not on PATH — add it to ~/.zshrc or ~/.bashrc";; esac

reinstall: clean install ## rebuild from scratch and reinstall

demo: build ## seed a throwaway workspace to photograph Deck against
	go run ./demo -dir $(DEMO_DIR) $(if $(FORCE),-force $(DEMO_DIR))

uninstall: ## remove the installed binary
	@rm -f $(PREFIX)/$(APP) && echo "removed: $(PREFIX)/$(APP)" || true

watch: ## rebuild + reinstall on every *.go save (needs entr: `brew install entr`)
	@command -v entr >/dev/null || { echo "entr not found — run: brew install entr"; exit 1; }
	@find . -name '*.go' -not -path './.git/*' | entr -rc $(MAKE) install

clean: ## remove build artifacts
	rm -f $(APP)
