package collector

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/Azure/aks-periscope/pkg/utils"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	restclient "k8s.io/client-go/rest"
	metrics "k8s.io/metrics/pkg/client/clientset/versioned"
)

// SystemPerfCollector defines a SystemPerf Collector struct
type SystemPerfCollector struct {
	data        map[string]string
	kubeconfig  *restclient.Config
	runtimeInfo *utils.RuntimeInfo
}

type NodeMetrics struct {
	NodeName    string `json:"name"`
	CPUUsage    int64  `json:"cpuUsage"`
	MemoryUsage int64  `json:"memoryUsage"`
}

type PodMetrics struct {
	ContainerName string `json:"name"`
	CPUUsage      int64  `json:"cpuUsage"`
	MemoryUsage   int64  `json:"memoryUsage"`
}

// NewSystemPerfCollector is a constructor
func NewSystemPerfCollector(config *restclient.Config, runtimeInfo *utils.RuntimeInfo) *SystemPerfCollector {
	return &SystemPerfCollector{
		data:        make(map[string]string),
		kubeconfig:  config,
		runtimeInfo: runtimeInfo,
	}
}

func (collector *SystemPerfCollector) GetName() string {
	return "systemperf"
}

func (collector *SystemPerfCollector) CheckSupported() error {
	if utils.Contains(collector.runtimeInfo.CollectorList, "connectedCluster") {
		return fmt.Errorf("Not included because 'connectedCluster' is in COLLECTOR_LIST variable. Included values: %s", strings.Join(collector.runtimeInfo.CollectorList, " "))
	}

	return nil
}

// Collect implements the interface method
func (collector *SystemPerfCollector) Collect() error {
	metric, err := metrics.NewForConfig(collector.kubeconfig)
	if err != nil {
		return fmt.Errorf("metrics for config error: %w", err)
	}

	nodeMetrics, err := metric.MetricsV1beta1().NodeMetricses().List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("node metrics error: %w", err)
	}

	noderesult := make([]NodeMetrics, 0)

	for _, nodeMetric := range nodeMetrics.Items {
		cpuQuantity := nodeMetric.Usage.Cpu().MilliValue()
		memQuantity, ok := nodeMetric.Usage.Memory().AsInt64()
		if !ok {
			return err
		}

		nm := NodeMetrics{
			NodeName:    nodeMetric.Name,
			CPUUsage:    cpuQuantity,
			MemoryUsage: memQuantity,
		}

		noderesult = append(noderesult, nm)
	}
	jsonNodeResult, err := json.Marshal(noderesult)
	if err != nil {
		return fmt.Errorf("marshall node metrics to json: %w", err)
	}

	collector.data["nodes"] = string(jsonNodeResult)

	podMetrics, err := metric.MetricsV1beta1().PodMetricses(metav1.NamespaceAll).List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("pod metrics failure: %w", err)
	}

	podresult := make([]PodMetrics, 0)

	for _, podMetric := range podMetrics.Items {
		podContainers := podMetric.Containers
		for _, container := range podContainers {
			cpuQuantity := container.Usage.Cpu().MilliValue()
			memQuantity, ok := container.Usage.Memory().AsInt64()
			if !ok {
				return fmt.Errorf("usage memory failure: %w", err)
			}

			pm := PodMetrics{
				ContainerName: container.Name,
				CPUUsage:      cpuQuantity,
				MemoryUsage:   memQuantity,
			}

			podresult = append(podresult, pm)
		}
	}
	jsonPodResult, err := json.Marshal(podresult)
	if err != nil {
		return fmt.Errorf("marshall pod metrics to json: %w", err)
	}

	collector.data["pods"] = string(jsonPodResult)

	return nil
}

func (collector *SystemPerfCollector) GetData() map[string]string {
	return collector.data
}
