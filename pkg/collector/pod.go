package collector

import (
	"context"
	"fmt"

	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// PodCollector collects pod information for cost calculation
type PodCollector struct {
	clientset *kubernetes.Clientset
	logger    *logrus.Logger
}

// NewPodCollector creates a new pod collector
func NewPodCollector(clientset *kubernetes.Clientset) *PodCollector {
	logger := logrus.New()
	logger.SetLevel(logrus.InfoLevel)

	return &PodCollector{
		clientset: clientset,
		logger:    logger,
	}
}

// PodInfo contains information about a pod for cost calculation
type PodInfo struct {
	Name              string
	Namespace         string
	NodeName          string
	CPURequest        int64 // millicores
	MemoryRequest     int64 // bytes
	CPULimit          int64 // millicores
	MemoryLimit       int64 // bytes
	Labels            map[string]string
	OwnerKind         string
	OwnerName         string
}

// CollectPods collects all pods in the cluster
func (pc *PodCollector) CollectPods(ctx context.Context) ([]PodInfo, error) {
	pods, err := pc.clientset.CoreV1().Pods("").List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to list pods: %w", err)
	}

	var podInfos []PodInfo
	for _, pod := range pods.Items {
		// Skip pods that are not running or scheduled
		if pod.Status.Phase != corev1.PodRunning && pod.Status.Phase != corev1.PodPending {
			continue
		}

		podInfo := pc.extractPodInfo(&pod)
		podInfos = append(podInfos, podInfo)
	}

	return podInfos, nil
}

// CollectPodsOnNode collects pods running on a specific node
func (pc *PodCollector) CollectPodsOnNode(ctx context.Context, nodeName string) ([]PodInfo, error) {
	fieldSelector := fmt.Sprintf("spec.nodeName=%s", nodeName)
	pods, err := pc.clientset.CoreV1().Pods("").List(ctx, metav1.ListOptions{
		FieldSelector: fieldSelector,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list pods on node %s: %w", nodeName, err)
	}

	var podInfos []PodInfo
	for _, pod := range pods.Items {
		if pod.Status.Phase != corev1.PodRunning {
			continue
		}

		podInfo := pc.extractPodInfo(&pod)
		podInfos = append(podInfos, podInfo)
	}

	return podInfos, nil
}

// extractPodInfo extracts relevant information from a pod
func (pc *PodCollector) extractPodInfo(pod *corev1.Pod) PodInfo {
	cpuRequest, memoryRequest := pc.getPodRequests(pod)
	cpuLimit, memoryLimit := pc.getPodLimits(pod)
	ownerKind, ownerName := pc.getPodOwner(pod)

	return PodInfo{
		Name:          pod.Name,
		Namespace:     pod.Namespace,
		NodeName:      pod.Spec.NodeName,
		CPURequest:    cpuRequest,
		MemoryRequest: memoryRequest,
		CPULimit:      cpuLimit,
		MemoryLimit:   memoryLimit,
		Labels:        pod.Labels,
		OwnerKind:     ownerKind,
		OwnerName:     ownerName,
	}
}

// getPodRequests calculates total resource requests for a pod
func (pc *PodCollector) getPodRequests(pod *corev1.Pod) (int64, int64) {
	var cpuRequest, memoryRequest int64

	for _, container := range pod.Spec.Containers {
		if cpu := container.Resources.Requests.Cpu(); cpu != nil {
			cpuRequest += cpu.MilliValue()
		}
		if memory := container.Resources.Requests.Memory(); memory != nil {
			memoryRequest += memory.Value()
		}
	}

	// Include init containers (they run sequentially, so take max)
	var maxInitCPU, maxInitMemory int64
	for _, container := range pod.Spec.InitContainers {
		if cpu := container.Resources.Requests.Cpu(); cpu != nil {
			if cpu.MilliValue() > maxInitCPU {
				maxInitCPU = cpu.MilliValue()
			}
		}
		if memory := container.Resources.Requests.Memory(); memory != nil {
			if memory.Value() > maxInitMemory {
				maxInitMemory = memory.Value()
			}
		}
	}

	if maxInitCPU > cpuRequest {
		cpuRequest = maxInitCPU
	}
	if maxInitMemory > memoryRequest {
		memoryRequest = maxInitMemory
	}

	return cpuRequest, memoryRequest
}

// getPodLimits calculates total resource limits for a pod
func (pc *PodCollector) getPodLimits(pod *corev1.Pod) (int64, int64) {
	var cpuLimit, memoryLimit int64

	for _, container := range pod.Spec.Containers {
		if cpu := container.Resources.Limits.Cpu(); cpu != nil {
			cpuLimit += cpu.MilliValue()
		}
		if memory := container.Resources.Limits.Memory(); memory != nil {
			memoryLimit += memory.Value()
		}
	}

	return cpuLimit, memoryLimit
}

// getPodOwner extracts the owner reference (Deployment, StatefulSet, etc.)
func (pc *PodCollector) getPodOwner(pod *corev1.Pod) (string, string) {
	if len(pod.OwnerReferences) == 0 {
		return "Pod", pod.Name
	}

	owner := pod.OwnerReferences[0]
	ownerKind := owner.Kind
	ownerName := owner.Name

	// For ReplicaSets, try to find the Deployment owner
	if ownerKind == "ReplicaSet" {
		// The ReplicaSet name typically includes the Deployment name as a prefix
		// Format: <deployment-name>-<hash>
		// We'll just return the ReplicaSet for now; in production,
		// you'd query the ReplicaSet to find its Deployment owner
		return "ReplicaSet", ownerName
	}

	return ownerKind, ownerName
}
