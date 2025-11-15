package calculator

import (
	"github.com/deepcost/kube-cost-exporter/pkg/collector"
)

// StorageCost represents the calculated cost for storage
type StorageCost struct {
	PVName       string
	Namespace    string
	PVCName      string
	StorageClass string
	SizeGB       int64
	MonthlyCost  float64
	DailyCost    float64
	HourlyCost   float64
}

// NamespaceStorageCost represents aggregated storage cost for a namespace
type NamespaceStorageCost struct {
	Namespace   string
	TotalSizeGB int64
	MonthlyCost float64
	DailyCost   float64
	PVCount     int
}

// CalculateStorageCost calculates the cost for a persistent volume
func (cc *CostCalculator) CalculateStorageCost(pvInfo collector.PVInfo) StorageCost {
	monthlyCost := pvInfo.MonthlyCost
	dailyCost := monthlyCost / 30
	hourlyCost := monthlyCost / 730 // Average hours per month

	return StorageCost{
		PVName:       pvInfo.Name,
		Namespace:    pvInfo.Namespace,
		PVCName:      pvInfo.PVCName,
		StorageClass: pvInfo.StorageClass,
		SizeGB:       pvInfo.SizeGB,
		MonthlyCost:  monthlyCost,
		DailyCost:    dailyCost,
		HourlyCost:   hourlyCost,
	}
}

// CalculateNamespaceStorageCosts aggregates storage costs by namespace
func (cc *CostCalculator) CalculateNamespaceStorageCosts(storageCosts []StorageCost) []NamespaceStorageCost {
	namespaceMap := make(map[string]*NamespaceStorageCost)

	for _, cost := range storageCosts {
		if cost.Namespace == "" {
			continue // Skip unbound PVs
		}

		ns, exists := namespaceMap[cost.Namespace]
		if !exists {
			ns = &NamespaceStorageCost{
				Namespace: cost.Namespace,
			}
			namespaceMap[cost.Namespace] = ns
		}

		ns.TotalSizeGB += cost.SizeGB
		ns.MonthlyCost += cost.MonthlyCost
		ns.DailyCost += cost.DailyCost
		ns.PVCount++
	}

	// Convert map to slice
	var namespaceCosts []NamespaceStorageCost
	for _, ns := range namespaceMap {
		namespaceCosts = append(namespaceCosts, *ns)
	}

	return namespaceCosts
}

// CalculateTotalStorageCost calculates total cluster storage cost
func (cc *CostCalculator) CalculateTotalStorageCost(pvInfos []collector.PVInfo) float64 {
	var totalCost float64

	for _, pv := range pvInfos {
		totalCost += pv.MonthlyCost
	}

	return totalCost
}
