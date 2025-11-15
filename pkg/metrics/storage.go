package metrics

import (
	"github.com/deepcost/kube-cost-exporter/pkg/calculator"
	"github.com/prometheus/client_golang/prometheus"
)

// StorageMetrics contains storage-related Prometheus metrics
type StorageMetrics struct {
	pvMonthlyCost           *prometheus.GaugeVec
	namespaceStorageCost    *prometheus.GaugeVec
	clusterStorageCost      prometheus.Gauge
	storageClassCost        *prometheus.GaugeVec
}

// NewStorageMetrics creates new storage metrics
func NewStorageMetrics() *StorageMetrics {
	return &StorageMetrics{
		pvMonthlyCost: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "kube_cost_pv_monthly_usd",
				Help: "Monthly cost of persistent volume in USD",
			},
			[]string{"pv_name", "namespace", "pvc_name", "storage_class"},
		),
		namespaceStorageCost: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "kube_cost_namespace_storage_monthly_usd",
				Help: "Monthly storage cost per namespace in USD",
			},
			[]string{"namespace"},
		),
		clusterStorageCost: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Name: "kube_cost_cluster_storage_monthly_usd",
				Help: "Total monthly storage cost of the cluster in USD",
			},
		),
		storageClassCost: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "kube_cost_storage_class_monthly_usd",
				Help: "Monthly cost by storage class in USD",
			},
			[]string{"storage_class"},
		),
	}
}

// Register registers storage metrics with Prometheus
func (sm *StorageMetrics) Register(registry *prometheus.Registry) error {
	if err := registry.Register(sm.pvMonthlyCost); err != nil {
		return err
	}
	if err := registry.Register(sm.namespaceStorageCost); err != nil {
		return err
	}
	if err := registry.Register(sm.clusterStorageCost); err != nil {
		return err
	}
	if err := registry.Register(sm.storageClassCost); err != nil {
		return err
	}
	return nil
}

// UpdatePVMetrics updates PV cost metrics
func (sm *StorageMetrics) UpdatePVMetrics(storageCosts []calculator.StorageCost) {
	sm.pvMonthlyCost.Reset()
	sm.storageClassCost.Reset()

	storageClassTotals := make(map[string]float64)

	for _, cost := range storageCosts {
		sm.pvMonthlyCost.With(prometheus.Labels{
			"pv_name":       cost.PVName,
			"namespace":     cost.Namespace,
			"pvc_name":      cost.PVCName,
			"storage_class": cost.StorageClass,
		}).Set(cost.MonthlyCost)

		storageClassTotals[cost.StorageClass] += cost.MonthlyCost
	}

	// Update storage class totals
	for storageClass, total := range storageClassTotals {
		sm.storageClassCost.With(prometheus.Labels{
			"storage_class": storageClass,
		}).Set(total)
	}
}

// UpdateNamespaceStorageMetrics updates namespace storage cost metrics
func (sm *StorageMetrics) UpdateNamespaceStorageMetrics(namespaceCosts []calculator.NamespaceStorageCost) {
	sm.namespaceStorageCost.Reset()

	for _, nsCost := range namespaceCosts {
		sm.namespaceStorageCost.With(prometheus.Labels{
			"namespace": nsCost.Namespace,
		}).Set(nsCost.MonthlyCost)
	}
}

// UpdateClusterStorageMetrics updates cluster storage metrics
func (sm *StorageMetrics) UpdateClusterStorageMetrics(totalCost float64) {
	sm.clusterStorageCost.Set(totalCost)
}
