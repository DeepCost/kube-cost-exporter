package collector

import (
	"context"
	"fmt"
	"strings"

	"github.com/deepcost/kube-cost-exporter/pkg/pricing"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// NodeCollector collects node information and pricing
type NodeCollector struct {
	clientset      *kubernetes.Clientset
	pricingCache   *pricing.PricingCache
	cloudProvider  string
	region         string
	logger         *logrus.Logger
}

// NewNodeCollector creates a new node collector
func NewNodeCollector(clientset *kubernetes.Clientset, pricingCache *pricing.PricingCache, cloudProvider, region string) *NodeCollector {
	logger := logrus.New()
	logger.SetLevel(logrus.InfoLevel)

	return &NodeCollector{
		clientset:     clientset,
		pricingCache:  pricingCache,
		cloudProvider: cloudProvider,
		region:        region,
		logger:        logger,
	}
}

// NodeInfo contains information about a node and its pricing
type NodeInfo struct {
	Name             string
	InstanceType     string
	Region           string
	AvailabilityZone string
	IsSpot           bool
	HourlyPrice      float64
	CPUCapacity      int64  // millicores
	MemoryCapacity   int64  // bytes
	Labels           map[string]string
}

// CollectNodes collects all nodes and their pricing information
func (nc *NodeCollector) CollectNodes(ctx context.Context) ([]NodeInfo, error) {
	nodes, err := nc.clientset.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to list nodes: %w", err)
	}

	var nodeInfos []NodeInfo
	for _, node := range nodes.Items {
		nodeInfo, err := nc.collectNodeInfo(ctx, &node)
		if err != nil {
			nc.logger.Warnf("Failed to collect info for node %s: %v", node.Name, err)
			continue
		}
		nodeInfos = append(nodeInfos, nodeInfo)
	}

	return nodeInfos, nil
}

// collectNodeInfo extracts pricing information for a single node
func (nc *NodeCollector) collectNodeInfo(ctx context.Context, node *corev1.Node) (NodeInfo, error) {
	instanceType := nc.getInstanceType(node)
	region := nc.getRegion(node)
	az := nc.getAvailabilityZone(node)
	isSpot := nc.isSpotInstance(node)

	// Get pricing
	var hourlyPrice float64
	var err error

	if isSpot {
		hourlyPrice, err = nc.pricingCache.GetSpotPrice(ctx, instanceType, region, az)
	} else {
		hourlyPrice, err = nc.pricingCache.GetInstancePrice(ctx, instanceType, region, az)
	}

	if err != nil {
		nc.logger.Warnf("Failed to get price for node %s: %v", node.Name, err)
		hourlyPrice = 0.0
	}

	// Get capacity
	cpuCapacity := node.Status.Capacity.Cpu().MilliValue()
	memoryCapacity := node.Status.Capacity.Memory().Value()

	return NodeInfo{
		Name:             node.Name,
		InstanceType:     instanceType,
		Region:           region,
		AvailabilityZone: az,
		IsSpot:           isSpot,
		HourlyPrice:      hourlyPrice,
		CPUCapacity:      cpuCapacity,
		MemoryCapacity:   memoryCapacity,
		Labels:           node.Labels,
	}, nil
}

// getInstanceType extracts the instance type from node labels
func (nc *NodeCollector) getInstanceType(node *corev1.Node) string {
	// Try different label keys used by different cloud providers
	labelKeys := []string{
		"node.kubernetes.io/instance-type",
		"beta.kubernetes.io/instance-type",
		"kubernetes.io/instance-type",
	}

	for _, key := range labelKeys {
		if instanceType, ok := node.Labels[key]; ok {
			return instanceType
		}
	}

	// Fallback: try to parse from provider ID
	if node.Spec.ProviderID != "" {
		parts := strings.Split(node.Spec.ProviderID, "/")
		if len(parts) > 0 {
			return parts[len(parts)-1]
		}
	}

	return "unknown"
}

// getRegion extracts the region from node labels
func (nc *NodeCollector) getRegion(node *corev1.Node) string {
	labelKeys := []string{
		"topology.kubernetes.io/region",
		"failure-domain.beta.kubernetes.io/region",
		"kubernetes.io/region",
	}

	for _, key := range labelKeys {
		if region, ok := node.Labels[key]; ok {
			return region
		}
	}

	return nc.region // Use configured region as fallback
}

// getAvailabilityZone extracts the AZ from node labels
func (nc *NodeCollector) getAvailabilityZone(node *corev1.Node) string {
	labelKeys := []string{
		"topology.kubernetes.io/zone",
		"failure-domain.beta.kubernetes.io/zone",
		"kubernetes.io/zone",
	}

	for _, key := range labelKeys {
		if az, ok := node.Labels[key]; ok {
			return az
		}
	}

	return ""
}

// isSpotInstance determines if a node is a spot/preemptible instance
func (nc *NodeCollector) isSpotInstance(node *corev1.Node) bool {
	// Check various labels that indicate spot instances
	spotLabels := []string{
		"karpenter.sh/capacity-type",
		"eks.amazonaws.com/capacityType",
		"cloud.google.com/gke-preemptible",
		"kubernetes.azure.com/scalesetpriority",
	}

	for _, label := range spotLabels {
		if value, ok := node.Labels[label]; ok {
			lowerValue := strings.ToLower(value)
			if strings.Contains(lowerValue, "spot") ||
				strings.Contains(lowerValue, "preemptible") ||
				lowerValue == "spot" ||
				lowerValue == "true" {
				return true
			}
		}
	}

	return false
}
