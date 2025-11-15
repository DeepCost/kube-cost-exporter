package calculator

import (
	"fmt"

	"github.com/deepcost/kube-cost-exporter/pkg/collector"
	"github.com/sirupsen/logrus"
)

// CostCalculator calculates pod costs based on resource allocation
type CostCalculator struct {
	logger *logrus.Logger
}

// NewCostCalculator creates a new cost calculator
func NewCostCalculator() *CostCalculator {
	logger := logrus.New()
	logger.SetLevel(logrus.InfoLevel)

	return &CostCalculator{
		logger: logger,
	}
}

// PodCost represents the calculated cost for a pod
type PodCost struct {
	PodName      string
	Namespace    string
	NodeName     string
	HourlyCost   float64
	DailyCost    float64
	MonthlyCost  float64
	CPUCost      float64
	MemoryCost   float64
}

// NamespaceCost represents aggregated cost for a namespace
type NamespaceCost struct {
	Namespace   string
	HourlyCost  float64
	DailyCost   float64
	MonthlyCost float64
	PodCount    int
}

// CalculatePodCost calculates the cost of a pod based on its resource allocation
func (cc *CostCalculator) CalculatePodCost(pod collector.PodInfo, node collector.NodeInfo) (PodCost, error) {
	if node.CPUCapacity == 0 || node.MemoryCapacity == 0 {
		return PodCost{}, fmt.Errorf("node has zero capacity")
	}

	// Calculate CPU cost allocation
	// Pod Cost = (Pod Resource Request / Node Total Capacity) × Node Hourly Cost
	cpuFraction := float64(pod.CPURequest) / float64(node.CPUCapacity)
	memoryFraction := float64(pod.MemoryRequest) / float64(node.MemoryCapacity)

	// Use the maximum of CPU and memory fraction for more accurate cost allocation
	// This accounts for pods that are either CPU or memory bound
	resourceFraction := cpuFraction
	if memoryFraction > cpuFraction {
		resourceFraction = memoryFraction
	}

	// If no requests are set, use a minimal fraction
	if pod.CPURequest == 0 && pod.MemoryRequest == 0 {
		resourceFraction = 0.01 // Assign 1% of node cost
	}

	hourlyCost := node.HourlyPrice * resourceFraction

	// Calculate individual component costs for visibility
	cpuCost := node.HourlyPrice * cpuFraction
	memoryCost := node.HourlyPrice * memoryFraction

	return PodCost{
		PodName:     pod.Name,
		Namespace:   pod.Namespace,
		NodeName:    pod.NodeName,
		HourlyCost:  hourlyCost,
		DailyCost:   hourlyCost * 24,
		MonthlyCost: hourlyCost * 730, // Average hours per month
		CPUCost:     cpuCost,
		MemoryCost:  memoryCost,
	}, nil
}

// CalculateNamespaceCosts aggregates pod costs by namespace
func (cc *CostCalculator) CalculateNamespaceCosts(podCosts []PodCost) []NamespaceCost {
	namespaceMap := make(map[string]*NamespaceCost)

	for _, podCost := range podCosts {
		ns, exists := namespaceMap[podCost.Namespace]
		if !exists {
			ns = &NamespaceCost{
				Namespace: podCost.Namespace,
			}
			namespaceMap[podCost.Namespace] = ns
		}

		ns.HourlyCost += podCost.HourlyCost
		ns.DailyCost += podCost.DailyCost
		ns.MonthlyCost += podCost.MonthlyCost
		ns.PodCount++
	}

	// Convert map to slice
	var namespaceCosts []NamespaceCost
	for _, ns := range namespaceMap {
		namespaceCosts = append(namespaceCosts, *ns)
	}

	return namespaceCosts
}

// CalculateSpotSavings calculates the savings from using spot instances
func (cc *CostCalculator) CalculateSpotSavings(nodes []collector.NodeInfo) float64 {
	var totalSavings float64

	for _, node := range nodes {
		if node.IsSpot {
			// Estimate on-demand price (spot is typically 70% cheaper)
			estimatedOnDemand := node.HourlyPrice / 0.30
			savings := estimatedOnDemand - node.HourlyPrice
			totalSavings += savings
		}
	}

	return totalSavings
}

// CalculateTotalClusterCost calculates the total cluster cost
func (cc *CostCalculator) CalculateTotalClusterCost(nodes []collector.NodeInfo) float64 {
	var totalCost float64

	for _, node := range nodes {
		totalCost += node.HourlyPrice
	}

	return totalCost
}
