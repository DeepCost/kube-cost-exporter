# Prometheus Query Examples for Kube Cost Exporter

This document provides useful Prometheus queries for analyzing Kubernetes costs.

## Basic Cost Queries

### Total Monthly Cluster Cost
```promql
sum(kube_cost_cluster_hourly_usd) * 730
```

### Total Daily Cluster Cost
```promql
sum(kube_cost_cluster_hourly_usd) * 24
```

### Current Hourly Cluster Cost
```promql
sum(kube_cost_cluster_hourly_usd)
```

## Namespace Cost Queries

### Monthly Cost by Namespace
```promql
sum(kube_cost_namespace_daily_usd) by (namespace) * 30
```

### Top 5 Most Expensive Namespaces (Monthly)
```promql
topk(5, sum(kube_cost_namespace_daily_usd) by (namespace) * 30)
```

### Namespace Cost as Percentage of Total
```promql
(
  sum(kube_cost_namespace_hourly_usd) by (namespace) /
  sum(kube_cost_namespace_hourly_usd)
) * 100
```

### Namespace Cost Trend (Last 7 Days)
```promql
sum(rate(kube_cost_namespace_hourly_usd[7d])) by (namespace) * 24 * 7
```

## Pod Cost Queries

### Top 10 Most Expensive Pods (Hourly)
```promql
topk(10, kube_cost_pod_hourly_usd)
```

### Top 10 Most Expensive Pods (Monthly)
```promql
topk(10, kube_cost_pod_hourly_usd * 730)
```

### Average Pod Cost per Namespace
```promql
avg(kube_cost_pod_hourly_usd) by (namespace)
```

### Total Pod Count and Cost by Namespace
```promql
sum(kube_cost_pod_hourly_usd) by (namespace)
```

## Node Cost Queries

### Cost by Instance Type
```promql
sum(kube_cost_node_hourly_usd) by (instance_type)
```

### Most Expensive Nodes
```promql
topk(5, kube_cost_node_hourly_usd)
```

### Average Node Cost
```promql
avg(kube_cost_node_hourly_usd)
```

### Total Number of Nodes by Type
```promql
count(kube_cost_node_hourly_usd) by (instance_type)
```

## Spot Instance Queries

### Total Spot Instance Savings (Hourly)
```promql
sum(kube_cost_spot_savings_hourly_usd)
```

### Monthly Spot Instance Savings
```promql
sum(kube_cost_spot_savings_hourly_usd) * 730
```

### Spot Instance Savings Percentage
```promql
(
  sum(kube_cost_spot_savings_hourly_usd) /
  (sum(kube_cost_cluster_hourly_usd) + sum(kube_cost_spot_savings_hourly_usd))
) * 100
```

### Cost by Spot vs On-Demand
```promql
sum(kube_cost_node_hourly_usd) by (is_spot)
```

### Number of Spot vs On-Demand Nodes
```promql
count(kube_cost_node_hourly_usd) by (is_spot)
```

## Cost Trend Queries

### Daily Cost Trend (Last 30 Days)
```promql
sum(rate(kube_cost_cluster_hourly_usd[1d])) * 24
```

### Week-over-Week Cost Change
```promql
(
  sum(kube_cost_cluster_hourly_usd) -
  sum(kube_cost_cluster_hourly_usd offset 7d)
) / sum(kube_cost_cluster_hourly_usd offset 7d) * 100
```

### Month-over-Month Cost Change
```promql
(
  sum(kube_cost_cluster_hourly_usd) -
  sum(kube_cost_cluster_hourly_usd offset 30d)
) / sum(kube_cost_cluster_hourly_usd offset 30d) * 100
```

## Resource Efficiency Queries

### Cost per CPU Core (Hourly)
```promql
sum(kube_cost_cluster_hourly_usd) / sum(kube_node_status_capacity{resource="cpu"})
```

### Cost per GB Memory (Hourly)
```promql
sum(kube_cost_cluster_hourly_usd) / (sum(kube_node_status_capacity{resource="memory"}) / 1024 / 1024 / 1024)
```

### Pods with No Resource Requests (Potential Cost Waste)
```promql
count(kube_pod_container_resource_requests{resource="cpu"} == 0)
```

## Budget and Forecasting Queries

### Projected End-of-Month Cost
```promql
sum(kube_cost_cluster_hourly_usd) * 730
```

### Cost Run Rate (Based on Last 24 Hours)
```promql
avg_over_time(sum(kube_cost_cluster_hourly_usd)[24h]) * 730
```

### Burn Rate (Hours Until Budget Exhausted)
```promql
<budget_amount> / sum(kube_cost_cluster_hourly_usd)
```

### Days Remaining in Budget
```promql
(<budget_amount> / sum(kube_cost_cluster_hourly_usd)) / 24
```

## Advanced Queries

### Cost Anomaly Detection (Deviation from 24h Average)
```promql
abs(
  sum(kube_cost_cluster_hourly_usd) -
  avg_over_time(sum(kube_cost_cluster_hourly_usd)[24h])
) / avg_over_time(sum(kube_cost_cluster_hourly_usd)[24h]) * 100
```

### Namespace Cost Efficiency (Cost per Pod)
```promql
sum(kube_cost_namespace_hourly_usd) by (namespace) /
count(kube_cost_pod_hourly_usd) by (namespace)
```

### Idle Resource Cost (Nodes with Low Utilization)
```promql
sum(kube_cost_node_hourly_usd) *
(1 - avg(node_cpu_seconds_total) by (node))
```

## Grafana Dashboard Variables

These queries are useful for creating dynamic Grafana dashboard variables:

### Namespace List
```promql
label_values(kube_cost_namespace_hourly_usd, namespace)
```

### Instance Type List
```promql
label_values(kube_cost_node_hourly_usd, instance_type)
```

### Node List
```promql
label_values(kube_cost_node_hourly_usd, node)
```

## Recording Rules

For better performance, consider creating these Prometheus recording rules:

```yaml
groups:
  - name: kube_cost_recording_rules
    interval: 60s
    rules:
      - record: namespace:kube_cost_monthly:sum
        expr: sum(kube_cost_namespace_daily_usd) by (namespace) * 30

      - record: cluster:kube_cost_monthly:sum
        expr: sum(kube_cost_cluster_hourly_usd) * 730

      - record: cluster:spot_savings_monthly:sum
        expr: sum(kube_cost_spot_savings_hourly_usd) * 730

      - record: cluster:spot_percentage:ratio
        expr: |
          (
            sum(kube_cost_spot_savings_hourly_usd) /
            (sum(kube_cost_cluster_hourly_usd) + sum(kube_cost_spot_savings_hourly_usd))
          ) * 100
```
