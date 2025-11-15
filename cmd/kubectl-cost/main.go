package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/prometheus/client_golang/api"
	v1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"
	"gopkg.in/yaml.v3"
)

var (
	prometheusURL = flag.String("prometheus-url", "http://localhost:9090", "Prometheus server URL")
	window        = flag.String("window", "24h", "Time window for cost calculation (e.g., 1h, 24h, 7d, 30d)")
)

func main() {
	flag.Parse()

	if len(flag.Args()) < 1 {
		printUsage()
		os.Exit(1)
	}

	command := flag.Args()[0]

	client, err := api.NewClient(api.Config{
		Address: *prometheusURL,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating Prometheus client: %v\n", err)
		os.Exit(1)
	}

	v1api := v1.NewAPI(client)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	switch command {
	case "namespace":
		if len(flag.Args()) < 2 {
			fmt.Fprintln(os.Stderr, "Usage: kubectl cost namespace <namespace-name|--all>")
			os.Exit(1)
		}
		target := flag.Args()[1]
		if err := showNamespaceCost(ctx, v1api, target); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

	case "pod":
		if len(flag.Args()) < 2 {
			fmt.Fprintln(os.Stderr, "Usage: kubectl cost pod <pod-name> [--namespace <namespace>]")
			os.Exit(1)
		}
		podName := flag.Args()[1]
		namespace := "default"
		if len(flag.Args()) >= 4 && flag.Args()[2] == "--namespace" {
			namespace = flag.Args()[3]
		}
		if err := showPodCost(ctx, v1api, namespace, podName); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

	case "node":
		if err := showNodeCost(ctx, v1api); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

	case "cluster":
		if err := showClusterCost(ctx, v1api); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

	case "top":
		resource := "pods"
		if len(flag.Args()) >= 2 {
			resource = flag.Args()[1]
		}
		if err := showTopCosts(ctx, v1api, resource); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

	case "estimate":
		var filename string
		for i, arg := range flag.Args() {
			if arg == "-f" || arg == "--file" {
				if i+1 < len(flag.Args()) {
					filename = flag.Args()[i+1]
					break
				}
			}
		}
		if filename == "" {
			fmt.Fprintln(os.Stderr, "Usage: kubectl cost estimate -f <manifest-file>")
			os.Exit(1)
		}
		if err := estimateCost(filename); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n", command)
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println("kubectl cost - Query Kubernetes cost data")
	fmt.Println()
	fmt.Println("Usage:")
	fmt.Println("  kubectl cost namespace <name|--all> [--window <duration>]")
	fmt.Println("  kubectl cost pod <name> [--namespace <namespace>] [--window <duration>]")
	fmt.Println("  kubectl cost node [--window <duration>]")
	fmt.Println("  kubectl cost cluster [--window <duration>]")
	fmt.Println("  kubectl cost top <pods|namespaces|nodes> [--window <duration>]")
	fmt.Println("  kubectl cost estimate -f <manifest-file>")
	fmt.Println()
	fmt.Println("Options:")
	fmt.Println("  --prometheus-url <url>    Prometheus server URL (default: http://localhost:9090)")
	fmt.Println("  --window <duration>       Time window (default: 24h)")
	fmt.Println("  -f, --file <path>         Manifest file to estimate (for estimate command)")
	fmt.Println()
	fmt.Println("Examples:")
	fmt.Println("  kubectl cost namespace production --window 30d")
	fmt.Println("  kubectl cost pod my-pod --namespace default")
	fmt.Println("  kubectl cost cluster")
	fmt.Println("  kubectl cost top namespaces")
	fmt.Println("  kubectl cost estimate -f deployment.yaml")
}

func showNamespaceCost(ctx context.Context, api v1.API, namespace string) error {
	var query string
	if namespace == "--all" {
		query = fmt.Sprintf(`sum(avg_over_time(kube_cost_namespace_hourly_usd[%s])) by (namespace)`, *window)
	} else {
		query = fmt.Sprintf(`sum(avg_over_time(kube_cost_namespace_hourly_usd{namespace="%s"}[%s]))`, namespace, *window)
	}

	result, _, err := api.Query(ctx, query, time.Now())
	if err != nil {
		return fmt.Errorf("error querying Prometheus: %w", err)
	}

	vector, ok := result.(model.Vector)
	if !ok {
		return fmt.Errorf("unexpected result type: %T", result)
	}

	if len(vector) == 0 {
		fmt.Printf("No cost data found for namespace: %s\n", namespace)
		return nil
	}

	duration := parseDuration(*window)
	hourlyMultiplier := duration / time.Hour

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
	fmt.Fprintln(w, "NAMESPACE\tHOURLY COST\tTOTAL COST\tMONTHLY PROJECTION")

	for _, sample := range vector {
		ns := string(sample.Metric["namespace"])
		hourlyCost := float64(sample.Value)
		totalCost := hourlyCost * float64(hourlyMultiplier)
		monthlyCost := hourlyCost * 730

		fmt.Fprintf(w, "%s\t$%.4f\t$%.2f\t$%.2f\n", ns, hourlyCost, totalCost, monthlyCost)
	}

	w.Flush()
	return nil
}

func showPodCost(ctx context.Context, api v1.API, namespace, podName string) error {
	query := fmt.Sprintf(`avg_over_time(kube_cost_pod_hourly_usd{namespace="%s",pod=~"%s.*"}[%s])`, namespace, podName, *window)

	result, _, err := api.Query(ctx, query, time.Now())
	if err != nil {
		return fmt.Errorf("error querying Prometheus: %w", err)
	}

	vector, ok := result.(model.Vector)
	if !ok {
		return fmt.Errorf("unexpected result type: %T", result)
	}

	if len(vector) == 0 {
		fmt.Printf("No cost data found for pod: %s in namespace: %s\n", podName, namespace)
		return nil
	}

	duration := parseDuration(*window)
	hourlyMultiplier := duration / time.Hour

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
	fmt.Fprintln(w, "POD\tNAMESPACE\tNODE\tHOURLY COST\tTOTAL COST\tMONTHLY PROJECTION")

	for _, sample := range vector {
		pod := string(sample.Metric["pod"])
		ns := string(sample.Metric["namespace"])
		node := string(sample.Metric["node"])
		hourlyCost := float64(sample.Value)
		totalCost := hourlyCost * float64(hourlyMultiplier)
		monthlyCost := hourlyCost * 730

		fmt.Fprintf(w, "%s\t%s\t%s\t$%.4f\t$%.2f\t$%.2f\n", pod, ns, node, hourlyCost, totalCost, monthlyCost)
	}

	w.Flush()
	return nil
}

func showNodeCost(ctx context.Context, api v1.API) error {
	query := fmt.Sprintf(`avg_over_time(kube_cost_node_hourly_usd[%s])`, *window)

	result, _, err := api.Query(ctx, query, time.Now())
	if err != nil {
		return fmt.Errorf("error querying Prometheus: %w", err)
	}

	vector, ok := result.(model.Vector)
	if !ok {
		return fmt.Errorf("unexpected result type: %T", result)
	}

	if len(vector) == 0 {
		fmt.Println("No node cost data found")
		return nil
	}

	duration := parseDuration(*window)
	hourlyMultiplier := duration / time.Hour

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
	fmt.Fprintln(w, "NODE\tINSTANCE TYPE\tSPOT\tHOURLY COST\tTOTAL COST\tMONTHLY PROJECTION")

	for _, sample := range vector {
		node := string(sample.Metric["node"])
		instanceType := string(sample.Metric["instance_type"])
		isSpot := string(sample.Metric["is_spot"])
		hourlyCost := float64(sample.Value)
		totalCost := hourlyCost * float64(hourlyMultiplier)
		monthlyCost := hourlyCost * 730

		fmt.Fprintf(w, "%s\t%s\t%s\t$%.4f\t$%.2f\t$%.2f\n", node, instanceType, isSpot, hourlyCost, totalCost, monthlyCost)
	}

	w.Flush()
	return nil
}

func showClusterCost(ctx context.Context, api v1.API) error {
	queries := map[string]string{
		"Compute": fmt.Sprintf(`sum(avg_over_time(kube_cost_cluster_hourly_usd[%s]))`, *window),
		"Storage": fmt.Sprintf(`sum(avg_over_time(kube_cost_cluster_storage_monthly_usd[%s]))`, *window),
		"Spot Savings": fmt.Sprintf(`sum(avg_over_time(kube_cost_spot_savings_hourly_usd[%s]))`, *window),
	}

	duration := parseDuration(*window)
	hourlyMultiplier := duration / time.Hour

	fmt.Println("Cluster Cost Summary")
	fmt.Println("====================")
	fmt.Println()

	var totalHourly, totalMonthly float64

	for name, query := range queries {
		result, _, err := api.Query(ctx, query, time.Now())
		if err != nil {
			fmt.Printf("Warning: Could not query %s: %v\n", name, err)
			continue
		}

		vector, ok := result.(model.Vector)
		if !ok || len(vector) == 0 {
			continue
		}

		value := float64(vector[0].Value)

		if name == "Storage" {
			// Storage is already monthly
			monthlyValue := value
			hourlyValue := value / 730
			fmt.Printf("%-15s: $%.2f/hour  |  $%.2f/month\n", name, hourlyValue, monthlyValue)
			totalHourly += hourlyValue
			totalMonthly += monthlyValue
		} else if name == "Spot Savings" {
			monthlyValue := value * 730
			fmt.Printf("%-15s: $%.2f/hour  |  $%.2f/month (savings)\n", name, value, monthlyValue)
		} else {
			monthlyValue := value * 730
			fmt.Printf("%-15s: $%.2f/hour  |  $%.2f/month\n", name, value, monthlyValue)
			totalHourly += value
			totalMonthly += monthlyValue
		}
	}

	fmt.Println()
	fmt.Printf("Total Cost      : $%.2f/hour  |  $%.2f/month\n", totalHourly, totalMonthly)
	fmt.Printf("Window (%s)    : $%.2f\n", *window, totalHourly*float64(hourlyMultiplier))

	return nil
}

func showTopCosts(ctx context.Context, api v1.API, resource string) error {
	var query string
	var labelName string

	switch resource {
	case "pods":
		query = fmt.Sprintf(`topk(10, avg_over_time(kube_cost_pod_hourly_usd[%s]))`, *window)
		labelName = "pod"
	case "namespaces":
		query = fmt.Sprintf(`topk(10, sum(avg_over_time(kube_cost_namespace_hourly_usd[%s])) by (namespace))`, *window)
		labelName = "namespace"
	case "nodes":
		query = fmt.Sprintf(`topk(10, avg_over_time(kube_cost_node_hourly_usd[%s]))`, *window)
		labelName = "node"
	default:
		return fmt.Errorf("unknown resource type: %s (use: pods, namespaces, or nodes)", resource)
	}

	result, _, err := api.Query(ctx, query, time.Now())
	if err != nil {
		return fmt.Errorf("error querying Prometheus: %w", err)
	}

	vector, ok := result.(model.Vector)
	if !ok {
		return fmt.Errorf("unexpected result type: %T", result)
	}

	if len(vector) == 0 {
		fmt.Printf("No cost data found for %s\n", resource)
		return nil
	}

	// Sort by cost descending
	sort.Slice(vector, func(i, j int) bool {
		return vector[i].Value > vector[j].Value
	})

	fmt.Printf("Top 10 %s by cost:\n\n", strings.Title(resource))

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
	if resource == "pods" {
		fmt.Fprintln(w, "RANK\tPOD\tNAMESPACE\tHOURLY COST\tMONTHLY PROJECTION")
		for i, sample := range vector {
			pod := string(sample.Metric["pod"])
			namespace := string(sample.Metric["namespace"])
			hourlyCost := float64(sample.Value)
			monthlyCost := hourlyCost * 730

			fmt.Fprintf(w, "%d\t%s\t%s\t$%.4f\t$%.2f\n", i+1, pod, namespace, hourlyCost, monthlyCost)
		}
	} else {
		fmt.Fprintln(w, "RANK\tNAME\tHOURLY COST\tMONTHLY PROJECTION")
		for i, sample := range vector {
			name := string(sample.Metric[labelName])
			hourlyCost := float64(sample.Value)
			monthlyCost := hourlyCost * 730

			fmt.Fprintf(w, "%d\t%s\t$%.4f\t$%.2f\n", i+1, name, hourlyCost, monthlyCost)
		}
	}

	w.Flush()
	return nil
}

func parseDuration(d string) time.Duration {
	duration, err := time.ParseDuration(d)
	if err != nil {
		// Try parsing as "30d" format
		if strings.HasSuffix(d, "d") {
			days := strings.TrimSuffix(d, "d")
			var daysInt int
			fmt.Sscanf(days, "%d", &daysInt)
			return time.Duration(daysInt) * 24 * time.Hour
		}
		return 24 * time.Hour // Default to 24 hours
	}
	return duration
}

// ResourceEstimate represents estimated cost for a workload
type ResourceEstimate struct {
	Kind      string
	Name      string
	Namespace string
	Replicas  int
	CPUCores  float64
	MemoryGB  float64
	CPUCost   float64
	MemCost   float64
	TotalCost float64
}

func estimateCost(filename string) error {
	// Read file
	data, err := os.ReadFile(filename)
	if err != nil {
		return fmt.Errorf("failed to read file: %w", err)
	}

	// Split multi-doc YAML
	docs := strings.Split(string(data), "---")
	var estimates []ResourceEstimate

	for _, doc := range docs {
		doc = strings.TrimSpace(doc)
		if doc == "" {
			continue
		}

		estimate, err := estimateFromYAML([]byte(doc))
		if err != nil {
			// Skip non-workload resources
			continue
		}

		estimates = append(estimates, estimate)
	}

	if len(estimates) == 0 {
		return fmt.Errorf("no workload resources found in manifest")
	}

	// Display estimates
	fmt.Println("Cost Estimate for Deployment")
	fmt.Println("=============================")
	fmt.Println()

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
	fmt.Fprintln(w, "KIND\tNAME\tREPLICAS\tCPU\tMEMORY\tHOURLY\tDAILY\tMONTHLY")

	var totalHourly, totalDaily, totalMonthly float64

	for _, est := range estimates {
		hourly := est.TotalCost
		daily := hourly * 24
		monthly := hourly * 730

		totalHourly += hourly
		totalDaily += daily
		totalMonthly += monthly

		fmt.Fprintf(w, "%s\t%s\t%d\t%.2f\t%.2fGi\t$%.4f\t$%.2f\t$%.2f\n",
			est.Kind, est.Name, est.Replicas, est.CPUCores, est.MemoryGB,
			hourly, daily, monthly)
	}

	fmt.Fprintln(w, "")
	fmt.Fprintf(w, "TOTAL\t\t\t\t\t$%.4f\t$%.2f\t$%.2f\n",
		totalHourly, totalDaily, totalMonthly)

	w.Flush()

	fmt.Println()
	fmt.Println("Note: Estimates based on average cloud provider pricing:")
	fmt.Println("  - CPU: $30/vCPU/month (~$0.041/vCPU/hour)")
	fmt.Println("  - Memory: $4/GB/month (~$0.0055/GB/hour)")
	fmt.Println("  - Actual costs vary by region, instance type, and cloud provider")

	return nil
}

func estimateFromYAML(data []byte) (ResourceEstimate, error) {
	var manifest map[string]interface{}
	if err := yaml.Unmarshal(data, &manifest); err != nil {
		return ResourceEstimate{}, err
	}

	kind, ok := manifest["kind"].(string)
	if !ok {
		return ResourceEstimate{}, fmt.Errorf("no kind found")
	}

	// Only process workload resources
	validKinds := map[string]bool{
		"Deployment":  true,
		"StatefulSet": true,
		"DaemonSet":   true,
		"Job":         true,
		"CronJob":     true,
	}

	if !validKinds[kind] {
		return ResourceEstimate{}, fmt.Errorf("not a workload resource")
	}

	metadata, ok := manifest["metadata"].(map[string]interface{})
	if !ok {
		return ResourceEstimate{}, fmt.Errorf("no metadata found")
	}

	name, _ := metadata["name"].(string)
	namespace, _ := metadata["namespace"].(string)
	if namespace == "" {
		namespace = "default"
	}

	spec, ok := manifest["spec"].(map[string]interface{})
	if !ok {
		return ResourceEstimate{}, fmt.Errorf("no spec found")
	}

	// Get replicas
	replicas := 1
	if r, ok := spec["replicas"].(int); ok {
		replicas = r
	} else if r, ok := spec["replicas"].(float64); ok {
		replicas = int(r)
	}

	// For CronJob, extract jobTemplate
	if kind == "CronJob" {
		if jobTemplate, ok := spec["jobTemplate"].(map[string]interface{}); ok {
			spec = jobTemplate["spec"].(map[string]interface{})
		}
	}

	// Get template
	template, ok := spec["template"].(map[string]interface{})
	if !ok {
		return ResourceEstimate{}, fmt.Errorf("no template found")
	}

	templateSpec, ok := template["spec"].(map[string]interface{})
	if !ok {
		return ResourceEstimate{}, fmt.Errorf("no template spec found")
	}

	containers, ok := templateSpec["containers"].([]interface{})
	if !ok {
		return ResourceEstimate{}, fmt.Errorf("no containers found")
	}

	// Calculate total resources
	var totalCPU, totalMem float64

	for _, container := range containers {
		cont, ok := container.(map[string]interface{})
		if !ok {
			continue
		}

		resources, ok := cont["resources"].(map[string]interface{})
		if !ok {
			continue
		}

		requests, ok := resources["requests"].(map[string]interface{})
		if !ok {
			// Fall back to limits if no requests
			requests, ok = resources["limits"].(map[string]interface{})
			if !ok {
				continue
			}
		}

		if cpu, ok := requests["cpu"].(string); ok {
			totalCPU += parseCPU(cpu)
		}

		if mem, ok := requests["memory"].(string); ok {
			totalMem += parseMemory(mem)
		}
	}

	// Calculate costs (average cloud pricing)
	// CPU: ~$30/vCPU/month = $0.041/vCPU/hour
	// Memory: ~$4/GB/month = $0.0055/GB/hour
	cpuCostPerHour := 30.0 / 730 // $30 per month / 730 hours
	memCostPerHour := 4.0 / 730  // $4 per GB per month / 730 hours

	totalCPU *= float64(replicas)
	totalMem *= float64(replicas)

	cpuCost := totalCPU * cpuCostPerHour
	memCost := totalMem * memCostPerHour
	totalCost := cpuCost + memCost

	return ResourceEstimate{
		Kind:      kind,
		Name:      name,
		Namespace: namespace,
		Replicas:  replicas,
		CPUCores:  totalCPU,
		MemoryGB:  totalMem,
		CPUCost:   cpuCost,
		MemCost:   memCost,
		TotalCost: totalCost,
	}, nil
}

func parseCPU(cpu string) float64 {
	// Handle millicores (e.g., "100m", "500m")
	if strings.HasSuffix(cpu, "m") {
		millis := strings.TrimSuffix(cpu, "m")
		if val, err := strconv.ParseFloat(millis, 64); err == nil {
			return val / 1000.0
		}
	}

	// Handle cores (e.g., "1", "2.5")
	if val, err := strconv.ParseFloat(cpu, 64); err == nil {
		return val
	}

	return 0
}

func parseMemory(mem string) float64 {
	// Remove spaces
	mem = strings.TrimSpace(mem)

	// Extract number and unit
	re := regexp.MustCompile(`^(\d+(?:\.\d+)?)\s*([A-Za-z]*)$`)
	matches := re.FindStringSubmatch(mem)

	if len(matches) != 3 {
		return 0
	}

	value, err := strconv.ParseFloat(matches[1], 64)
	if err != nil {
		return 0
	}

	unit := strings.ToUpper(matches[2])

	// Convert to GB
	switch unit {
	case "K", "KI":
		return value / (1024 * 1024)
	case "M", "MI":
		return value / 1024
	case "G", "GI":
		return value
	case "T", "TI":
		return value * 1024
	case "KB":
		return value / (1000 * 1000)
	case "MB":
		return value / 1000
	case "GB":
		return value
	case "TB":
		return value * 1000
	default:
		// Assume bytes
		return value / (1024 * 1024 * 1024)
	}
}
