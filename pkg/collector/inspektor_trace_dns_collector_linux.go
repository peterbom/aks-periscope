package collector

import (
	"fmt"
	"github.com/Azure/aks-periscope/pkg/collector/gadgets"
	"github.com/Azure/aks-periscope/pkg/interfaces"
	"github.com/Azure/aks-periscope/pkg/utils"
	containercollection "github.com/inspektor-gadget/inspektor-gadget/pkg/container-collection"
	"github.com/inspektor-gadget/inspektor-gadget/pkg/container-collection/networktracer"
	"github.com/inspektor-gadget/inspektor-gadget/pkg/gadgets/trace/dns/tracer"
	dnstypes "github.com/inspektor-gadget/inspektor-gadget/pkg/gadgets/trace/dns/types"
	eventtypes "github.com/inspektor-gadget/inspektor-gadget/pkg/types"
)

// InspektorGadgetDNSTraceCollector defines a InspektorGadget Trace DNS Collector struct
type InspektorGadgetDNSTraceCollector struct {
	igContainerCollector *gadgets.IGTraceContainerCollector
	runtimeInfo          *utils.RuntimeInfo
	waiter               func()
}

// CheckSupported implements the interface method
func (collector *InspektorGadgetDNSTraceCollector) CheckSupported() error {
	// Inspektor Gadget relies on eBPF which is not (currently) available on Windows nodes.
	// However, we're only compiling this for Linux OS right now, so we can skip the OS check.
	return nil
}

// NewInspektorGadgetDNSTraceCollector is a constructor.
func NewInspektorGadgetDNSTraceCollector(
	runtimeInfo *utils.RuntimeInfo,
	waiter func(),
	containerCollectionOptions []containercollection.ContainerCollectionOption,
) *InspektorGadgetDNSTraceCollector {
	return &InspektorGadgetDNSTraceCollector{
		runtimeInfo:          runtimeInfo,
		waiter:               waiter,
		igContainerCollector: gadgets.NewIGTraceContainerCollector(containerCollectionOptions),
	}
}

func (collector *InspektorGadgetDNSTraceCollector) GetName() string {
	return "ig-dnstrace"
}

// Collect implements the interface method
func (collector *InspektorGadgetDNSTraceCollector) Collect() error {

	containerCollection, err := collector.igContainerCollector.InitContainerCollection()
	if err != nil {
		return fmt.Errorf("failed to initialize container collection: %w", err)
	}
	defer containerCollection.Close()

	// The DNS tracer by itself is not associated with any process. It will need to be 'connected'
	// to the container collection, defined in igContainerCollector.
	tracer, err := tracer.NewTracer()
	if err != nil {
		return fmt.Errorf("failed to start dns tracer: %w", err)
	}
	defer tracer.Close()

	// Events will be collected in a callback from the DNS tracer.
	dnsEventCallback := func(container *containercollection.Container, event dnstypes.Event) {
		// Enrich event with data from container
		event.Node = collector.runtimeInfo.HostNodeName
		if !container.HostNetwork {
			event.Namespace = container.Namespace
			event.Pod = container.Podname
			event.Container = container.Name
		}
		// TODO publish event rather than string, however requires super type
		collector.igContainerCollector.PublishEvent(collector.GetName(), container, eventtypes.EventString(event))
	}

	// Set up the information needed to link the tracer to the containers. The selector is empty,
	// meaning that all containers in the collection will be traced.
	config := &networktracer.ConnectToContainerCollectionConfig[dnstypes.Event]{
		Tracer:        tracer,
		Resolver:      containerCollection,
		Selector:      containercollection.ContainerSelector{},
		EventCallback: dnsEventCallback,
		Base:          dnstypes.Base,
	}

	// Connect the tracer up. Closing the connection will detach the PIDs from the tracer.
	conn, err := networktracer.ConnectToContainerCollection(config)
	if err != nil {
		return fmt.Errorf("failed to connect network tracer - dns tracer: %w", err)
	}
	defer conn.Close()

	// The trace is now running. Run whatever function our consumer has supplied before storing the
	// collected data.
	collector.waiter()

	return nil
}

// GetData implements the interface method
func (collector *InspektorGadgetDNSTraceCollector) GetData() map[string]interfaces.DataValue {
	return utils.ToDataValueMap(collector.igContainerCollector.GetTracerData(collector.GetName()))
}
