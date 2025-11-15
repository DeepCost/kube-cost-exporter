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

// SpotSavings contains detailed spot instance savings information
type SpotSavings struct {
	TotalSavingsHourly   float64
	TotalSavingsMonthly  float64
	SpotNodeCount        int
	OnDemandNodeCount    int
	SpotPercentage       float64
	SpotCostHourly       float64
	OnDemandCostHourly   float64
	EstimatedSavingsRate float64 // Percentage saved by using spot
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

// CalculateDetailedSpotSavings provides comprehensive spot instance savings analysis
func (cc *CostCalculator) CalculateDetailedSpotSavings(nodes []collector.NodeInfo) SpotSavings {
	var spotCost, onDemandCost, totalEstimatedOnDemand float64
	var spotCount, onDemandCount int

	for _, node := range nodes {
		if node.IsSpot {
			spotCost += node.HourlyPrice
			spotCount++
			// Estimate what it would cost on on-demand
			// Spot is typically 70% cheaper (30% of on-demand price)
			estimatedOnDemand := node.HourlyPrice / 0.30
			totalEstimatedOnDemand += estimatedOnDemand
		} else {
			onDemandCost += node.HourlyPrice
			onDemandCount++
		}
	}

	totalNodes := spotCount + onDemandCount
	spotPercentage := 0.0
	if totalNodes > 0 {
		spotPercentage = (float64(spotCount) / float64(totalNodes)) * 100
	}

	totalSavings := totalEstimatedOnDemand - spotCost
	savingsRate := 0.0
	if totalEstimatedOnDemand > 0 {
		savingsRate = (totalSavings / totalEstimatedOnDemand) * 100
	}

	return SpotSavings{
		TotalSavingsHourly:   totalSavings,
		TotalSavingsMonthly:  totalSavings * 730,
		SpotNodeCount:        spotCount,
		OnDemandNodeCount:    onDemandCount,
		SpotPercentage:       spotPercentage,
		SpotCostHourly:       spotCost,
		OnDemandCostHourly:   onDemandCost,
		EstimatedSavingsRate: savingsRate,
	}
}

// NamespaceSpotUsage contains spot instance usage for a namespace
type NamespaceSpotUsage struct {
	Namespace     string
	PodsOnSpot    int
	PodsOnDemand  int
	SpotCost      float64
	OnDemandCost  float64
	SpotPercentage float64
}

// CalculateNamespaceSpotUsage calculates spot instance usage per namespace
func (cc *CostCalculator) CalculateNamespaceSpotUsage(podCosts []PodCost, nodes []collector.NodeInfo) []NamespaceSpotUsage {
	// Create node lookup map
	nodeMap := make(map[string]bool)
	for _, node := range nodes {
		nodeMap[node.Name] = node.IsSpot
	}

	// Track by namespace
	nsMap := make(map[string]*NamespaceSpotUsage)

	for _, pod := range podCosts {
		isSpot := nodeMap[pod.NodeName]

		ns, exists := nsMap[pod.Namespace]
		if !exists {
			ns = &NamespaceSpotUsage{
				Namespace: pod.Namespace,
			}
			nsMap[pod.Namespace] = ns
		}

		if isSpot {
			ns.PodsOnSpot++
			ns.SpotCost += pod.HourlyCost
		} else {
			ns.PodsOnDemand++
			ns.OnDemandCost += pod.HourlyCost
		}
	}

	// Calculate percentages and convert to slice
	var result []NamespaceSpotUsage
	for _, ns := range nsMap {
		totalPods := ns.PodsOnSpot + ns.PodsOnDemand
		if totalPods > 0 {
			ns.SpotPercentage = (float64(ns.PodsOnSpot) / float64(totalPods)) * 100
		}
		result = append(result, *ns)
	}

	return result
}

// CalculateTotalClusterCost calculates the total cluster cost
func (cc *CostCalculator) CalculateTotalClusterCost(nodes []collector.NodeInfo) float64 {
	var totalCost float64

	for _, node := range nodes {
		totalCost += node.HourlyPrice
	}

	return totalCost
}
