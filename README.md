# Kube Cost Exporter

> **Real-time Kubernetes cost monitoring that integrates with your existing Prometheus and Grafana stack.**

[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](./LICENSE)
[![Go Version](https://img.shields.io/badge/Go-1.21+-blue.svg)](https://golang.org/)
[![Kubernetes](https://img.shields.io/badge/Kubernetes-1.20+-blue.svg)](https://kubernetes.io/)

## What is Kube Cost Exporter?

Kube Cost Exporter is a lightweight, open-source tool that tracks your Kubernetes costs in real-time and exports them as Prometheus metrics. Get instant visibility into pod, namespace, and cluster costs without the overhead of enterprise tools.

### Why Kube Cost Exporter?

**The Problem:**
- Kubecost's free version lacks real-time metrics (15-day limit)
- Enterprise solutions cost $349+/month per cluster
- No native integration with existing Prometheus/Grafana stacks
- Manual cost calculations waste 8-12 hours/month

**Our Solution:**
- ✅ **Free & Open Source** - No licensing costs, ever
- ✅ **Real-time Metrics** - Updated every 60 seconds
- ✅ **Prometheus Native** - Works with your existing monitoring
- ✅ **Multi-Cloud** - Supports AWS, GCP, and Azure
- ✅ **Lightweight** - <100MB RAM, <0.1 CPU
- ✅ **Spot Savings Tracking** - See how much you're saving
- ✅ **Pre-built Dashboards** - Grafana dashboards included

## Quick Start

### Prerequisites

- Kubernetes cluster (1.20+)
- Helm 3.x
- Prometheus installed
- (Optional) Grafana for dashboards

### Installation

#### AWS (EKS)

```bash
helm repo add deepcost https://deepcost.github.io/kube-cost-exporter
helm repo update

helm install kube-cost-exporter deepcost/kube-cost-exporter \
  --namespace kube-system \
  --create-namespace \
  --set cloudProvider=aws \
  --set aws.region=us-east-1
```

#### GCP (GKE)

```bash
helm install kube-cost-exporter deepcost/kube-cost-exporter \
  --namespace kube-system \
  --create-namespace \
  --set cloudProvider=gcp \
  --set gcp.project=YOUR_PROJECT_ID
```

#### Azure (AKS)

```bash
helm install kube-cost-exporter deepcost/kube-cost-exporter \
  --namespace kube-system \
  --create-namespace \
  --set cloudProvider=azure \
  --set azure.subscriptionId=YOUR_SUBSCRIPTION_ID
```

### Verify Installation

```bash
# Check the pod is running
kubectl get pods -n kube-system -l app=kube-cost-exporter

# View metrics
kubectl port-forward -n kube-system svc/kube-cost-exporter 9090:9090
curl http://localhost:9090/metrics | grep kube_cost
```

## Features

### 📊 Real-Time Cost Metrics

Track costs at multiple levels:
- **Pod-level**: Individual pod costs per hour/day/month
- **Namespace-level**: Aggregated costs by namespace
- **Cluster-level**: Total cluster spend
- **Node-level**: Cost per node with instance type
- **Storage**: Persistent volume costs

### 💰 Cost Breakdown

```
Monthly Cluster Cost: $1,245.00
├─ Compute:  $890.00 (71%)
├─ Storage:  $200.00 (16%)
└─ Savings:  $155.00 (12% from spot instances)

Top Namespaces:
1. production    $456.00
2. staging       $234.00
3. ml-training   $189.00
```

### 📈 Prometheus Metrics

All metrics are exported in Prometheus format:

```promql
# Total monthly cluster cost
sum(kube_cost_cluster_hourly_usd) * 730

# Cost by namespace
sum(kube_cost_namespace_hourly_usd) by (namespace) * 730

# Top 10 most expensive pods
topk(10, kube_cost_pod_hourly_usd * 730)

# Spot instance savings
sum(kube_cost_spot_savings_hourly_usd) * 730
```

See [Prometheus Query Examples](examples/prometheus-queries.md) for more.

### 📉 Grafana Dashboards

Pre-built dashboards included:

- **Cost Overview** (`dashboards/kubernetes-cost-overview.json`)
  - Cluster-wide cost summary
  - Per-namespace breakdown
  - Most expensive pods and namespaces
  - Cost trends over time

- **Spot Savings Analysis** (`dashboards/spot-savings-analysis.json`)
  - Monthly spot instance savings
  - Node distribution (spot vs on-demand)
  - Spot percentage gauge
  - Namespace-level spot usage
  - Cost breakdown by instance type

- **Storage Costs** (`dashboards/storage-costs.json`)
  - Total storage costs
  - Per-namespace storage breakdown
  - Persistent volume cost analysis
  - Storage capacity tracking

### 🚨 Cost Alerts

Set up budget alerts with Prometheus AlertManager:

```yaml
- alert: NamespaceCostBudgetExceeded
  expr: sum(kube_cost_namespace_daily_usd{namespace="production"}) * 30 > 500
  annotations:
    summary: "Production exceeds $500/month budget"
```

See [Prometheus Alerts](examples/prometheus-alerts.yaml) for examples.

### 🔌 kubectl Plugin

Query costs directly from your terminal:

```bash
# Install kubectl-cost plugin
curl -L https://github.com/deepcost/kube-cost-exporter/releases/latest/download/kubectl-cost -o kubectl-cost
chmod +x kubectl-cost
sudo mv kubectl-cost /usr/local/bin/

# Query namespace costs
kubectl cost namespace production --window 30d

# Show cluster summary
kubectl cost cluster

# Top 10 pods by cost
kubectl cost top pods
```

## Configuration

### Basic Configuration

```yaml
# values.yaml
cloudProvider: aws

aws:
  region: us-east-1
  irsaRoleArn: "arn:aws:iam::123456789:role/kube-cost-exporter"

updateInterval: 60s  # How often to collect metrics

resources:
  requests:
    memory: 128Mi
    cpu: 100m
  limits:
    memory: 512Mi
    cpu: 500m
```

### Cloud Provider Setup

#### AWS (with IRSA)

1. Create IAM policy:

```json
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Action": [
        "ec2:DescribeSpotPriceHistory",
        "pricing:GetProducts"
      ],
      "Resource": "*"
    }
  ]
}
```

2. Create IRSA role:

```bash
eksctl create iamserviceaccount \
  --name kube-cost-exporter \
  --namespace kube-system \
  --cluster YOUR_CLUSTER \
  --attach-policy-arn arn:aws:iam::ACCOUNT:policy/KubeCostPolicy \
  --approve
```

See [Installation Guide](INSTALL.md) for detailed cloud provider setup.

## Usage Examples

### Monitor Namespace Costs

```bash
# Get current costs
kubectl cost namespace production

# Output:
# NAMESPACE    HOURLY COST  TOTAL COST  MONTHLY PROJECTION
# production   $1.85        $44.40      $1,350.50
```

### Set Up Budget Alerts

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: cost-budgets
data:
  budgets.yaml: |
    production:
      monthly_limit: 1000
      alert_at: 80%  # Alert at 80% of budget
```

### View Costs in Grafana

1. Import dashboard from `dashboards/kubernetes-cost-overview.json`
2. Select your Prometheus datasource
3. View real-time cost metrics

### CI/CD Integration

Automatically check cost impact of deployments:

```yaml
# .github/workflows/cost-check.yaml
- name: Check Cost Impact
  run: |
    kubectl cost estimate -f deployment.yaml
    # Fails if cost increase > 50%
```

See [CI/CD Examples](examples/ci-cd/) for GitHub Actions and GitLab CI.

## Supported Metrics

### Compute Metrics

| Metric | Description | Labels |
|--------|-------------|--------|
| `kube_cost_pod_hourly_usd` | Hourly pod cost | namespace, pod, node |
| `kube_cost_namespace_hourly_usd` | Hourly namespace cost | namespace |
| `kube_cost_namespace_daily_usd` | Daily namespace cost | namespace |
| `kube_cost_node_hourly_usd` | Hourly node cost | node, instance_type, is_spot |
| `kube_cost_cluster_hourly_usd` | Total cluster hourly cost | - |

### Storage Metrics

| Metric | Description | Labels |
|--------|-------------|--------|
| `kube_cost_pv_monthly_usd` | Monthly persistent volume cost | pv_name, storage_class, namespace |
| `kube_cost_namespace_storage_monthly_usd` | Monthly storage cost per namespace | namespace |
| `kube_cost_cluster_storage_monthly_usd` | Total cluster monthly storage cost | - |

### Spot Instance Metrics

| Metric | Description | Labels |
|--------|-------------|--------|
| `kube_cost_spot_savings_hourly_usd` | Hourly savings from spot instances | - |
| `kube_cost_spot_node_count` | Number of spot/preemptible nodes | - |
| `kube_cost_ondemand_node_count` | Number of on-demand nodes | - |
| `kube_cost_spot_percentage` | Percentage of nodes that are spot instances | - |
| `kube_cost_spot_hourly_usd` | Hourly cost of all spot instances | - |
| `kube_cost_ondemand_hourly_usd` | Hourly cost of all on-demand instances | - |
| `kube_cost_namespace_spot_pods` | Number of pods on spot instances | namespace |
| `kube_cost_namespace_spot_percentage` | Percentage of namespace pods on spot | namespace |

## Architecture

Kube Cost Exporter consists of:

1. **Cost Agent** - Collects resource data from Kubernetes API
2. **Pricing Engine** - Fetches pricing from AWS/GCP/Azure APIs
3. **Calculator** - Computes costs based on resource allocation
4. **Metrics Exporter** - Exports Prometheus metrics

See [Architecture Documentation](ARCHITECTURE.md) for detailed design.

## Comparison

| Feature | Kube-Cost-Exporter | Kubecost Free | Kubecost Enterprise |
|---------|-------------------|---------------|---------------------|
| **Price** | Free | Free | $349+/cluster/mo |
| **Real-time** | ✅ Yes (60s) | ❌ No | ✅ Yes |
| **Prometheus** | ✅ Native | ⚠️ Limited | ✅ Yes |
| **Grafana** | ✅ Included | ❌ No | ✅ Yes |
| **Data Retention** | ♾️ Unlimited | 15 days | Custom |
| **Multi-Cloud** | ✅ AWS/GCP/Azure | ✅ Yes | ✅ Yes |
| **Spot Tracking** | ✅ Yes | ❌ No | ✅ Yes |
| **Self-Hosted** | ✅ Yes | ✅ Yes | ✅ Yes |

## Documentation

- **[Installation Guide](INSTALL.md)** - Detailed installation instructions
- **[Quick Start](examples/quickstart.md)** - Get started in 5 minutes
- **[Architecture](ARCHITECTURE.md)** - Technical design and implementation
- **[Prometheus Queries](examples/prometheus-queries.md)** - Example PromQL queries
- **[Prometheus Alerts](examples/prometheus-alerts.yaml)** - Pre-configured alerts
- **[Contributing](CONTRIBUTING.md)** - How to contribute

## Support & Community

- **Issues**: [GitHub Issues](https://github.com/deepcost/kube-cost-exporter/issues)
- **Discussions**: [GitHub Discussions](https://github.com/deepcost/kube-cost-exporter/discussions)
- **Documentation**: [Full Docs](https://docs.deepcost.ai)

## Contributing

We welcome contributions! Please see [CONTRIBUTING.md](CONTRIBUTING.md) for guidelines.

## License

Kube Cost Exporter is licensed under the [MIT License](LICENSE).

---

**Built with ❤️ by the DeepCost team**

*Making Kubernetes cost visibility accessible to everyone*
