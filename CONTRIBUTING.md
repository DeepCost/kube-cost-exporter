# Contributing to Kube Cost Exporter

Thank you for your interest in contributing to Kube Cost Exporter! This document provides guidelines and instructions for contributing.

## Table of Contents

- [Code of Conduct](#code-of-conduct)
- [Getting Started](#getting-started)
- [Development Setup](#development-setup)
- [Making Changes](#making-changes)
- [Testing](#testing)
- [Submitting Changes](#submitting-changes)

## Code of Conduct

This project adheres to a code of conduct. By participating, you are expected to uphold this code. Please be respectful and constructive in all interactions.

## Getting Started

1. Fork the repository on GitHub
2. Clone your fork locally:
   ```bash
   git clone https://github.com/YOUR_USERNAME/kube-cost-exporter.git
   cd kube-cost-exporter
   ```

3. Add the upstream repository:
   ```bash
   git remote add upstream https://github.com/deepcost/kube-cost-exporter.git
   ```

## Development Setup

### Prerequisites

- Go 1.21 or later
- Docker
- Kubernetes cluster (kind, minikube, or cloud provider)
- kubectl
- Helm 3.x
- make

### Install Dependencies

```bash
make deps
```

### Build the Application

```bash
make build
```

### Run Tests

```bash
make test
```

### Run Locally

You can run the exporter locally against a Kubernetes cluster:

```bash
go run cmd/agent/main.go \
  --kubeconfig=$HOME/.kube/config \
  --cloud-provider=aws \
  --region=us-east-1
```

## Making Changes

### Branch Naming

- Feature branches: `feature/description`
- Bug fixes: `fix/description`
- Documentation: `docs/description`

Example:
```bash
git checkout -b feature/add-cost-forecasting
```

### Code Style

- Follow standard Go conventions
- Run `make fmt` to format code
- Run `make lint` to check for issues
- Write meaningful commit messages

### Commit Messages

Follow the conventional commits specification:

- `feat:` New feature
- `fix:` Bug fix
- `docs:` Documentation changes
- `refactor:` Code refactoring
- `test:` Adding or updating tests
- `chore:` Maintenance tasks

Example:
```
feat: add GCP committed use discount support

- Implement CUD pricing calculation
- Add tests for GCP pricing
- Update documentation
```

## Testing

### Unit Tests

```bash
make test
```

### Integration Tests

```bash
# Deploy to local cluster
make deploy

# Run integration tests
make test-integration

# Clean up
make undeploy
```

### Test Coverage

```bash
make test-coverage
```

This generates a coverage report in `coverage.html`.

## Submitting Changes

### Before Submitting

1. Ensure all tests pass:
   ```bash
   make check
   ```

2. Update documentation if needed

3. Add tests for new features

4. Ensure your branch is up to date:
   ```bash
   git fetch upstream
   git rebase upstream/main
   ```

### Pull Request Process

1. Push your changes to your fork:
   ```bash
   git push origin feature/your-feature
   ```

2. Create a pull request on GitHub

3. Fill out the PR template with:
   - Description of changes
   - Related issues
   - Testing performed
   - Screenshots (if applicable)

4. Wait for review and address feedback

### PR Review Criteria

- Code follows project conventions
- Tests are included and passing
- Documentation is updated
- No breaking changes (or clearly documented)
- Commit history is clean

## Development Guidelines

### Adding a New Cloud Provider

1. Create a new provider in `pkg/pricing/`:
   ```go
   // pkg/pricing/newcloud.go
   type NewCloudProvider struct {
       // ...
   }

   func (p *NewCloudProvider) GetInstancePrice(ctx context.Context, instanceType, region, az string) (float64, error) {
       // Implementation
   }
   ```

2. Add tests in `pkg/pricing/newcloud_test.go`

3. Update documentation

4. Add example configuration to Helm chart

### Adding New Metrics

1. Define the metric in `pkg/metrics/exporter.go`:
   ```go
   newMetric: prometheus.NewGaugeVec(
       prometheus.GaugeOpts{
           Name: "kube_cost_new_metric",
           Help: "Description of new metric",
       },
       []string{"label1", "label2"},
   )
   ```

2. Update the metric in the appropriate method

3. Document the metric in `examples/prometheus-queries.md`

4. Add to Grafana dashboard if relevant

### Project Structure

```
kube-cost-exporter/
├── cmd/
│   └── agent/          # Main application entry point
├── pkg/
│   ├── calculator/     # Cost calculation logic
│   ├── collector/      # Kubernetes resource collectors
│   ├── metrics/        # Prometheus metrics
│   └── pricing/        # Cloud provider pricing
├── charts/             # Helm chart
├── deploy/             # Kubernetes manifests
├── dashboards/         # Grafana dashboards
├── examples/           # Example configurations
└── docs/               # Documentation
```

## Getting Help

- Open an issue for bugs or feature requests
- Join our Slack channel: [link]
- Check existing issues and pull requests

## License

By contributing, you agree that your contributions will be licensed under the same license as the project.
