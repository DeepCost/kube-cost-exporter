module github.com/deepcost/kube-cost-exporter

go 1.21

require (
	github.com/aws/aws-sdk-go-v2 v1.24.0
	github.com/aws/aws-sdk-go-v2/config v1.26.1
	github.com/aws/aws-sdk-go-v2/service/ec2 v1.141.0
	github.com/aws/aws-sdk-go-v2/service/pricing v1.26.0
	github.com/prometheus/client_golang v1.18.0
	github.com/prometheus/client_model v0.5.0
	github.com/prometheus/common v0.45.0
	github.com/sirupsen/logrus v1.9.3
	google.golang.org/api v0.159.0
	k8s.io/api v0.29.0
	k8s.io/apimachinery v0.29.0
	k8s.io/client-go v0.29.0
	k8s.io/metrics v0.29.0
)
