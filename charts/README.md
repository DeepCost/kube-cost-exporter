# Helm Charts

This directory contains the Helm charts for Kube Cost Exporter.

## Helm Repository

The Helm repository is hosted on GitHub Pages at:
```
https://deepcost.github.io/kube-cost-exporter
```

### Adding the Repository

```bash
helm repo add deepcost https://deepcost.github.io/kube-cost-exporter
helm repo update
```

### Installing the Chart

```bash
helm install kube-cost-exporter deepcost/kube-cost-exporter \
  --namespace kube-system \
  --create-namespace \
  --set cloudProvider=aws \
  --set aws.region=us-east-1
```

## Publishing Charts to GitHub Pages

To publish a new chart version to GitHub Pages:

1. **Package the chart:**
   ```bash
   helm package charts/kube-cost-exporter
   ```

2. **Move to gh-pages branch:**
   ```bash
   git checkout gh-pages
   mv kube-cost-exporter-*.tgz .
   ```

3. **Update the index:**
   ```bash
   helm repo index . --url https://deepcost.github.io/kube-cost-exporter
   ```

4. **Commit and push:**
   ```bash
   git add .
   git commit -m "Release chart version X.Y.Z"
   git push origin gh-pages
   ```

## GitHub Pages Setup

To set up GitHub Pages for the first time:

1. Create a `gh-pages` branch:
   ```bash
   git checkout --orphan gh-pages
   git rm -rf .
   ```

2. Create initial index:
   ```bash
   helm repo index . --url https://deepcost.github.io/kube-cost-exporter
   ```

3. Add a README:
   ```bash
   echo "# Kube Cost Exporter Helm Charts" > README.md
   ```

4. Commit and push:
   ```bash
   git add .
   git commit -m "Initialize Helm chart repository"
   git push origin gh-pages
   ```

5. Enable GitHub Pages in repository settings:
   - Go to Settings > Pages
   - Set source to `gh-pages` branch
   - Save

## Chart Versioning

Chart versions follow Semantic Versioning (SemVer):
- **MAJOR**: Incompatible API changes
- **MINOR**: Backwards-compatible functionality
- **PATCH**: Backwards-compatible bug fixes

Update the version in `charts/kube-cost-exporter/Chart.yaml` before packaging.

## Local Testing

To test the chart locally before publishing:

```bash
# Install from local directory
helm install kube-cost-exporter ./charts/kube-cost-exporter \
  --namespace kube-system \
  --dry-run --debug

# Or template it
helm template kube-cost-exporter ./charts/kube-cost-exporter \
  --namespace kube-system
```

## CI/CD Automation

For automated chart releases, see `.github/workflows/release-chart.yaml` (if configured).
