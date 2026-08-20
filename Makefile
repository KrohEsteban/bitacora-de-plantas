# Bitácora de Plantas — all checks run inside Docker. The same `make ci`
# command is what CI (GitHub Actions) runs.
#
#   make lint       golangci-lint inside a container
#   make test       unit + integration tests (go test .)
#   make test-e2e   chromedp E2E against the app in an isolated compose stack
#   make ci         lint + test + test-e2e, in order
#   make up / down  docker compose helpers for the app itself

COMPOSE         := docker compose
TEST_IMAGE_LINT := bitacora-plantas-test-lint
TEST_IMAGE_E2E  := bitacora-plantas-test-e2e
GO_MOD_CACHE    := bitacora-go-mod-cache
GO_BUILD_CACHE  := bitacora-go-build-cache
E2E_PROJECT     := bitacora-e2e
E2E_COMPOSE     := docker-compose.e2e.yml

.PHONY: build
build:
	docker build -f Dockerfile.test --target lint -t $(TEST_IMAGE_LINT) .
	docker build -f Dockerfile.test --target e2e -t $(TEST_IMAGE_E2E) .

.PHONY: lint
lint: build
	docker run --rm \
		-v $(CURDIR):/workspace:ro \
		-w /workspace \
		-v $(GO_MOD_CACHE):/go/pkg/mod \
		-v $(GO_BUILD_CACHE):/root/.cache/go-build \
		-e GOFLAGS=-mod=mod \
		$(TEST_IMAGE_LINT) sh -c "go mod download && golangci-lint run ./..."

.PHONY: test
test: build
	docker run --rm \
		-v $(CURDIR):/workspace \
		-w /workspace \
		-v $(GO_MOD_CACHE):/go/pkg/mod \
		-v $(GO_BUILD_CACHE):/root/.cache/go-build \
		-e GOFLAGS=-mod=mod \
		$(TEST_IMAGE_LINT) go test -count=1 -cover -v .

.PHONY: test-e2e
test-e2e: build
	docker volume create $(GO_MOD_CACHE) >/dev/null
	docker volume create $(GO_BUILD_CACHE) >/dev/null
	@echo "==> Starting isolated E2E stack (project $(E2E_PROJECT))"
	$(COMPOSE) -p $(E2E_PROJECT) -f $(E2E_COMPOSE) up --build --abort-on-container-exit --exit-code-from e2e \
		|| (rc=$$?; $(COMPOSE) -p $(E2E_PROJECT) -f $(E2E_COMPOSE) down -v --remove-orphans; exit $$rc)
	$(COMPOSE) -p $(E2E_PROJECT) -f $(E2E_COMPOSE) down -v --remove-orphans

.PHONY: ci
ci: lint test test-e2e

.PHONY: up
up:
	$(COMPOSE) up -d --build

.PHONY: down
down:
	$(COMPOSE) down