# Installation Guide

This guide provides detailed instructions for installing and configuring Kube Cost Exporter in your Kubernetes cluster.

## Prerequisites

- Kubernetes cluster (v1.20+)
- `kubectl` configured to access your cluster
- Helm 3.x (for Helm installation method)
- Prometheus operator or Prometheus server installed
- (Optional) Grafana for visualization

## Installation Methods

### Method 1: Helm Chart (Recommended)

#### 1. Add Helm Repository

```bash
helm repo add deepcost https://charts.deepcost.ai
helm repo update
```

#### 2. Install for AWS EKS

```bash
helm install kube-cost-exporter deepcost/kube-cost-exporter \
  --namespace kube-system \
  --set cloudProvider=aws \
  --set aws.region=us-east-1 \
  --set aws.irsaRoleArn=arn:aws:iam::YOUR_ACCOUNT:role/kube-cost-exporter
```

#### 3. Install for GCP GKE

```bash
helm install kube-cost-exporter deepcost/kube-cost-exporter \
  --namespace kube-system \
  --set cloudProvider=gcp \
  --set gcp.project=YOUR_GCP_PROJECT \
  --set gcp.workloadIdentity=kube-cost-exporter@YOUR_PROJECT.iam.gserviceaccount.com
```

#### 4. Install for Azure AKS

```bash
helm install kube-cost-exporter deepcost/kube-cost-exporter \
  --namespace kube-system \
  --set cloudProvider=azure \
  --set azure.subscriptionId=YOUR_SUBSCRIPTION_ID
```

### Method 2: Kubernetes Manifests

```bash
# Apply RBAC
kubectl apply -f deploy/rbac.yaml

# Apply deployment
kubectl apply -f deploy/deployment.yaml
```

## Cloud Provider Configuration

### AWS Configuration

#### Option 1: IRSA (Recommended for EKS)

1. Create IAM policy:

```json
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Action": [
        "ec2:DescribeSpotPriceHistory",
        "ec2:DescribeInstances",
        "pricing:GetProducts"
      ],
      "Resource": "*"
    }
  ]
}
```

2. Create IAM role with IRSA:

```bash
eksctl create iamserviceaccount \
  --name kube-cost-exporter \
  --namespace kube-system \
  --cluster YOUR_CLUSTER_NAME \
  --attach-policy-arn arn:aws:iam::YOUR_ACCOUNT:policy/KubeCostExporterPolicy \
  --approve \
  --override-existing-serviceaccounts
```

3. Install with Helm:

```bash
helm install kube-cost-exporter deepcost/kube-cost-exporter \
  --namespace kube-system \
  --set cloudProvider=aws \
  --set aws.region=us-east-1 \
  --set aws.irsaRoleArn=arn:aws:iam::YOUR_ACCOUNT:role/eksctl-YOUR_CLUSTER-addon-iamserviceaccount-Role
```

#### Option 2: AWS Access Keys (Not Recommended for Production)

Create a Kubernetes secret:

```bash
kubectl create secret generic aws-credentials \
  --namespace kube-system \
  --from-literal=AWS_ACCESS_KEY_ID=YOUR_ACCESS_KEY \
  --from-literal=AWS_SECRET_ACCESS_KEY=YOUR_SECRET_KEY
```

### GCP Configuration

1. Create a service account:

```bash
gcloud iam service-accounts create kube-cost-exporter \
  --display-name="Kube Cost Exporter"
```

2. Grant required permissions:

```bash
gcloud projects add-iam-policy-binding YOUR_PROJECT_ID \
  --member="serviceAccount:kube-cost-exporter@YOUR_PROJECT.iam.gserviceaccount.com" \
  --role="roles/compute.viewer"
```

3. Enable Workload Identity:

```bash
gcloud iam service-accounts add-iam-policy-binding \
  kube-cost-exporter@YOUR_PROJECT.iam.gserviceaccount.com \
  --role roles/iam.workloadIdentityUser \
  --member "serviceAccount:YOUR_PROJECT.svc.id.goog[kube-system/kube-cost-exporter]"
```

### Azure Configuration

1. Create a managed identity or use system-assigned identity on AKS

2. Grant required permissions:

```bash
az role assignment create \
  --assignee YOUR_IDENTITY_CLIENT_ID \
  --role "Reader" \
  --scope /subscriptions/YOUR_SUBSCRIPTION_ID
```

## Verify Installation

1. Check if the pod is running:

```bash
kubectl get pods -n kube-system -l app=kube-cost-exporter
```

2. Check logs:

```bash
kubectl logs -n kube-system -l app=kube-cost-exporter
```

3. Verify metrics are being exported:

```bash
kubectl port-forward -n kube-system svc/kube-cost-exporter 9090:9090
curl http://localhost:9090/metrics | grep kube_cost
```

Expected output:
```
kube_cost_pod_hourly_usd{namespace="production",pod="api-server-xyz",node="node-1"} 0.045
kube_cost_namespace_hourly_usd{namespace="production"} 1.234
kube_cost_node_hourly_usd{node="node-1",instance_type="m5.large",is_spot="false"} 0.096
```

## Configure Prometheus

### Option 1: ServiceMonitor (Prometheus Operator)

The ServiceMonitor is automatically created if you installed via Helm with `serviceMonitor.enabled=true`.

Verify it's working:

```bash
kubectl get servicemonitor -n kube-system kube-cost-exporter
```

### Option 2: Manual Prometheus Configuration

Add this to your Prometheus configuration:

```yaml
scrape_configs:
  - job_name: 'kube-cost-exporter'
    kubernetes_sd_configs:
      - role: service
        namespaces:
          names:
            - kube-system
    relabel_configs:
      - source_labels: [__meta_kubernetes_service_label_app]
        action: keep
        regex: kube-cost-exporter
      - source_labels: [__meta_kubernetes_service_annotation_prometheus_io_scrape]
        action: keep
        regex: true
      - source_labels: [__meta_kubernetes_service_annotation_prometheus_io_path]
        action: replace
        target_label: __metrics_path__
        regex: (.+)
      - source_labels: [__address__, __meta_kubernetes_service_annotation_prometheus_io_port]
        action: replace
        regex: ([^:]+)(?::\d+)?;(\d+)
        replacement: $1:$2
        target_label: __address__
```

## Install Grafana Dashboards

### Option 1: Import via Grafana UI

1. Open Grafana
2. Go to Dashboards → Import
3. Upload `dashboards/kubernetes-cost-overview.json`
4. Select your Prometheus datasource
5. Click Import

### Option 2: ConfigMap (Grafana with Dashboard Sidecar)

```bash
kubectl create configmap kube-cost-dashboard \
  --from-file=dashboards/kubernetes-cost-overview.json \
  --namespace monitoring

kubectl label configmap kube-cost-dashboard \
  grafana_dashboard=1 \
  --namespace monitoring
```

## Configure Alerts

Apply the Prometheus alert rules:

```bash
kubectl apply -f examples/prometheus-alerts.yaml
```

Verify alerts are loaded:

```bash
kubectl exec -n monitoring prometheus-k8s-0 -- \
  wget -qO- http://localhost:9090/api/v1/rules | jq '.data.groups[] | select(.name=="kubernetes-cost-alerts")'
```

## Configuration Options

### Helm Values

Create a `values.yaml` file:

```yaml
cloudProvider: aws

aws:
  region: us-east-1
  irsaRoleArn: "arn:aws:iam::123456789:role/kube-cost-exporter"

updateInterval: 60s

resources:
  requests:
    memory: 128Mi
    cpu: 100m
  limits:
    memory: 512Mi
    cpu: 500m

serviceMonitor:
  enabled: true
  interval: 60s

grafanaDashboard:
  enabled: true
```

Install with custom values:

```bash
helm install kube-cost-exporter deepcost/kube-cost-exporter \
  --namespace kube-system \
  -f values.yaml
```

## Troubleshooting

### Metrics Not Appearing

1. Check if the service is running:
```bash
kubectl get pods -n kube-system -l app=kube-cost-exporter
```

2. Check logs for errors:
```bash
kubectl logs -n kube-system -l app=kube-cost-exporter
```

3. Verify Prometheus is scraping:
```bash
kubectl port-forward -n monitoring svc/prometheus-operated 9090:9090
# Open http://localhost:9090/targets and look for kube-cost-exporter
```

### Pricing API Errors

If you see errors related to pricing API:

**AWS:**
- Verify IAM permissions include `pricing:GetProducts`
- Ensure IRSA is configured correctly
- Check if the region is correct

**GCP:**
- Verify Workload Identity is configured
- Check if the service account has `compute.viewer` role
- Ensure `GCP_PROJECT` environment variable is set

**Azure:**
- Verify Managed Identity has Reader role
- Check if `AZURE_SUBSCRIPTION_ID` is correct

### High Memory Usage

If the exporter is using too much memory:

1. Increase the `updateInterval` to reduce API calls:
```bash
helm upgrade kube-cost-exporter deepcost/kube-cost-exporter \
  --reuse-values \
  --set updateInterval=120s
```

2. Increase resource limits:
```bash
helm upgrade kube-cost-exporter deepcost/kube-cost-exporter \
  --reuse-values \
  --set resources.limits.memory=1Gi
```

## Upgrading

### Helm Upgrade

```bash
helm repo update
helm upgrade kube-cost-exporter deepcost/kube-cost-exporter \
  --namespace kube-system \
  --reuse-values
```

### Manifest Upgrade

```bash
kubectl apply -f deploy/rbac.yaml
kubectl apply -f deploy/deployment.yaml
kubectl rollout status deployment/kube-cost-exporter -n kube-system
```

## Uninstallation

### Helm

```bash
helm uninstall kube-cost-exporter --namespace kube-system
```

### Manifests

```bash
kubectl delete -f deploy/deployment.yaml
kubectl delete -f deploy/rbac.yaml
```

## Next Steps

- [Configure Prometheus queries](examples/prometheus-queries.md)
- [Set up alerts](examples/prometheus-alerts.yaml)
- Import Grafana dashboards
- Integrate with your CI/CD pipeline
- Set up cost budgets and notifications
