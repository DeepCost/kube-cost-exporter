## Terraform Module for Kube Cost Exporter

This Terraform module deploys Kube Cost Exporter to an AWS EKS cluster with proper IAM roles and permissions.

### Prerequisites

- Terraform >= 1.0
- AWS EKS cluster
- OIDC provider configured for the EKS cluster
- Prometheus installed in the cluster
- (Optional) Grafana for dashboards

### Usage

```hcl
module "kube_cost_exporter" {
  source = "./path/to/kube-cost-exporter/terraform"

  cluster_name      = "my-eks-cluster"
  aws_region        = "us-east-1"
  oidc_provider_arn = "arn:aws:iam::123456789012:oidc-provider/oidc.eks.us-east-1.amazonaws.com/id/EXAMPLED539D4633E53DE1B71EXAMPLE"

  # Optional: Enable cost budgets
  enable_cost_budgets = true
  namespace_budgets = {
    production = {
      monthly_limit   = 1000
      alert_threshold = 80
    }
    staging = {
      monthly_limit   = 500
      alert_threshold = 80
    }
  }

  # Optional: Enable cost alerts
  enable_cost_alerts = true

  # Optional: Custom resource limits
  resources = {
    requests = {
      cpu    = "200m"
      memory = "256Mi"
    }
    limits = {
      cpu    = "1000m"
      memory = "1Gi"
    }
  }

  tags = {
    Environment = "production"
    Team        = "platform"
  }
}
```

### Getting the OIDC Provider ARN

```bash
aws eks describe-cluster --name my-cluster --query "cluster.identity.oidc.issuer" --output text
```

Then construct the ARN:
```
arn:aws:iam::<account-id>:oidc-provider/<issuer-url-without-https>
```

### Inputs

| Name | Description | Type | Default | Required |
|------|-------------|------|---------|----------|
| cluster_name | Name of the EKS cluster | string | - | yes |
| aws_region | AWS region | string | us-east-1 | no |
| oidc_provider_arn | ARN of the OIDC provider | string | - | yes |
| chart_version | Helm chart version | string | 1.0.0 | no |
| update_interval | Cost update interval | string | 60s | no |
| enable_service_monitor | Enable ServiceMonitor | bool | true | no |
| enable_grafana_dashboard | Enable Grafana dashboard | bool | true | no |
| enable_cost_budgets | Enable cost budgets | bool | false | no |
| enable_cost_alerts | Enable cost alerts | bool | true | no |

### Outputs

| Name | Description |
|------|-------------|
| kube_cost_exporter_role_arn | IAM role ARN for the exporter |
| estimated_monthly_cost | Current estimated monthly cost |

### Examples

#### Minimal Configuration

```hcl
module "kube_cost_exporter" {
  source = "./terraform"

  cluster_name      = "my-cluster"
  oidc_provider_arn = var.oidc_provider_arn
}
```

#### With Cost Budgets

```hcl
module "kube_cost_exporter" {
  source = "./terraform"

  cluster_name      = "my-cluster"
  oidc_provider_arn = var.oidc_provider_arn

  enable_cost_budgets = true
  namespace_budgets = {
    production = {
      monthly_limit   = 5000
      alert_threshold = 90
    }
    development = {
      monthly_limit   = 500
      alert_threshold = 75
    }
  }
}
```

### Notes

- The module creates an IAM role with IRSA (IAM Roles for Service Accounts) for secure AWS API access
- ServiceMonitor is created automatically if Prometheus Operator is installed
- Cost alerts require Prometheus AlertManager to be configured
- The `estimated_monthly_cost` output requires Prometheus to be accessible from where Terraform runs
