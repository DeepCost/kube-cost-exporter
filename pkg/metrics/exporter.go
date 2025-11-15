package metrics

import (
	"github.com/deepcost/kube-cost-exporter/pkg/calculator"
	"github.com/deepcost/kube-cost-exporter/pkg/collector"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/sirupsen/logrus"
)

// Exporter exports cost metrics for Prometheus
type Exporter struct {
	podHourlyCost       *prometheus.GaugeVec
	namespaceHourlyCost *prometheus.GaugeVec
	namespaceDailyCost  *prometheus.GaugeVec
	nodeHourlyCost          *prometheus.GaugeVec
	spotSavings             prometheus.Gauge
	clusterHourlyCost       prometheus.Gauge
	spotNodeCount           prometheus.Gauge
	onDemandNodeCount       prometheus.Gauge
	spotPercentage          prometheus.Gauge
	spotCostHourly          prometheus.Gauge
	onDemandCostHourly      prometheus.Gauge
	namespaceSpotUsage      *prometheus.GaugeVec
	namespaceSpotPercentage *prometheus.GaugeVec
	logger                  *logrus.Logger
}

// NewExporter creates a new metrics exporter
func NewExporter() *Exporter {
	logger := logrus.New()
	logger.SetLevel(logrus.InfoLevel)

	return &Exporter{
		podHourlyCost: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "kube_cost_pod_hourly_usd",
				Help: "Hourly cost of pod in USD",
			},
			[]string{"namespace", "pod", "node"},
		),
		namespaceHourlyCost: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "kube_cost_namespace_hourly_usd",
				Help: "Hourly cost per namespace in USD",
			},
			[]string{"namespace"},
		),
		namespaceDailyCost: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "kube_cost_namespace_daily_usd",
				Help: "Daily cost per namespace in USD",
			},
			[]string{"namespace"},
		),
		nodeHourlyCost: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "kube_cost_node_hourly_usd",
				Help: "Hourly cost per node in USD",
			},
			[]string{"node", "instance_type", "is_spot"},
		),
		spotSavings: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Name: "kube_cost_spot_savings_hourly_usd",
				Help: "Hourly savings from spot instances in USD",
			},
		),
		clusterHourlyCost: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Name: "kube_cost_cluster_hourly_usd",
				Help: "Total hourly cost of the cluster in USD",
			},
		),
		spotNodeCount: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Name: "kube_cost_spot_node_count",
				Help: "Number of spot/preemptible nodes",
			},
		),
		onDemandNodeCount: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Name: "kube_cost_ondemand_node_count",
				Help: "Number of on-demand nodes",
			},
		),
		spotPercentage: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Name: "kube_cost_spot_percentage",
				Help: "Percentage of nodes that are spot instances",
			},
		),
		spotCostHourly: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Name: "kube_cost_spot_hourly_usd",
				Help: "Hourly cost of spot instances in USD",
			},
		),
		onDemandCostHourly: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Name: "kube_cost_ondemand_hourly_usd",
				Help: "Hourly cost of on-demand instances in USD",
			},
		),
		namespaceSpotUsage: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "kube_cost_namespace_spot_pods",
				Help: "Number of pods on spot instances per namespace",
			},
			[]string{"namespace"},
		),
		namespaceSpotPercentage: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "kube_cost_namespace_spot_percentage",
				Help: "Percentage of namespace pods on spot instances",
			},
			[]string{"namespace"},
		),
		logger: logger,
	}
}

// Register registers all metrics with Prometheus
func (e *Exporter) Register(registry *prometheus.Registry) error {
	if err := registry.Register(e.podHourlyCost); err != nil {
		return err
	}
	if err := registry.Register(e.namespaceHourlyCost); err != nil {
		return err
	}
	if err := registry.Register(e.namespaceDailyCost); err != nil {
		return err
	}
	if err := registry.Register(e.nodeHourlyCost); err != nil {
		return err
	}
	if err := registry.Register(e.spotSavings); err != nil {
		return err
	}
	if err := registry.Register(e.clusterHourlyCost); err != nil {
		return err
	}
	if err := registry.Register(e.spotNodeCount); err != nil {
		return err
	}
	if err := registry.Register(e.onDemandNodeCount); err != nil {
		return err
	}
	if err := registry.Register(e.spotPercentage); err != nil {
		return err
	}
	if err := registry.Register(e.spotCostHourly); err != nil {
		return err
	}
	if err := registry.Register(e.onDemandCostHourly); err != nil {
		return err
	}
	if err := registry.Register(e.namespaceSpotUsage); err != nil {
		return err
	}
	if err := registry.Register(e.namespaceSpotPercentage); err != nil {
		return err
	}
	return nil
}

// UpdatePodMetrics updates pod cost metrics
func (e *Exporter) UpdatePodMetrics(podCosts []calculator.PodCost) {
	// Reset existing metrics
	e.podHourlyCost.Reset()

	for _, podCost := range podCosts {
		e.podHourlyCost.With(prometheus.Labels{
			"namespace": podCost.Namespace,
			"pod":       podCost.PodName,
			"node":      podCost.NodeName,
		}).Set(podCost.HourlyCost)
	}

	e.logger.Infof("Updated metrics for %d pods", len(podCosts))
}

// UpdateNamespaceMetrics updates namespace cost metrics
func (e *Exporter) UpdateNamespaceMetrics(namespaceCosts []calculator.NamespaceCost) {
	// Reset existing metrics
	e.namespaceHourlyCost.Reset()
	e.namespaceDailyCost.Reset()

	for _, nsCost := range namespaceCosts {
		e.namespaceHourlyCost.With(prometheus.Labels{
			"namespace": nsCost.Namespace,
		}).Set(nsCost.HourlyCost)

		e.namespaceDailyCost.With(prometheus.Labels{
			"namespace": nsCost.Namespace,
		}).Set(nsCost.DailyCost)
	}

	e.logger.Infof("Updated metrics for %d namespaces", len(namespaceCosts))
}

// UpdateNodeMetrics updates node cost metrics
func (e *Exporter) UpdateNodeMetrics(nodes []collector.NodeInfo) {
	// Reset existing metrics
	e.nodeHourlyCost.Reset()

	for _, node := range nodes {
		spotLabel := "false"
		if node.IsSpot {
			spotLabel = "true"
		}

		e.nodeHourlyCost.With(prometheus.Labels{
			"node":          node.Name,
			"instance_type": node.InstanceType,
			"is_spot":       spotLabel,
		}).Set(node.HourlyPrice)
	}

	e.logger.Infof("Updated metrics for %d nodes", len(nodes))
}

// UpdateClusterMetrics updates cluster-wide metrics
func (e *Exporter) UpdateClusterMetrics(totalCost, spotSavings float64) {
	e.clusterHourlyCost.Set(totalCost)
	e.spotSavings.Set(spotSavings)

	e.logger.Infof("Updated cluster metrics: total=$%.2f/hr, spot savings=$%.2f/hr", totalCost, spotSavings)
}

// UpdateDetailedSpotMetrics updates detailed spot instance metrics
func (e *Exporter) UpdateDetailedSpotMetrics(spotSavings calculator.SpotSavings) {
	e.spotSavings.Set(spotSavings.TotalSavingsHourly)
	e.spotNodeCount.Set(float64(spotSavings.SpotNodeCount))
	e.onDemandNodeCount.Set(float64(spotSavings.OnDemandNodeCount))
	e.spotPercentage.Set(spotSavings.SpotPercentage)
	e.spotCostHourly.Set(spotSavings.SpotCostHourly)
	e.onDemandCostHourly.Set(spotSavings.OnDemandCostHourly)

	e.logger.Infof("Updated detailed spot metrics: %d spot nodes (%.1f%%), $%.2f/hr savings",
		spotSavings.SpotNodeCount, spotSavings.SpotPercentage, spotSavings.TotalSavingsHourly)
}

// UpdateNamespaceSpotMetrics updates namespace-level spot usage metrics
func (e *Exporter) UpdateNamespaceSpotMetrics(namespaceSpotUsage []calculator.NamespaceSpotUsage) {
	// Reset existing metrics
	e.namespaceSpotUsage.Reset()
	e.namespaceSpotPercentage.Reset()

	for _, nsSpot := range namespaceSpotUsage {
		e.namespaceSpotUsage.With(prometheus.Labels{
			"namespace": nsSpot.Namespace,
		}).Set(float64(nsSpot.PodsOnSpot))

		e.namespaceSpotPercentage.With(prometheus.Labels{
			"namespace": nsSpot.Namespace,
		}).Set(nsSpot.SpotPercentage)
	}

	e.logger.Infof("Updated spot metrics for %d namespaces", len(namespaceSpotUsage))
}
