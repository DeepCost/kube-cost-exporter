variable "cluster_name" {
  description = "Name of the EKS cluster"
  type        = string
}

variable "aws_region" {
  description = "AWS region"
  type        = string
  default     = "us-east-1"
}

variable "oidc_provider_arn" {
  description = "ARN of the OIDC provider for EKS"
  type        = string
}

variable "chart_version" {
  description = "Version of the Kube Cost Exporter Helm chart"
  type        = string
  default     = "1.0.0"
}

variable "update_interval" {
  description = "How often to update cost metrics"
  type        = string
  default     = "60s"
}

variable "scrape_interval" {
  description = "Prometheus scrape interval"
  type        = string
  default     = "60s"
}

variable "enable_service_monitor" {
  description = "Enable Prometheus ServiceMonitor"
  type        = bool
  default     = true
}

variable "enable_grafana_dashboard" {
  description = "Enable Grafana dashboard"
  type        = bool
  default     = true
}

variable "enable_cost_budgets" {
  description = "Enable cost budget configuration"
  type        = bool
  default     = false
}

variable "enable_cost_alerts" {
  description = "Enable Prometheus cost alerts"
  type        = bool
  default     = true
}

variable "namespace_budgets" {
  description = "Cost budgets per namespace"
  type = map(object({
    monthly_limit = number
    alert_threshold = number
  }))
  default = {}
}

variable "resources" {
  description = "Resource requests and limits for the exporter"
  type = object({
    requests = object({
      cpu    = string
      memory = string
    })
    limits = object({
      cpu    = string
      memory = string
    })
  })
  default = {
    requests = {
      cpu    = "100m"
      memory = "128Mi"
    }
    limits = {
      cpu    = "500m"
      memory = "512Mi"
    }
  }
}

variable "tags" {
  description = "AWS resource tags"
  type        = map(string)
  default     = {}
}
