package gadgets

import (
	"fmt"
	"github.com/cilium/ebpf/rlimit"
	containercollection "github.com/inspektor-gadget/inspektor-gadget/pkg/container-collection"
	"log"
	"sync"
	"time"
)

// IGTraceContainerCollector is a constructor.
type IGTraceContainerCollector struct {
	data                       *sync.Map
	containerCollectionOptions []containercollection.ContainerCollectionOption
}

// NewIGTraceContainerCollector is a constructor.
func NewIGTraceContainerCollector(
	containerCollectionOptions []containercollection.ContainerCollectionOption) *IGTraceContainerCollector {
	return &IGTraceContainerCollector{
		data:                       &sync.Map{},
		containerCollectionOptions: containerCollectionOptions,
	}
}

func info(container *containercollection.Container) string {
	if container == nil {
		return time.Now().Format(time.RFC3339Nano)
	}
	return fmt.Sprintf("%s /namespaces/%s/pods/%s/containers/%s ", container.Namespace, container.Podname,
		container.Name, time.Now().Format(time.RFC3339Nano))
}

func (collector *IGTraceContainerCollector) PublishEvent(
	traceName string,
	container *containercollection.Container,
	eventDetails string) {

	events, loaded := collector.data.LoadOrStore(traceName, map[string]string{info(container): eventDetails})
	if loaded {
		// hopefully nano is granular enough to avoid collisions
		(events.(map[string]string))[info(container)] = eventDetails
		collector.data.Store(traceName, events)
	}
}

func (collector *IGTraceContainerCollector) InitContainerCollection() (*containercollection.ContainerCollection, error) {
	// From https://www.inspektor-gadget.io/blog/2022/09/using-inspektor-gadget-from-golang-applications/
	// In some kernel versions it's needed to bump the rlimits to use run BPF programs.
	if err := rlimit.RemoveMemlock(); err != nil {
		return nil, fmt.Errorf("failed to remove memlock: %w", err)
	}

	// We want to trace events from all pods running on the node, not just the current process, aka aks-periscope.
	// To do this we need to make use of a ContainerCollection, which can be initially populated
	// with all the pod processes, and dynamically updated as pods are created and deleted.

	//this is common to all gadgets to listen to container events

	containerEventCallback := func(event containercollection.PubSubEvent) {
		switch event.Type {
		case containercollection.EventTypeAddContainer:
			log.Printf("Container added: %q pid %d\n", event.Container.Name, event.Container.Pid)
		case containercollection.EventTypeRemoveContainer:
			log.Printf("Container removed: %q pid %d\n", event.Container.Name, event.Container.Pid)
		}
	}

	// Use the supplied container collection options, but prepend the container event callback.
	// The options are all functions that are executed when the container collection is initialized.
	opts := append(
		[]containercollection.ContainerCollectionOption{containercollection.WithPubSub(containerEventCallback)},
		collector.containerCollectionOptions...,
	)

	// Initialize the container collection, receiver should close the container collection
	containerCollection := &containercollection.ContainerCollection{}
	if err := containerCollection.Initialize(opts...); err != nil {
		return nil, fmt.Errorf("failed to initialize container collection: %w", err)
	}

	return containerCollection, nil
}

func (collector *IGTraceContainerCollector) GetTracerData(tracerName string) map[string]string {
	events, _ := collector.data.Load(tracerName)
	return events.(map[string]string)
}
