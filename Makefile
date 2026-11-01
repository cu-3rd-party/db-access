# One entrypoint for build, lint and the
# three test layers (unit, integration, e2e). CI only calls make targets.
GO ?= go
NODE ?= node
NPM ?= npm
NPX ?= npx

GOLANGCI_LINT_PACKAGE ?= github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.6.1
GOFUMPT_PACKAGE ?= mvdan.cc/gofumpt@v0.9.2
GOVULNCHECK_PACKAGE ?= golang.org/x/vuln/cmd/govulncheck@v1.1.4

EXECUTABLE ?= dbaccess
TAGS ?=
LDFLAGS ?= -s -w
GOTESTFLAGS ?=
PLAYWRIGHT_PROJECT ?= chromium

GO_SOURCES := $(shell find . -type f -name '*.go' -not -path './node_modules/*')
WEB_SOURCES := $(shell find web_src templates -type f) package-lock.json webpack.config.ts tailwind.config.ts
GO_TEST_PACKAGES ?= $(filter-out git.cu3rd.ru/cu3rd/db-access/tests/integration git.cu3rd.ru/cu3rd/db-access/tests/e2e,$(shell $(GO) list ./...))

.PHONY: all
all: build

.PHONY: help
help:
	@echo "Make Routines:"
	@echo " - build                 build frontend + backend"
	@echo " - frontend / backend    build one side"
	@echo " - watch                 rebuild frontend and restart backend on change"
	@echo " - fmt / fmt-check       format Go (gofumpt) sources"
	@echo " - lint / lint-fix       all linters"
	@echo " - test                  test-frontend + test-backend (unit)"
	@echo " - test-sqlite           integration tests on sqlite"
	@echo " - test-pgsql            integration tests on postgres (TEST_PGSQL_DSN)"
	@echo " - test-e2e[#name]       playwright e2e tests (PLAYWRIGHT_PROJECT=$(PLAYWRIGHT_PROJECT))"
	@echo " - checks                tidy-check, fmt-check, security-check"

## build

.PHONY: build
build: frontend backend

.PHONY: frontend
frontend: public/assets/js/index.js

public/assets/js/index.js: node_modules $(WEB_SOURCES)
	$(NPX) webpack
	@touch public/assets/js/index.js

.PHONY: backend
backend: $(EXECUTABLE)

$(EXECUTABLE): $(GO_SOURCES)
	CGO_ENABLED=0 $(GO) build -trimpath -tags '$(TAGS)' -ldflags '$(LDFLAGS)' -o $@ .

node_modules: package-lock.json
	$(NPM) install --no-save
	@touch node_modules

.PHONY: deps
deps: deps-frontend deps-backend

.PHONY: deps-frontend
deps-frontend: node_modules

.PHONY: deps-backend
deps-backend:
	$(GO) mod download

.PHONY: watch
watch:
	@bash -c 'trap "kill 0" EXIT; $(MAKE) watch-frontend & $(MAKE) watch-backend & wait'

.PHONY: watch-frontend
watch-frontend: node_modules
	NODE_ENV=development $(NPX) webpack --watch --progress

.PHONY: watch-backend
watch-backend:
	$(GO) run github.com/air-verse/air@v1.63.0 -c .air.toml

.PHONY: clean
clean:
	rm -rf $(EXECUTABLE) public/assets data coverage.out \
		tests/e2e/reports/ tests/e2e/test-artifacts/ tests/e2e/test-snapshots/

## format / lint / checks

.PHONY: fmt
fmt:
	$(GO) run $(GOFUMPT_PACKAGE) -w .

.PHONY: fmt-check
fmt-check:
	@diff=$$($(GO) run $(GOFUMPT_PACKAGE) -l .); \
	if [ -n "$$diff" ]; then echo "Please run 'make fmt':"; echo "$$diff"; exit 1; fi

.PHONY: lint
lint: lint-frontend lint-backend

.PHONY: lint-fix
lint-fix: lint-frontend-fix lint-backend-fix

.PHONY: lint-frontend
lint-frontend: lint-js lint-css tsc

.PHONY: lint-frontend-fix
lint-frontend-fix: lint-js-fix lint-css-fix

.PHONY: lint-backend
lint-backend: lint-go lint-go-vet

.PHONY: lint-backend-fix
lint-backend-fix: lint-go-fix

.PHONY: lint-js
lint-js: node_modules
	$(NPX) eslint --max-warnings=0 .

.PHONY: lint-js-fix
lint-js-fix: node_modules
	$(NPX) eslint --max-warnings=0 --fix .

.PHONY: lint-css
lint-css: node_modules
	$(NPX) stylelint --max-warnings=0 'web_src/**/*.css'

.PHONY: lint-css-fix
lint-css-fix: node_modules
	$(NPX) stylelint --max-warnings=0 --fix 'web_src/**/*.css'

.PHONY: tsc
tsc: node_modules
	$(NPX) tsc

.PHONY: lint-go
lint-go:
	$(GO) run $(GOLANGCI_LINT_PACKAGE) run

.PHONY: lint-go-fix
lint-go-fix:
	$(GO) run $(GOLANGCI_LINT_PACKAGE) run --fix

.PHONY: lint-go-vet
lint-go-vet:
	$(GO) vet ./...

.PHONY: checks
checks: tidy-check fmt-check security-check

.PHONY: tidy
tidy:
	$(GO) mod tidy

.PHONY: tidy-check
tidy-check: tidy
	@git diff --exit-code go.mod go.sum || (echo "Please run 'make tidy' and commit the result" && exit 1)

.PHONY: security-check
security-check:
	$(GO) run $(GOVULNCHECK_PACKAGE) ./...

## tests

.PHONY: test
test: test-frontend test-backend

.PHONY: test-frontend
test-frontend: node_modules
	$(NPX) vitest run

.PHONY: test-backend
test-backend:
	$(GO) test $(GOTESTFLAGS) -tags='$(TAGS)' $(GO_TEST_PACKAGES)

.PHONY: test-sqlite
test-sqlite:
	DBACCESS_DB_TYPE=sqlite $(GO) test $(GOTESTFLAGS) ./tests/integration/...

# run a single integration test: make test-sqlite#TestRotate
.PHONY: test-sqlite\#%
test-sqlite\#%:
	DBACCESS_DB_TYPE=sqlite $(GO) test $(GOTESTFLAGS) ./tests/integration/... -run '$*'

TEST_PGSQL_DSN ?= postgres://postgres:postgres@localhost:5432/test?sslmode=disable

.PHONY: test-pgsql
test-pgsql:
	DBACCESS_DB_TYPE=postgres DBACCESS_DB_DSN='$(TEST_PGSQL_DSN)' $(GO) test -p 1 $(GOTESTFLAGS) $(GO_TEST_PACKAGES) ./tests/integration/...

# privileges on a real cluster; the test creates databases and roles: use a throwaway one
.PHONY: test-provision
test-provision:
	$(GO) test -count=1 ./services/provision/...

.PHONY: playwright
playwright: deps-frontend
	$(NPX) playwright install $(PLAYWRIGHT_FLAGS) $(PLAYWRIGHT_PROJECT_BROWSER)

.PHONY: test-e2e
test-e2e: frontend
	PLAYWRIGHT_PROJECT='$(PLAYWRIGHT_PROJECT)' $(GO) test $(GOTESTFLAGS) -count=1 ./tests/e2e/... -run 'TestE2e'

# run one file: make test-e2e#access  (tests/e2e/access.test.e2e.ts)
.PHONY: test-e2e\#%
test-e2e\#%: frontend
	PLAYWRIGHT_PROJECT='$(PLAYWRIGHT_PROJECT)' $(GO) test $(GOTESTFLAGS) -count=1 ./tests/e2e/... -run 'TestE2e/$*'

.PHONY: coverage
coverage:
	$(GO) test -coverpkg=./... -coverprofile=coverage.out $(GO_TEST_PACKAGES) ./tests/integration/...
	$(GO) tool cover -func=coverage.out | tail -1

## CI aggregates

.PHONY: pr-go
pr-go: deps-backend lint-backend checks test-backend test-sqlite

.PHONY: pr-frontend
pr-frontend: deps-frontend lint-frontend test-frontend frontend
