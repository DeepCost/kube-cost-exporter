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

## Release Process

### Overview

Kube Cost Exporter uses semantic versioning (SemVer) and automated releases through GitHub Actions.

### Release Types

- **Patch Release** (v1.0.X): Bug fixes, minor improvements
- **Minor Release** (v1.X.0): New features, backwards-compatible changes
- **Major Release** (vX.0.0): Breaking changes, major features

### Creating a Release

#### 1. Prepare the Release

```bash
# Ensure you're on main branch
git checkout main
git pull origin main

# Update CHANGELOG.md with release notes
# Update version in charts/kube-cost-exporter/Chart.yaml

# Commit version changes
git add CHANGELOG.md charts/kube-cost-exporter/Chart.yaml
git commit -m "chore: prepare release v1.2.3"
git push origin main
```

#### 2. Create a Git Tag

```bash
# Create an annotated tag
git tag -a v1.2.3 -m "Release v1.2.3"

# Push the tag to trigger release workflows
git push origin v1.2.3
```

#### 3. Automated Release Process

When you push a tag (e.g., `v1.2.3`), GitHub Actions automatically:

1. **Docker Image Release**:
   - Builds multi-arch Docker images (amd64, arm64)
   - Pushes to `deepcost/kube-cost-exporter:1.2.3`
   - Pushes to `deepcost/kube-cost-exporter:latest`
   - Creates GitHub release with changelog

2. **Helm Chart Release**:
   - Updates Chart.yaml with version
   - Packages the Helm chart
   - Publishes to GitHub Pages (gh-pages branch)
   - Available at `https://deepcost.github.io/kube-cost-exporter`

3. **kubectl Plugin Release**:
   - Builds kubectl-cost for multiple platforms
   - Attaches binaries to GitHub release

### Manual Release (If Needed)

#### Docker Image

```bash
# Build multi-arch image
docker buildx build --platform linux/amd64,linux/arm64 \
  -t deepcost/kube-cost-exporter:v1.2.3 \
  -t deepcost/kube-cost-exporter:latest \
  --push .
```

#### Helm Chart

```bash
# Package the chart
helm package charts/kube-cost-exporter

# Checkout gh-pages branch
git checkout gh-pages

# Add the packaged chart
mv kube-cost-exporter-1.2.3.tgz .

# Update index
helm repo index . --url https://deepcost.github.io/kube-cost-exporter

# Commit and push
git add .
git commit -m "Release chart version 1.2.3"
git push origin gh-pages

# Return to main branch
git checkout main
```

#### kubectl Plugin

```bash
# Build for all platforms
make build-plugin

# Binaries will be in bin/:
# - kubectl-cost-linux-amd64
# - kubectl-cost-linux-arm64
# - kubectl-cost-darwin-amd64
# - kubectl-cost-darwin-arm64
# - kubectl-cost-windows-amd64.exe
```

### Release Checklist

Before creating a release, ensure:

- [ ] All tests pass (`make check`)
- [ ] Documentation is updated
- [ ] CHANGELOG.md is updated with release notes
- [ ] Version numbers are updated in:
  - [ ] `charts/kube-cost-exporter/Chart.yaml` (version and appVersion)
  - [ ] Any version constants in code
- [ ] Breaking changes are documented
- [ ] Migration guide is provided (if needed)
- [ ] Security vulnerabilities are addressed

### Post-Release

After a release is published:

1. **Verify Docker Image**:
   ```bash
   docker pull deepcost/kube-cost-exporter:v1.2.3
   docker run --rm deepcost/kube-cost-exporter:v1.2.3 --version
   ```

2. **Verify Helm Chart**:
   ```bash
   helm repo update
   helm search repo deepcost/kube-cost-exporter --versions
   ```

3. **Update Documentation**:
   - Update installation guides with new version
   - Publish blog post for major releases
   - Announce on community channels

4. **Monitor**:
   - Watch for issues related to the new release
   - Monitor Docker Hub download stats
   - Check Helm chart usage metrics

### Hotfix Releases

For critical bugs in production:

1. Create a hotfix branch from the release tag:
   ```bash
   git checkout -b hotfix/v1.2.4 v1.2.3
   ```

2. Fix the bug and commit:
   ```bash
   git commit -m "fix: critical bug in cost calculation"
   ```

3. Create a new patch tag:
   ```bash
   git tag -a v1.2.4 -m "Hotfix: critical bug fix"
   git push origin v1.2.4
   ```

4. Merge hotfix back to main:
   ```bash
   git checkout main
   git merge hotfix/v1.2.4
   git push origin main
   ```

### Release Notes Template

```markdown
## [v1.2.3] - 2024-01-15

### Added
- New feature X that does Y
- Support for Z cloud provider

### Changed
- Improved performance of cost calculation by 30%
- Updated dependencies to latest versions

### Fixed
- Fixed bug where spot instance prices were incorrect
- Resolved memory leak in pricing cache

### Security
- Updated vulnerable dependency X to v2.0.0

### Breaking Changes
- Configuration parameter `oldParam` renamed to `newParam`
  - Migration: Update your values.yaml to use `newParam`
```

## Getting Help

- Open an issue for bugs or feature requests
- Join our Slack channel: [link]
- Check existing issues and pull requests

## License

By contributing, you agree that your contributions will be licensed under the same license as the project.
