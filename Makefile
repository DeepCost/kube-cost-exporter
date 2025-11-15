.PHONY: help build test docker-build docker-push deploy clean

# Variables
APP_NAME := kube-cost-exporter
DOCKER_REGISTRY := deepcost
DOCKER_IMAGE := $(DOCKER_REGISTRY)/$(APP_NAME)
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
PLATFORMS := linux/amd64,linux/arm64

# Go variables
GOCMD := go
GOBUILD := $(GOCMD) build
GOCLEAN := $(GOCMD) clean
GOTEST := $(GOCMD) test
GOGET := $(GOCMD) get
GOMOD := $(GOCMD) mod

# Build directories
BUILD_DIR := bin
CMD_DIR := cmd/agent

# Color output
RED := \033[0;31m
GREEN := \033[0;32m
YELLOW := \033[1;33m
NC := \033[0m # No Color

help: ## Show this help message
	@echo "$(GREEN)$(APP_NAME) - Makefile commands$(NC)"
	@echo ""
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "  $(YELLOW)%-20s$(NC) %s\n", $$1, $$2}'

deps: ## Download Go module dependencies
	@echo "$(GREEN)Downloading dependencies...$(NC)"
	$(GOMOD) download
	$(GOMOD) tidy

build: deps ## Build the application binary
	@echo "$(GREEN)Building $(APP_NAME)...$(NC)"
	@mkdir -p $(BUILD_DIR)
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 $(GOBUILD) -a -installsuffix cgo \
		-ldflags="-w -s -X main.Version=$(VERSION)" \
		-o $(BUILD_DIR)/$(APP_NAME) \
		./$(CMD_DIR)
	@echo "$(GREEN)Build complete: $(BUILD_DIR)/$(APP_NAME)$(NC)"

test: ## Run unit tests
	@echo "$(GREEN)Running tests...$(NC)"
	$(GOTEST) -v -race -coverprofile=coverage.out ./...
	@echo "$(GREEN)Tests complete$(NC)"

test-coverage: test ## Run tests and show coverage
	@echo "$(GREEN)Generating coverage report...$(NC)"
	$(GOCMD) tool cover -html=coverage.out -o coverage.html
	@echo "$(GREEN)Coverage report: coverage.html$(NC)"

lint: ## Run linters
	@echo "$(GREEN)Running linters...$(NC)"
	@which golangci-lint > /dev/null || (echo "$(RED)golangci-lint not found. Install it from https://golangci-lint.run/$(NC)" && exit 1)
	golangci-lint run ./...

docker-build: ## Build Docker image
	@echo "$(GREEN)Building Docker image: $(DOCKER_IMAGE):$(VERSION)$(NC)"
	docker build -t $(DOCKER_IMAGE):$(VERSION) .
	docker tag $(DOCKER_IMAGE):$(VERSION) $(DOCKER_IMAGE):latest
	@echo "$(GREEN)Docker image built: $(DOCKER_IMAGE):$(VERSION)$(NC)"

docker-build-multiarch: ## Build multi-architecture Docker image
	@echo "$(GREEN)Building multi-arch Docker image: $(DOCKER_IMAGE):$(VERSION)$(NC)"
	docker buildx build --platform $(PLATFORMS) \
		-t $(DOCKER_IMAGE):$(VERSION) \
		-t $(DOCKER_IMAGE):latest \
		--push .
	@echo "$(GREEN)Multi-arch Docker image built and pushed$(NC)"

docker-push: docker-build ## Push Docker image to registry
	@echo "$(GREEN)Pushing Docker image: $(DOCKER_IMAGE):$(VERSION)$(NC)"
	docker push $(DOCKER_IMAGE):$(VERSION)
	docker push $(DOCKER_IMAGE):latest
	@echo "$(GREEN)Docker image pushed$(NC)"

helm-lint: ## Lint Helm chart
	@echo "$(GREEN)Linting Helm chart...$(NC)"
	helm lint charts/$(APP_NAME)

helm-package: ## Package Helm chart
	@echo "$(GREEN)Packaging Helm chart...$(NC)"
	helm package charts/$(APP_NAME)
	@echo "$(GREEN)Helm chart packaged$(NC)"

helm-install: ## Install Helm chart locally (for testing)
	@echo "$(GREEN)Installing Helm chart...$(NC)"
	helm upgrade --install $(APP_NAME) charts/$(APP_NAME) \
		--namespace kube-system \
		--create-namespace \
		--set image.tag=$(VERSION)
	@echo "$(GREEN)Helm chart installed$(NC)"

helm-uninstall: ## Uninstall Helm chart
	@echo "$(YELLOW)Uninstalling Helm chart...$(NC)"
	helm uninstall $(APP_NAME) --namespace kube-system
	@echo "$(GREEN)Helm chart uninstalled$(NC)"

deploy: docker-build helm-install ## Build and deploy locally

deploy-manifests: ## Deploy using raw Kubernetes manifests
	@echo "$(GREEN)Deploying Kubernetes manifests...$(NC)"
	kubectl apply -f deploy/rbac.yaml
	kubectl apply -f deploy/deployment.yaml
	@echo "$(GREEN)Deployed successfully$(NC)"

undeploy-manifests: ## Remove Kubernetes manifests
	@echo "$(YELLOW)Removing Kubernetes manifests...$(NC)"
	kubectl delete -f deploy/deployment.yaml --ignore-not-found=true
	kubectl delete -f deploy/rbac.yaml --ignore-not-found=true
	@echo "$(GREEN)Undeployed successfully$(NC)"

logs: ## Show application logs
	@echo "$(GREEN)Fetching logs...$(NC)"
	kubectl logs -n kube-system -l app=$(APP_NAME) --tail=100 -f

metrics: ## Query metrics endpoint
	@echo "$(GREEN)Querying metrics...$(NC)"
	kubectl port-forward -n kube-system svc/$(APP_NAME) 9090:9090 &
	sleep 2
	curl -s http://localhost:9090/metrics | grep kube_cost
	pkill -f "port-forward.*$(APP_NAME)"

clean: ## Clean build artifacts
	@echo "$(YELLOW)Cleaning build artifacts...$(NC)"
	$(GOCLEAN)
	rm -rf $(BUILD_DIR)
	rm -f coverage.out coverage.html
	rm -f $(APP_NAME)-*.tgz
	@echo "$(GREEN)Clean complete$(NC)"

fmt: ## Format Go code
	@echo "$(GREEN)Formatting code...$(NC)"
	$(GOCMD) fmt ./...

vet: ## Run go vet
	@echo "$(GREEN)Running go vet...$(NC)"
	$(GOCMD) vet ./...

check: fmt vet lint test ## Run all checks (fmt, vet, lint, test)

.DEFAULT_GOAL := help
