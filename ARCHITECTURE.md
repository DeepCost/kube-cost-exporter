# Kube Cost Exporter - Architecture Documentation

This document provides detailed technical architecture and design information for Kube Cost Exporter.

## Table of Contents

- [High-Level Architecture](#high-level-architecture)
- [Component Architecture](#component-architecture)
- [Data Flow](#data-flow)
- [Database Design](#database-design)
- [Security Architecture](#security-architecture)
- [Scalability](#scalability-considerations)
- [Technology Stack](#technology-stack)
- [Configuration](#configuration-options)
- [Testing Strategy](#testing-strategy)

---

## High-Level Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                      Kubernetes Cluster                      │
├─────────────────────────────────────────────────────────────┤
│                                                               │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐       │
│  │   Node 1     │  │   Node 2     │  │   Node 3     │       │
│  │              │  │              │  │              │       │
│  │ ┌──────────┐ │  │ ┌──────────┐ │  │ ┌──────────┐ │       │
│  │ │Cost Agent│ │  │ │Cost Agent│ │  │ │Cost Agent│ │       │
│  │ └────┬─────┘ │  │ └────┬─────┘ │  │ └────┬─────┘ │       │
│  └──────┼───────┘  └──────┼───────┘  └──────┼───────┘       │
│         │                  │                  │               │
│         └──────────────────┼──────────────────┘               │
│                            │                                  │
│                    ┌───────▼────────┐                         │
│                    │ Aggregator Svc │                         │
│                    │  (Deployment)  │                         │
│                    └───────┬────────┘                         │
│                            │                                  │
└────────────────────────────┼──────────────────────────────────┘
                             │
                    ┌────────▼────────┐
                    │ Prometheus      │◄────── Scrape /metrics
                    │   (9090)        │
                    └────────┬────────┘
                             │
                    ┌────────▼────────┐
                    │   Grafana       │◄────── Visualize
                    │   (3000)        │
                    └─────────────────┘

External APIs:
┌──────────────────┐
│ AWS Pricing API  │────► Spot prices, instance costs
│ GCP Billing API  │────► Sustained use discounts
│ Azure Retail API │────► VM pricing, storage costs
└──────────────────┘
```

---

## Component Architecture

### 1. Cost Agent (Deployment)

The Cost Agent is deployed as a single Deployment that collects cluster-wide cost information.

**Responsibilities:**
- Collects pod resource usage and requests
- Reads node instance type and pricing
- Collects persistent volume information
- Calculates per-pod costs in real-time
- Exports metrics via Prometheus endpoint

**Data Collection Flow:**
```
Node Metadata → Instance Type → Cloud Pricing API → Hourly Rate
     +
Pod Resources → CPU/Memory Requests → Resource Allocation → Pod Cost
     +
Storage → PV Size → Storage Pricing → Storage Cost
     ↓
Metrics: kube_cost_pod_hourly_usd{namespace, pod, node}
```

**Key Metrics Collected:**
- CPU requests/usage (millicores)
- Memory requests/usage (bytes)
- Storage (persistent volumes)
- Node instance types and pricing
- Spot vs on-demand classification

### 2. Pricing Engine

The pricing engine provides a unified interface for fetching pricing from different cloud providers.

**Cloud Provider Adapters:**

```
┌────────────────────────────────────────────┐
│          Pricing Engine Interface          │
├────────────────────────────────────────────┤
│                                            │
│  getInstancePrice(type, region, az)       │
│  getSpotPrice(type, region, az)           │
│  getStoragePrice(type, size, region)      │
│  getNetworkPrice(region, destination)     │
│                                            │
└──────┬─────────────┬─────────────┬─────────┘
       │             │             │
   ┌───▼───┐    ┌────▼────┐   ┌───▼────┐
   │  AWS  │    │   GCP   │   │ Azure  │
   │Adapter│    │ Adapter │   │Adapter │
   └───────┘    └─────────┘   └────────┘
```

**Pricing Data Sources:**
- **AWS**: EC2 Pricing API, Spot Price History API
- **GCP**: Cloud Billing API, Committed Use Discounts
- **Azure**: Retail Prices API, Reserved Instances

**Caching Strategy:**
- Instance pricing: 1 hour TTL
- Spot prices: 5 minutes TTL
- Storage pricing: 24 hours TTL
- Network egress: 1 hour TTL

**Rate Limiting:**
- Max 100 API calls/minute to cloud providers
- Exponential backoff on errors
- Circuit breaker pattern for API failures
- Aggressive caching to minimize API calls

### 3. Cost Calculator

The calculator component performs all cost calculations based on resource allocation.

**Calculation Logic:**

```
Pod Cost = (Pod Resource Request / Node Total Capacity) × Node Hourly Cost

Storage Cost = Volume Size (GB) × Storage Price per GB/month

Namespace Cost = Σ (Pod Costs in Namespace) + Σ (Storage Costs)

Cluster Cost = Σ (All Node Costs) + Σ (Storage Costs)

Spot Savings = (On-Demand Price - Spot Price) × Spot Instance Hours
```

### 4. Metrics Exporter

Exports Prometheus-compatible metrics for monitoring and visualization.

**Exported Metrics:**

| Metric Name | Type | Labels | Description |
|------------|------|--------|-------------|
| `kube_cost_pod_hourly_usd` | Gauge | namespace, pod, node | Hourly cost per pod |
| `kube_cost_namespace_hourly_usd` | Gauge | namespace | Hourly cost per namespace |
| `kube_cost_namespace_daily_usd` | Gauge | namespace | Daily cost per namespace |
| `kube_cost_node_hourly_usd` | Gauge | node, instance_type, is_spot | Hourly cost per node |
| `kube_cost_cluster_hourly_usd` | Gauge | | Total hourly cluster cost |
| `kube_cost_spot_savings_hourly_usd` | Gauge | | Hourly savings from spot instances |
| `kube_cost_pv_monthly_usd` | Gauge | pv_name, namespace, storage_class | Monthly PV cost |
| `kube_cost_cluster_storage_monthly_usd` | Gauge | | Total monthly storage cost |

---

## Data Flow Architecture

```
┌─────────────────────────────────────────────────────────────┐
│ Step 1: Metrics Collection (Every 60 seconds)               │
└─────────────────────────────────────────────────────────────┘
         │
         ▼
┌─────────────────────────────────────────────────────────────┐
│ Cost Agent reads:                                            │
│ • kubectl get nodes (instance type, zone, labels)           │
│ • kubectl get pods (resource requests, assignments)         │
│ • kubectl get pv (storage class, size)                      │
│ • Kubernetes API (resource metadata)                        │
└─────────────────────────────────────────────────────────────┘
         │
         ▼
┌─────────────────────────────────────────────────────────────┐
│ Step 2: Pricing Data Retrieval                              │
└─────────────────────────────────────────────────────────────┘
         │
         ▼
┌─────────────────────────────────────────────────────────────┐
│ Pricing Engine:                                              │
│ • Check cache for pricing data                              │
│ • Query Cloud Provider API if cache miss                    │
│ • Store in cache with appropriate TTL                       │
│ • Return pricing: $0.096/hour for m5.large                  │
└─────────────────────────────────────────────────────────────┘
         │
         ▼
┌─────────────────────────────────────────────────────────────┐
│ Step 3: Cost Calculation                                     │
└─────────────────────────────────────────────────────────────┘
         │
         ▼
┌─────────────────────────────────────────────────────────────┐
│ Calculator:                                                  │
│ • Calculate pod cost: (500m CPU / 2000m CPU) × $0.096      │
│ • Result: $0.024/hour per pod                               │
│ • Aggregate by namespace                                    │
│ • Calculate storage costs                                   │
└─────────────────────────────────────────────────────────────┘
         │
         ▼
┌─────────────────────────────────────────────────────────────┐
│ Step 4: Metric Export                                        │
└─────────────────────────────────────────────────────────────┘
         │
         ▼
┌─────────────────────────────────────────────────────────────┐
│ Exporter exposes:                                            │
│ GET /metrics                                                 │
│                                                              │
│ # HELP kube_cost_pod_hourly_usd Hourly cost of pod in USD  │
│ # TYPE kube_cost_pod_hourly_usd gauge                       │
│ kube_cost_pod_hourly_usd{namespace="prod",pod="api"} 0.024  │
└─────────────────────────────────────────────────────────────┘
         │
         ▼
┌─────────────────────────────────────────────────────────────┐
│ Step 5: Prometheus Scrape & Storage                         │
└─────────────────────────────────────────────────────────────┘
         │
         ▼
┌─────────────────────────────────────────────────────────────┐
│ Grafana visualizes:                                          │
│ • Real-time cost dashboards                                 │
│ • Cost trends and forecasts                                 │
│ • Budget vs actual comparisons                              │
└─────────────────────────────────────────────────────────────┘
```

---

## Database Design

### Optional Time-Series Storage

For long-term cost data retention (>15 days), you can use TimescaleDB or another time-series database.

**TimescaleDB Schema:**

```sql
-- Pod costs historical data
CREATE TABLE pod_costs (
    timestamp TIMESTAMPTZ NOT NULL,
    cluster_id VARCHAR(50),
    namespace VARCHAR(63),
    pod_name VARCHAR(253),
    node_name VARCHAR(63),
    instance_type VARCHAR(50),
    cpu_millicores INT,
    memory_bytes BIGINT,
    hourly_cost_usd DECIMAL(10, 6),
    PRIMARY KEY (timestamp, cluster_id, namespace, pod_name)
);

-- Hypertable for time-series optimization
SELECT create_hypertable('pod_costs', 'timestamp');

-- Retention policy: 90 days
SELECT add_retention_policy('pod_costs', INTERVAL '90 days');

-- Continuous aggregate for daily rollups
CREATE MATERIALIZED VIEW daily_namespace_costs
WITH (timescaledb.continuous) AS
SELECT
    time_bucket('1 day', timestamp) AS day,
    cluster_id,
    namespace,
    SUM(hourly_cost_usd) * 24 AS daily_cost_usd
FROM pod_costs
GROUP BY day, cluster_id, namespace;

-- Index for fast queries
CREATE INDEX idx_pod_costs_namespace ON pod_costs (namespace, timestamp DESC);
CREATE INDEX idx_pod_costs_cluster ON pod_costs (cluster_id, timestamp DESC);
```

---

## Security Architecture

### Authentication & Authorization

```
┌─────────────────────────────────────────────┐
│ RBAC Security Model                         │
├─────────────────────────────────────────────┤
│                                             │
│ Cost Exporter Service Account:             │
│  - Read-only access to nodes, pods, PVs    │
│  - No write permissions                     │
│  - No secret access                         │
│  - No exec/attach permissions              │
│                                             │
│ Cloud Provider Credentials:                │
│  - AWS: IRSA with least-privilege IAM role │
│  - GCP: Workload Identity                  │
│  - Azure: Managed Identity                 │
│                                             │
│ Metrics Endpoint:                          │
│  - HTTP endpoint (no sensitive data)       │
│  - Optional: Basic auth or bearer token    │
│  - Optional: IP whitelist for Prometheus   │
│  - Optional: TLS encryption                │
└─────────────────────────────────────────────┘
```

### Data Privacy

- **No sensitive data collected**: Only resource metadata and costs
- **No secrets or environment variables**: Configuration via ConfigMaps
- **No PII by default**: Pod/namespace names can be anonymized
- **Read-only permissions**: Cannot modify cluster resources
- **Network isolation**: Runs in kube-system namespace

### Cloud Provider IAM Policies

**AWS Minimum Required Permissions:**
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

**GCP Minimum Required Permissions:**
- `compute.instances.list` (Compute Engine Viewer)
- `compute.disks.list` (Compute Engine Viewer)

**Azure Minimum Required Permissions:**
- Reader role on subscription

---

## Scalability Considerations

### Horizontal Scaling

- **Single Deployment**: Runs as one deployment (not DaemonSet)
- **Stateless Design**: Can be scaled horizontally if needed
- **Prometheus Federation**: Supports multi-cluster deployments

### Performance Benchmarks

| Cluster Size | Memory Usage | CPU Usage | Latency (p99) |
|-------------|--------------|-----------|---------------|
| 100 pods | <100MB | <0.1 CPU | <10ms |
| 1,000 pods | <150MB | <0.3 CPU | <30ms |
| 10,000 pods | <500MB | <1.5 CPU | <100ms |

### Optimization Techniques

1. **Caching Strategy**
   - Aggressive caching of pricing data
   - In-memory cache with TTL
   - Reduces API calls by 95%

2. **Batch Processing**
   - Collect all metrics in one cycle
   - Update Prometheus metrics atomically
   - Configurable update interval (default: 60s)

3. **Resource Efficiency**
   - Written in Go for low memory footprint
   - Minimal CPU usage
   - Alpine-based container (<50MB)

4. **API Rate Limiting**
   - Respect cloud provider rate limits
   - Exponential backoff on failures
   - Circuit breaker pattern

---

## Technology Stack

### Core Components

- **Language**: Go 1.21+
  - High performance
  - Small binary size
  - Excellent concurrency support

- **Metrics**: Prometheus client library
  - Industry-standard metrics format
  - Native Kubernetes integration
  - Built-in aggregation support

- **Kubernetes Client**: client-go
  - Official Kubernetes Go client
  - Watches and caching support
  - Auto-retry and backoff

- **Cloud SDKs**:
  - AWS SDK v2
  - Google Cloud Go SDK
  - Azure SDK for Go

### Deployment

- **Packaging**: Helm chart
- **Container**: Multi-stage Docker build, Alpine-based (<50MB)
- **Configuration**: ConfigMaps and environment variables
- **Secrets**: Cloud credentials via IRSA/Workload Identity

---

## Configuration Options

### Helm Values

```yaml
# Cloud provider configuration
cloudProvider: aws  # aws | gcp | azure

aws:
  region: us-east-1
  irsaRoleArn: "arn:aws:iam::123456789:role/kube-cost-exporter"

gcp:
  project: my-gcp-project
  workloadIdentity: "kube-cost-exporter@project.iam.gserviceaccount.com"

azure:
  subscriptionId: "xxxxx-xxxx-xxxx"
  managedIdentityClientId: "xxxxx-xxxx"

# Update interval
updateInterval: 60s

# Metrics configuration
metrics:
  port: 9090
  path: /metrics

# Resource limits
resources:
  requests:
    memory: 128Mi
    cpu: 100m
  limits:
    memory: 512Mi
    cpu: 500m

# Prometheus integration
serviceMonitor:
  enabled: true
  interval: 60s

# Grafana dashboard
grafanaDashboard:
  enabled: true
```

### Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `AWS_REGION` | AWS region for pricing | us-east-1 |
| `GCP_PROJECT` | GCP project ID | - |
| `AZURE_SUBSCRIPTION_ID` | Azure subscription | - |
| `UPDATE_INTERVAL` | Cost update interval | 60s |
| `METRICS_PORT` | Metrics HTTP port | 9090 |
| `LOG_LEVEL` | Logging level | info |

---

## Testing Strategy

### Unit Tests

```bash
# Run unit tests
make test

# With coverage
make test-coverage
```

**Test Coverage:**
- Pricing calculation accuracy
- Cloud provider adapter mocking
- Metric aggregation logic
- Cache TTL and expiration
- Error handling and retries

### Integration Tests

```bash
# Deploy to kind cluster
kind create cluster
make deploy

# Run integration tests
make test-integration
```

**Integration Test Scenarios:**
- Deploy to local Kubernetes cluster
- Verify metrics endpoint responds
- Validate Prometheus scraping works
- Test multi-namespace scenarios

### End-to-End Tests

**Manual Validation:**
1. Deploy to production cluster
2. Compare calculated costs to cloud bill
3. Validate spot price accuracy (within 5%)
4. Test Grafana dashboard rendering
5. Verify alerts trigger correctly

---

## Monitoring & Observability

### Self-Monitoring Metrics

The exporter exposes metrics about its own operation:

```
# Exporter health
kube_cost_exporter_up{component="agent"} 1

# API call metrics
kube_cost_pricing_api_calls_total{provider="aws",endpoint="spot_price"} 1234
kube_cost_pricing_api_errors_total{provider="aws",error_type="rate_limit"} 5

# Cache metrics
kube_cost_cache_hits_total 9500
kube_cost_cache_misses_total 500
kube_cost_cache_size_bytes 1048576

# Processing metrics
kube_cost_collection_duration_seconds{quantile="0.99"} 0.045
kube_cost_pods_collected 150
kube_cost_nodes_collected 5
```

### Logging

```
{
  "level": "info",
  "msg": "Collecting cost metrics...",
  "nodes": 5,
  "pods": 150,
  "pvs": 10
}

{
  "level": "info",
  "msg": "Metrics updated successfully",
  "cluster_hourly_cost": 2.45,
  "duration_ms": 234
}
```

---

## Future Enhancements

1. **FinOps Recommendations**
   - Auto-detect over-provisioned pods (>50% unused resources)
   - Suggest reserved instance purchases
   - Identify idle resources and orphaned PVs

2. **Cost Forecasting**
   - ML-based cost predictions
   - Budget burn-down charts
   - Anomaly detection (cost spikes >30%)

3. **Multi-Cluster Support**
   - Centralized cost aggregation
   - Cross-cluster cost comparison
   - Global cost dashboards

4. **Enhanced Integrations**
   - Slack bot for cost queries
   - Jira integration for cost alerts
   - Terraform provider for cost budgets
   - CSV export for finance reporting

5. **Advanced Features**
   - Network egress cost tracking
   - GPU cost calculation
   - Reserved instance utilization tracking
   - Savings plan recommendations
