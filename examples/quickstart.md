# Quick Start Guide

Get Kube Cost Exporter up and running in 5 minutes!

## Prerequisites

- Kubernetes cluster (EKS, GKE, AKS, or any Kubernetes 1.20+)
- kubectl configured
- Helm 3.x installed
- Prometheus installed in your cluster

## Step 1: Install Kube Cost Exporter

### For AWS EKS

```bash
# Add Helm repository
helm repo add deepcost https://charts.deepcost.ai
helm repo update

# Install the chart
helm install kube-cost-exporter deepcost/kube-cost-exporter \
  --namespace kube-system \
  --create-namespace \
  --set cloudProvider=aws \
  --set aws.region=us-east-1
```

### For GCP GKE

```bash
helm install kube-cost-exporter deepcost/kube-cost-exporter \
  --namespace kube-system \
  --create-namespace \
  --set cloudProvider=gcp \
  --set gcp.project=YOUR_PROJECT_ID
```

### For Azure AKS

```bash
helm install kube-cost-exporter deepcost/kube-cost-exporter \
  --namespace kube-system \
  --create-namespace \
  --set cloudProvider=azure \
  --set azure.subscriptionId=YOUR_SUBSCRIPTION_ID
```

## Step 2: Verify Installation

```bash
# Check if the pod is running
kubectl get pods -n kube-system -l app=kube-cost-exporter

# Should show:
# NAME                                   READY   STATUS    RESTARTS   AGE
# kube-cost-exporter-xxxxxxxxxx-xxxxx    1/1     Running   0          1m
```

## Step 3: View Metrics

```bash
# Port forward to the metrics endpoint
kubectl port-forward -n kube-system svc/kube-cost-exporter 9090:9090 &

# Query metrics
curl http://localhost:9090/metrics | grep kube_cost

# You should see output like:
# kube_cost_pod_hourly_usd{namespace="default",pod="nginx-xxx",node="node-1"} 0.045
# kube_cost_namespace_hourly_usd{namespace="default"} 0.234
# kube_cost_cluster_hourly_usd 5.67
```

## Step 4: Query in Prometheus

Open your Prometheus UI and run this query:

```promql
# Total monthly cluster cost
sum(kube_cost_cluster_hourly_usd) * 730
```

More query examples: [prometheus-queries.md](prometheus-queries.md)

## Step 5: Import Grafana Dashboard

1. Download the dashboard:
   ```bash
   curl -O https://raw.githubusercontent.com/deepcost/kube-cost-exporter/main/dashboards/kubernetes-cost-overview.json
   ```

2. In Grafana:
   - Go to Dashboards → Import
   - Upload `kubernetes-cost-overview.json`
   - Select your Prometheus datasource
   - Click Import

3. View your cost dashboard!

## What's Next?

### Set Up Alerts

Apply the example alerts:

```bash
kubectl apply -f https://raw.githubusercontent.com/deepcost/kube-cost-exporter/main/examples/prometheus-alerts.yaml
```

### Customize Configuration

Create a `values.yaml` file:

```yaml
cloudProvider: aws

aws:
  region: us-east-1

# Update metrics every 30 seconds (default is 60s)
updateInterval: 30s

# Adjust resource limits
resources:
  limits:
    memory: 256Mi
    cpu: 200m
```

Apply the configuration:

```bash
helm upgrade kube-cost-exporter deepcost/kube-cost-exporter \
  --namespace kube-system \
  -f values.yaml
```

### Explore Cost Data

#### See your most expensive namespaces

```bash
kubectl port-forward -n monitoring svc/prometheus-operated 9090:9090 &

# Open http://localhost:9090 and run:
# topk(5, sum(kube_cost_namespace_daily_usd) by (namespace) * 30)
```

#### Check spot instance savings

```promql
# Spot savings percentage
(sum(kube_cost_spot_savings_hourly_usd) /
 (sum(kube_cost_cluster_hourly_usd) + sum(kube_cost_spot_savings_hourly_usd))) * 100
```

#### Find expensive pods

```promql
# Top 10 most expensive pods
topk(10, kube_cost_pod_hourly_usd * 730)
```

## Troubleshooting

### Metrics Not Showing Up?

1. Check logs:
   ```bash
   kubectl logs -n kube-system -l app=kube-cost-exporter
   ```

2. Verify Prometheus is scraping:
   - Open Prometheus UI
   - Go to Status → Targets
   - Look for `kube-cost-exporter`

3. Check ServiceMonitor (if using Prometheus Operator):
   ```bash
   kubectl get servicemonitor -n kube-system kube-cost-exporter
   ```

### Need More Help?

- [Full Installation Guide](../INSTALL.md)
- [Prometheus Queries](prometheus-queries.md)
- [GitHub Issues](https://github.com/deepcost/kube-cost-exporter/issues)

## Example Use Cases

### 1. Cost Breakdown by Team

Add labels to your namespaces:

```bash
kubectl label namespace frontend team=web
kubectl label namespace backend team=api
kubectl label namespace ml team=data-science
```

Then query by team:

```promql
sum(kube_cost_namespace_hourly_usd * on(namespace) group_left(team)
    kube_namespace_labels) by (team) * 730
```

### 2. Budget Alerts

Set up an alert when a namespace exceeds budget:

```yaml
- alert: NamespaceBudgetExceeded
  expr: sum(kube_cost_namespace_daily_usd{namespace="production"}) * 30 > 1000
  annotations:
    summary: "Production namespace exceeds $1000/month budget"
```

### 3. Cost Optimization

Find pods without resource requests:

```promql
kube_pod_container_resource_requests{resource="cpu"} == 0
```

These pods may be inefficiently allocated!

## Common Commands

```bash
# View live costs
kubectl port-forward -n kube-system svc/kube-cost-exporter 9090:9090
watch -n 5 'curl -s http://localhost:9090/metrics | grep kube_cost_cluster_hourly'

# Update configuration
helm upgrade kube-cost-exporter deepcost/kube-cost-exporter \
  --reuse-values \
  --set updateInterval=120s

# View logs
kubectl logs -n kube-system -l app=kube-cost-exporter -f

# Restart exporter
kubectl rollout restart deployment/kube-cost-exporter -n kube-system

# Uninstall
helm uninstall kube-cost-exporter -n kube-system
```

---

**That's it!** You now have real-time Kubernetes cost metrics. 🎉

For advanced configuration and features, see the [full documentation](../README.md).
