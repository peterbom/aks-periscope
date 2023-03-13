package collector

import (
	"fmt"
	"github.com/Azure/aks-periscope/pkg/collector/gadgets"
	"github.com/Azure/aks-periscope/pkg/interfaces"
	"github.com/Azure/aks-periscope/pkg/utils"
	containercollection "github.com/inspektor-gadget/inspektor-gadget/pkg/container-collection"
	"github.com/inspektor-gadget/inspektor-gadget/pkg/gadget-collection/gadgets/trace"
	tcptracer "github.com/inspektor-gadget/inspektor-gadget/pkg/gadgets/trace/tcp/tracer"
	tcptypes "github.com/inspektor-gadget/inspektor-gadget/pkg/gadgets/trace/tcp/types"
	standardtracer "github.com/inspektor-gadget/inspektor-gadget/pkg/standardgadgets/trace/tcp"
	eventtypes "github.com/inspektor-gadget/inspektor-gadget/pkg/types"
	"log"
)

// InspektorGadgetTCPTraceCollector defines a InspektorGadget Trace TCP Collector struct
type InspektorGadgetTCPTraceCollector struct {
	runtimeInfo          *utils.RuntimeInfo
	igContainerCollector *gadgets.IGTraceContainerCollector
	waiter               func()
}

// CheckSupported implements the interface method
func (collector *InspektorGadgetTCPTraceCollector) CheckSupported() error {
	// Inspektor Gadget relies on eBPF which is not (currently) available on Windows nodes.
	// However, we're only compiling this for Linux OS right now, so we can skip the OS check.
	return nil
}

// NewInspektorGadgetTCPTraceCollector is a constructor.
func NewInspektorGadgetTCPTraceCollector(
	runtimeInfo *utils.RuntimeInfo,
	waiter func(),
	containerCollectionOptions []containercollection.ContainerCollectionOption,
) *InspektorGadgetTCPTraceCollector {
	return &InspektorGadgetTCPTraceCollector{
		runtimeInfo:          runtimeInfo,
		waiter:               waiter,
		igContainerCollector: gadgets.NewIGTraceContainerCollector(containerCollectionOptions),
	}
}

func (collector *InspektorGadgetTCPTraceCollector) GetName() string {
	return "ig-tcptrace"
}

// Collect implements the interface method
func (collector *InspektorGadgetTCPTraceCollector) Collect() error {
	containerCollection, err := collector.igContainerCollector.InitContainerCollection()
	if err != nil {
		return fmt.Errorf("failed to initialize container collection: %w", err)
	}
	defer containerCollection.Close()

	tcpEventCallback := func(event tcptypes.Event) {
		collector.igContainerCollector.PublishEvent(collector.GetName(), nil, eventtypes.EventString(event))

	}

	traceConfig := &tcptracer.Config{}

	var tracer trace.Tracer
	tracer, err = tcptracer.NewTracer(traceConfig, containerCollection, tcpEventCallback)
	if err != nil {
		log.Printf("Failed to create core tracer, falling back to standard one: %v", err)
		tracer, err = standardtracer.NewTracer(traceConfig, tcpEventCallback)
		if err != nil {
			return fmt.Errorf("failed to create a tracer: %w", err)
		}
	}
	defer tracer.Stop()

	collector.waiter()

	return nil
}

// GetData implements the interface method
func (collector *InspektorGadgetTCPTraceCollector) GetData() map[string]interfaces.DataValue {
	return utils.ToDataValueMap(collector.igContainerCollector.GetTracerData(collector.GetName()))
}
