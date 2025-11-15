package collector

import (
	"context"
	"fmt"

	"github.com/deepcost/kube-cost-exporter/pkg/pricing"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// StorageCollector collects persistent volume information and pricing
type StorageCollector struct {
	clientset     *kubernetes.Clientset
	pricingCache  *pricing.PricingCache
	cloudProvider string
	region        string
	logger        *logrus.Logger
}

// NewStorageCollector creates a new storage collector
func NewStorageCollector(clientset *kubernetes.Clientset, pricingCache *pricing.PricingCache, cloudProvider, region string) *StorageCollector {
	logger := logrus.New()
	logger.SetLevel(logrus.InfoLevel)

	return &StorageCollector{
		clientset:     clientset,
		pricingCache:  pricingCache,
		cloudProvider: cloudProvider,
		region:        region,
		logger:        logger,
	}
}

// PVInfo contains information about a persistent volume and its pricing
type PVInfo struct {
	Name         string
	StorageClass string
	Namespace    string
	PVCName      string
	SizeGB       int64
	PricePerGB   float64
	MonthlyCost  float64
	Region       string
}

// CollectPVs collects all persistent volumes and their pricing
func (sc *StorageCollector) CollectPVs(ctx context.Context) ([]PVInfo, error) {
	pvs, err := sc.clientset.CoreV1().PersistentVolumes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to list persistent volumes: %w", err)
	}

	var pvInfos []PVInfo
	for _, pv := range pvs.Items {
		pvInfo, err := sc.collectPVInfo(ctx, &pv)
		if err != nil {
			sc.logger.Warnf("Failed to collect info for PV %s: %v", pv.Name, err)
			continue
		}
		pvInfos = append(pvInfos, pvInfo)
	}

	return pvInfos, nil
}

// collectPVInfo extracts pricing information for a single persistent volume
func (sc *StorageCollector) collectPVInfo(ctx context.Context, pv *corev1.PersistentVolume) (PVInfo, error) {
	storageClass := sc.getStorageClass(pv)
	sizeGB := sc.getPVSizeGB(pv)
	namespace, pvcName := sc.getPVCInfo(pv)

	// Get storage pricing
	pricePerGB, err := sc.pricingCache.GetStoragePrice(ctx, storageClass, sc.region)
	if err != nil {
		sc.logger.Warnf("Failed to get storage price for %s: %v", pv.Name, err)
		pricePerGB = 0.10 // Default fallback
	}

	monthlyCost := float64(sizeGB) * pricePerGB

	return PVInfo{
		Name:         pv.Name,
		StorageClass: storageClass,
		Namespace:    namespace,
		PVCName:      pvcName,
		SizeGB:       sizeGB,
		PricePerGB:   pricePerGB,
		MonthlyCost:  monthlyCost,
		Region:       sc.region,
	}, nil
}

// getStorageClass extracts the storage class from PV
func (sc *StorageCollector) getStorageClass(pv *corev1.PersistentVolume) string {
	if pv.Spec.StorageClassName != "" {
		return pv.Spec.StorageClassName
	}

	// Try to infer from annotations or labels
	if class, ok := pv.Annotations["volume.beta.kubernetes.io/storage-class"]; ok {
		return class
	}

	// Map cloud-specific storage types
	switch sc.cloudProvider {
	case "aws":
		if pv.Spec.AWSElasticBlockStore != nil {
			return "gp2" // Default EBS type
		}
		return "gp3"
	case "gcp":
		if pv.Spec.GCEPersistentDisk != nil {
			return "pd-standard"
		}
		return "pd-balanced"
	case "azure":
		if pv.Spec.AzureDisk != nil {
			return "StandardSSD_LRS"
		}
		return "Standard_LRS"
	}

	return "standard"
}

// getPVSizeGB extracts the size in GB from PV
func (sc *StorageCollector) getPVSizeGB(pv *corev1.PersistentVolume) int64 {
	if capacity, ok := pv.Spec.Capacity[corev1.ResourceStorage]; ok {
		// Convert to GB (1 GB = 1000^3 bytes)
		bytes := capacity.Value()
		gb := bytes / (1000 * 1000 * 1000)
		if gb == 0 {
			gb = 1 // Minimum 1 GB for pricing
		}
		return gb
	}
	return 0
}

// getPVCInfo extracts the PVC information if bound
func (sc *StorageCollector) getPVCInfo(pv *corev1.PersistentVolume) (string, string) {
	if pv.Spec.ClaimRef != nil {
		return pv.Spec.ClaimRef.Namespace, pv.Spec.ClaimRef.Name
	}
	return "", ""
}

// CollectPVCsInNamespace collects PVCs in a specific namespace
func (sc *StorageCollector) CollectPVCsInNamespace(ctx context.Context, namespace string) ([]PVInfo, error) {
	pvcs, err := sc.clientset.CoreV1().PersistentVolumeClaims(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to list PVCs in namespace %s: %w", namespace, err)
	}

	var pvInfos []PVInfo
	for _, pvc := range pvcs.Items {
		// Get the bound PV
		if pvc.Spec.VolumeName == "" {
			continue // Skip unbound PVCs
		}

		pv, err := sc.clientset.CoreV1().PersistentVolumes().Get(ctx, pvc.Spec.VolumeName, metav1.GetOptions{})
		if err != nil {
			sc.logger.Warnf("Failed to get PV %s for PVC %s/%s: %v", pvc.Spec.VolumeName, namespace, pvc.Name, err)
			continue
		}

		pvInfo, err := sc.collectPVInfo(ctx, pv)
		if err != nil {
			sc.logger.Warnf("Failed to collect info for PV %s: %v", pv.Name, err)
			continue
		}

		pvInfos = append(pvInfos, pvInfo)
	}

	return pvInfos, nil
}
