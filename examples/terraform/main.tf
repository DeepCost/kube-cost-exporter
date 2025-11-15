terraform {
  required_version = ">= 1.0"

  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 5.0"
    }
    kubernetes = {
      source  = "hashicorp/kubernetes"
      version = "~> 2.20"
    }
    helm = {
      source  = "hashicorp/helm"
      version = "~> 2.10"
    }
  }
}

provider "aws" {
  region = var.aws_region
}

provider "kubernetes" {
  host                   = data.aws_eks_cluster.cluster.endpoint
  cluster_ca_certificate = base64decode(data.aws_eks_cluster.cluster.certificate_authority[0].data)
  token                  = data.aws_eks_cluster_auth.cluster.token
}

provider "helm" {
  kubernetes {
    host                   = data.aws_eks_cluster.cluster.endpoint
    cluster_ca_certificate = base64decode(data.aws_eks_cluster.cluster.certificate_authority[0].data)
    token                  = data.aws_eks_cluster_auth.cluster.token
  }
}

data "aws_eks_cluster" "cluster" {
  name = var.cluster_name
}

data "aws_eks_cluster_auth" "cluster" {
  name = var.cluster_name
}

# IAM Role for Kube Cost Exporter (IRSA)
resource "aws_iam_role" "kube_cost_exporter" {
  name = "${var.cluster_name}-kube-cost-exporter"

  assume_role_policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Effect = "Allow"
        Principal = {
          Federated = var.oidc_provider_arn
        }
        Action = "sts:AssumeRoleWithWebIdentity"
        Condition = {
          StringEquals = {
            "${replace(var.oidc_provider_arn, "/^(.*provider/)/", "")}:sub" = "system:serviceaccount:kube-system:kube-cost-exporter"
            "${replace(var.oidc_provider_arn, "/^(.*provider/)/", "")}:aud" = "sts.amazonaws.com"
          }
        }
      }
    ]
  })

  tags = merge(
    var.tags,
    {
      Name = "${var.cluster_name}-kube-cost-exporter"
    }
  )
}

resource "aws_iam_role_policy" "kube_cost_exporter" {
  name = "kube-cost-exporter-policy"
  role = aws_iam_role.kube_cost_exporter.id

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Effect = "Allow"
        Action = [
          "ec2:DescribeSpotPriceHistory",
          "ec2:DescribeInstances",
          "ec2:DescribeRegions",
          "ec2:DescribeAvailabilityZones",
          "pricing:GetProducts"
        ]
        Resource = "*"
      }
    ]
  })
}

# Deploy Kube Cost Exporter via Helm
resource "helm_release" "kube_cost_exporter" {
  name       = "kube-cost-exporter"
  repository = "https://charts.deepcost.ai"
  chart      = "kube-cost-exporter"
  version    = var.chart_version
  namespace  = "kube-system"

  values = [
    yamlencode({
      cloudProvider = "aws"

      aws = {
        region      = var.aws_region
        irsaRoleArn = aws_iam_role.kube_cost_exporter.arn
      }

      updateInterval = var.update_interval

      serviceAccount = {
        create = true
        annotations = {
          "eks.amazonaws.com/role-arn" = aws_iam_role.kube_cost_exporter.arn
        }
      }

      resources = var.resources

      serviceMonitor = {
        enabled  = var.enable_service_monitor
        interval = var.scrape_interval
      }

      grafanaDashboard = {
        enabled = var.enable_grafana_dashboard
      }
    })
  ]

  depends_on = [
    aws_iam_role_policy.kube_cost_exporter
  ]
}

# Optional: Create ConfigMap for custom cost budgets
resource "kubernetes_config_map" "cost_budgets" {
  count = var.enable_cost_budgets ? 1 : 0

  metadata {
    name      = "kube-cost-budgets"
    namespace = "kube-system"
    labels = {
      "app.kubernetes.io/name" = "kube-cost-exporter"
    }
  }

  data = {
    "budgets.yaml" = yamlencode({
      budgets = var.namespace_budgets
    })
  }
}

# Optional: Create Prometheus alerts
resource "kubernetes_config_map" "cost_alerts" {
  count = var.enable_cost_alerts ? 1 : 0

  metadata {
    name      = "kube-cost-alerts"
    namespace = "monitoring"
    labels = {
      "prometheus" = "kube-prometheus"
    }
  }

  data = {
    "kube-cost-alerts.yaml" = file("${path.module}/../../prometheus-alerts.yaml")
  }
}

# Data source to query Prometheus for cost metrics
data "external" "cluster_cost" {
  program = ["bash", "-c", <<-EOT
    COST=$(curl -s "http://prometheus.monitoring.svc.cluster.local:9090/api/v1/query?query=sum(kube_cost_cluster_hourly_usd)*730" | jq -r '.data.result[0].value[1]')
    echo "{\"monthly_cost\": \"$COST\"}"
  EOT
  ]

  depends_on = [
    helm_release.kube_cost_exporter
  ]
}

output "kube_cost_exporter_role_arn" {
  description = "ARN of the IAM role for Kube Cost Exporter"
  value       = aws_iam_role.kube_cost_exporter.arn
}

output "estimated_monthly_cost" {
  description = "Estimated monthly cluster cost (requires Prometheus to be accessible)"
  value       = try(data.external.cluster_cost.result.monthly_cost, "N/A")
}
