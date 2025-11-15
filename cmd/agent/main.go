package main

import (
	"context"
	"flag"
	"net/http"
	"os"
	"time"

	"github.com/deepcost/kube-cost-exporter/pkg/calculator"
	"github.com/deepcost/kube-cost-exporter/pkg/collector"
	"github.com/deepcost/kube-cost-exporter/pkg/metrics"
	"github.com/deepcost/kube-cost-exporter/pkg/pricing"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/sirupsen/logrus"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

var (
	kubeconfig     = flag.String("kubeconfig", "", "Path to kubeconfig file (optional, uses in-cluster config by default)")
	cloudProvider  = flag.String("cloud-provider", "aws", "Cloud provider (aws, gcp, azure)")
	region         = flag.String("region", "us-east-1", "Cloud provider region")
	metricsPort    = flag.String("metrics-port", "9090", "Port to expose metrics on")
	updateInterval = flag.Duration("update-interval", 60*time.Second, "Interval to update cost metrics")
	logger         = logrus.New()
)

func main() {
	flag.Parse()

	logger.SetLevel(logrus.InfoLevel)
	logger.SetFormatter(&logrus.JSONFormatter{})
	logger.Info("Starting Kube Cost Exporter Agent")

	// Create Kubernetes client
	config, err := getKubeConfig()
	if err != nil {
		logger.Fatalf("Failed to get Kubernetes config: %v", err)
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		logger.Fatalf("Failed to create Kubernetes client: %v", err)
	}

	// Initialize pricing provider
	var pricingProvider pricing.Provider
	switch *cloudProvider {
	case "aws":
		pricingProvider, err = pricing.NewAWSProvider(*region)
		if err != nil {
			logger.Fatalf("Failed to create AWS pricing provider: %v", err)
		}
	case "gcp":
		project := os.Getenv("GCP_PROJECT")
		if project == "" {
			logger.Fatal("GCP_PROJECT environment variable is required for GCP provider")
		}
		pricingProvider, err = pricing.NewGCPProvider(project)
		if err != nil {
			logger.Fatalf("Failed to create GCP pricing provider: %v", err)
		}
	case "azure":
		subscriptionID := os.Getenv("AZURE_SUBSCRIPTION_ID")
		if subscriptionID == "" {
			logger.Fatal("AZURE_SUBSCRIPTION_ID environment variable is required for Azure provider")
		}
		pricingProvider, err = pricing.NewAzureProvider(subscriptionID)
		if err != nil {
			logger.Fatalf("Failed to create Azure pricing provider: %v", err)
		}
	default:
		logger.Fatalf("Unknown cloud provider: %s", *cloudProvider)
	}

	// Wrap provider with caching
	pricingCache := pricing.NewPricingCache(pricingProvider)

	// Initialize collectors
	nodeCollector := collector.NewNodeCollector(clientset, pricingCache, *cloudProvider, *region)
	podCollector := collector.NewPodCollector(clientset)
	storageCollector := collector.NewStorageCollector(clientset, pricingCache, *cloudProvider, *region)

	// Initialize calculator and metrics exporter
	calc := calculator.NewCostCalculator()
	exporter := metrics.NewExporter()
	storageMetrics := metrics.NewStorageMetrics()

	// Create custom registry
	registry := prometheus.NewRegistry()
	if err := exporter.Register(registry); err != nil {
		logger.Fatalf("Failed to register metrics: %v", err)
	}
	if err := storageMetrics.Register(registry); err != nil {
		logger.Fatalf("Failed to register storage metrics: %v", err)
	}

	// Start metrics HTTP server
	go func() {
		http.Handle("/metrics", promhttp.HandlerFor(registry, promhttp.HandlerOpts{}))
		http.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("OK"))
		})

		logger.Infof("Starting metrics server on :%s", *metricsPort)
		if err := http.ListenAndServe(":"+*metricsPort, nil); err != nil {
			logger.Fatalf("Failed to start metrics server: %v", err)
		}
	}()

	// Start cost collection loop
	ctx := context.Background()
	ticker := time.NewTicker(*updateInterval)
	defer ticker.Stop()

	// Run immediately on startup
	collectAndExportMetrics(ctx, nodeCollector, podCollector, storageCollector, calc, exporter, storageMetrics)

	// Then run on schedule
	for range ticker.C {
		collectAndExportMetrics(ctx, nodeCollector, podCollector, storageCollector, calc, exporter, storageMetrics)
	}
}

func collectAndExportMetrics(
	ctx context.Context,
	nodeCollector *collector.NodeCollector,
	podCollector *collector.PodCollector,
	storageCollector *collector.StorageCollector,
	calc *calculator.CostCalculator,
	exporter *metrics.Exporter,
	storageMetrics *metrics.StorageMetrics,
) {
	logger.Info("Collecting cost metrics...")

	// Collect nodes
	nodes, err := nodeCollector.CollectNodes(ctx)
	if err != nil {
		logger.Errorf("Failed to collect nodes: %v", err)
		return
	}
	logger.Infof("Collected %d nodes", len(nodes))

	// Collect pods
	pods, err := podCollector.CollectPods(ctx)
	if err != nil {
		logger.Errorf("Failed to collect pods: %v", err)
		return
	}
	logger.Infof("Collected %d pods", len(pods))

	// Calculate pod costs
	nodeMap := make(map[string]collector.NodeInfo)
	for _, node := range nodes {
		nodeMap[node.Name] = node
	}

	var podCosts []calculator.PodCost
	for _, pod := range pods {
		node, exists := nodeMap[pod.NodeName]
		if !exists {
			logger.Warnf("Node %s not found for pod %s/%s", pod.NodeName, pod.Namespace, pod.Name)
			continue
		}

		podCost, err := calc.CalculatePodCost(pod, node)
		if err != nil {
			logger.Warnf("Failed to calculate cost for pod %s/%s: %v", pod.Namespace, pod.Name, err)
			continue
		}

		podCosts = append(podCosts, podCost)
	}

	// Calculate namespace costs
	namespaceCosts := calc.CalculateNamespaceCosts(podCosts)

	// Calculate cluster metrics
	totalCost := calc.CalculateTotalClusterCost(nodes)
	detailedSpotSavings := calc.CalculateDetailedSpotSavings(nodes)
	namespaceSpotUsage := calc.CalculateNamespaceSpotUsage(podCosts, nodes)

	// Collect storage (PVs)
	pvs, err := storageCollector.CollectPVs(ctx)
	if err != nil {
		logger.Warnf("Failed to collect storage: %v", err)
	} else {
		logger.Infof("Collected %d persistent volumes", len(pvs))

		// Calculate storage costs
		var storageCosts []calculator.StorageCost
		for _, pv := range pvs {
			cost := calc.CalculateStorageCost(pv)
			storageCosts = append(storageCosts, cost)
		}

		// Calculate namespace storage costs
		namespaceStorageCosts := calc.CalculateNamespaceStorageCosts(storageCosts)
		totalStorageCost := calc.CalculateTotalStorageCost(pvs)

		// Update storage metrics
		storageMetrics.UpdatePVMetrics(storageCosts)
		storageMetrics.UpdateNamespaceStorageMetrics(namespaceStorageCosts)
		storageMetrics.UpdateClusterStorageMetrics(totalStorageCost)

		logger.Infof("Storage metrics updated. Total monthly storage cost: $%.2f", totalStorageCost)
	}

	// Update Prometheus metrics
	exporter.UpdatePodMetrics(podCosts)
	exporter.UpdateNamespaceMetrics(namespaceCosts)
	exporter.UpdateNodeMetrics(nodes)
	exporter.UpdateClusterMetrics(totalCost, detailedSpotSavings.TotalSavingsHourly)
	exporter.UpdateDetailedSpotMetrics(detailedSpotSavings)
	exporter.UpdateNamespaceSpotMetrics(namespaceSpotUsage)

	logger.Infof("Metrics updated successfully. Cluster hourly cost: $%.2f, spot savings: $%.2f/hr",
		totalCost, detailedSpotSavings.TotalSavingsHourly)
}

func getKubeConfig() (*rest.Config, error) {
	if *kubeconfig != "" {
		// Use kubeconfig file
		return clientcmd.BuildConfigFromFlags("", *kubeconfig)
	}

	// Use in-cluster config
	return rest.InClusterConfig()
}
