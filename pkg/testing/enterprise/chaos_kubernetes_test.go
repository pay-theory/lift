package enterprise

import (
	"context"
	"testing"
	"time"
)

func TestChaosMeshIntegration_Basics(t *testing.T) {
	if _, err := NewChaosMeshIntegration(nil); err == nil {
		t.Fatalf("NewChaosMeshIntegration(nil) expected error")
	}

	integration, err := NewChaosMeshIntegration(&KubernetesConfig{})
	if err != nil {
		t.Fatalf("NewChaosMeshIntegration error: %v", err)
	}

	faults := integration.GetSupportedFaults()
	if len(faults) == 0 {
		t.Fatalf("expected supported faults")
	}

	// ValidateExperimentSpec errors.
	_, err = integration.CreateExperiment(context.Background(), &ChaosExperimentSpec{})
	if err == nil {
		t.Fatalf("CreateExperiment expected error for invalid spec")
	}

	// Controller type mismatch (constants vs controller map keys).
	_, err = integration.CreateExperiment(context.Background(), &ChaosExperimentSpec{
		Name:           "exp",
		Namespace:      "default",
		ControllerType: PodChaosControllerType,
		FaultType:      LatencyFault,
		TargetSelector: &TargetSelector{PodSelector: &PodSelector{}},
		Duration:       1 * time.Second,
	})
	if err == nil {
		t.Fatalf("CreateExperiment expected error for unsupported controller type")
	}

	// Success path uses controller name as controller type.
	result, err := integration.CreateExperiment(context.Background(), &ChaosExperimentSpec{
		Name:           "exp",
		Namespace:      "default",
		ControllerType: ChaosControllerType("pod-chaos-controller"),
		FaultType:      LatencyFault,
		TargetSelector: &TargetSelector{PodSelector: &PodSelector{}},
		Duration:       1 * time.Second,
	})
	if err != nil || result == nil || result.ExperimentID == "" {
		t.Fatalf("CreateExperiment expected success, got result=%#v err=%v", result, err)
	}

	if _, err := integration.MonitorExperiment(context.Background(), "id", ChaosControllerType("missing")); err == nil {
		t.Fatalf("MonitorExperiment expected error for missing controller")
	}
	if err := integration.StopExperiment(context.Background(), "id", ChaosControllerType("missing")); err == nil {
		t.Fatalf("StopExperiment expected error for missing controller")
	}
}

func TestChaosEventBus_PublishEvent_QueueFull(t *testing.T) {
	bus := &ChaosEventBus{
		subscribers: make(map[string][]EventSubscriber),
		eventQueue:  make(chan *ChaosEvent),
	}

	if err := bus.PublishEvent(context.Background(), &ChaosEvent{ID: "1"}); err == nil {
		t.Fatalf("PublishEvent expected queue full error")
	}

	bus.Subscribe("x", nil)
	if len(bus.subscribers["x"]) != 1 {
		t.Fatalf("Subscribe did not record subscriber")
	}
}

func TestPodChaosController_Methods(t *testing.T) {
	ctx := context.Background()

	pod := NewPodChaosController(&KubernetesConfig{})
	if pod.GetName() == "" {
		t.Fatalf("pod GetName empty")
	}
	if pod.GetType() != PodChaosControllerType {
		t.Fatalf("pod GetType=%q, want %q", pod.GetType(), PodChaosControllerType)
	}
	if err := pod.Initialize(ctx, &KubernetesConfig{}); err != nil {
		t.Fatalf("pod Initialize error: %v", err)
	}
	if _, err := pod.CreateChaosExperiment(ctx, &ChaosExperimentSpec{}); err != nil {
		t.Fatalf("pod CreateChaosExperiment error: %v", err)
	}
	if _, err := pod.MonitorExperiment(ctx, "id"); err != nil {
		t.Fatalf("pod MonitorExperiment error: %v", err)
	}
	if err := pod.StopExperiment(ctx, "id"); err != nil {
		t.Fatalf("pod StopExperiment error: %v", err)
	}
	if len(pod.GetSupportedFaults()) == 0 {
		t.Fatalf("pod GetSupportedFaults empty")
	}
	if err := pod.ValidateSpec(&ChaosExperimentSpec{TargetSelector: &TargetSelector{}}); err == nil {
		t.Fatalf("pod ValidateSpec expected error when PodSelector is nil")
	}
	if err := pod.ValidateSpec(&ChaosExperimentSpec{TargetSelector: &TargetSelector{PodSelector: &PodSelector{}}}); err != nil {
		t.Fatalf("pod ValidateSpec error: %v", err)
	}
	if err := pod.Cleanup(ctx); err != nil {
		t.Fatalf("pod Cleanup error: %v", err)
	}
}

func TestNetworkChaosController_Methods(t *testing.T) {
	ctx := context.Background()

	network := NewNetworkChaosController(&KubernetesConfig{})
	if network.GetType() != NetworkChaosControllerType {
		t.Fatalf("network GetType=%q, want %q", network.GetType(), NetworkChaosControllerType)
	}
	if err := network.Initialize(ctx, &KubernetesConfig{}); err != nil {
		t.Fatalf("network Initialize error: %v", err)
	}
	if _, err := network.CreateChaosExperiment(ctx, nil); err != nil {
		t.Fatalf("network CreateChaosExperiment error: %v", err)
	}
	if _, err := network.MonitorExperiment(ctx, "id"); err != nil {
		t.Fatalf("network MonitorExperiment error: %v", err)
	}
	if err := network.StopExperiment(ctx, "id"); err != nil {
		t.Fatalf("network StopExperiment error: %v", err)
	}
	if len(network.GetSupportedFaults()) == 0 {
		t.Fatalf("network GetSupportedFaults empty")
	}
	if err := network.ValidateSpec(nil); err != nil {
		t.Fatalf("network ValidateSpec error: %v", err)
	}
	if err := network.Cleanup(ctx); err != nil {
		t.Fatalf("network Cleanup error: %v", err)
	}
}

func TestStressChaosController_Methods(t *testing.T) {
	ctx := context.Background()

	stress := NewStressChaosController(&KubernetesConfig{})
	if stress.GetType() != StressChaosControllerType {
		t.Fatalf("stress GetType=%q, want %q", stress.GetType(), StressChaosControllerType)
	}
	if err := stress.Initialize(ctx, &KubernetesConfig{}); err != nil {
		t.Fatalf("stress Initialize error: %v", err)
	}
	if _, err := stress.CreateChaosExperiment(ctx, nil); err != nil {
		t.Fatalf("stress CreateChaosExperiment error: %v", err)
	}
	if _, err := stress.MonitorExperiment(ctx, "id"); err != nil {
		t.Fatalf("stress MonitorExperiment error: %v", err)
	}
	if err := stress.StopExperiment(ctx, "id"); err != nil {
		t.Fatalf("stress StopExperiment error: %v", err)
	}
	if len(stress.GetSupportedFaults()) == 0 {
		t.Fatalf("stress GetSupportedFaults empty")
	}
	if err := stress.ValidateSpec(nil); err != nil {
		t.Fatalf("stress ValidateSpec error: %v", err)
	}
	if err := stress.Cleanup(ctx); err != nil {
		t.Fatalf("stress Cleanup error: %v", err)
	}
}

func TestIOChaosController_Methods(t *testing.T) {
	ctx := context.Background()

	ioctl := NewIOChaosController(&KubernetesConfig{})
	if ioctl.GetType() != IOChaosControllerType {
		t.Fatalf("io GetType=%q, want %q", ioctl.GetType(), IOChaosControllerType)
	}
	if err := ioctl.Initialize(ctx, &KubernetesConfig{}); err != nil {
		t.Fatalf("io Initialize error: %v", err)
	}
	if _, err := ioctl.CreateChaosExperiment(ctx, nil); err != nil {
		t.Fatalf("io CreateChaosExperiment error: %v", err)
	}
	if _, err := ioctl.MonitorExperiment(ctx, "id"); err != nil {
		t.Fatalf("io MonitorExperiment error: %v", err)
	}
	if err := ioctl.StopExperiment(ctx, "id"); err != nil {
		t.Fatalf("io StopExperiment error: %v", err)
	}
	if len(ioctl.GetSupportedFaults()) == 0 {
		t.Fatalf("io GetSupportedFaults empty")
	}
	if err := ioctl.ValidateSpec(nil); err != nil {
		t.Fatalf("io ValidateSpec error: %v", err)
	}
	if err := ioctl.Cleanup(ctx); err != nil {
		t.Fatalf("io Cleanup error: %v", err)
	}
}

func TestTimeChaosController_Methods(t *testing.T) {
	ctx := context.Background()

	timeCtl := NewTimeChaosController(&KubernetesConfig{})
	if timeCtl.GetType() != TimeChaosControllerType {
		t.Fatalf("time GetType=%q, want %q", timeCtl.GetType(), TimeChaosControllerType)
	}
	if err := timeCtl.Initialize(ctx, &KubernetesConfig{}); err != nil {
		t.Fatalf("time Initialize error: %v", err)
	}
	if _, err := timeCtl.CreateChaosExperiment(ctx, nil); err != nil {
		t.Fatalf("time CreateChaosExperiment error: %v", err)
	}
	if _, err := timeCtl.MonitorExperiment(ctx, "id"); err != nil {
		t.Fatalf("time MonitorExperiment error: %v", err)
	}
	if err := timeCtl.StopExperiment(ctx, "id"); err != nil {
		t.Fatalf("time StopExperiment error: %v", err)
	}
	if len(timeCtl.GetSupportedFaults()) == 0 {
		t.Fatalf("time GetSupportedFaults empty")
	}
	if err := timeCtl.ValidateSpec(nil); err != nil {
		t.Fatalf("time ValidateSpec error: %v", err)
	}
	if err := timeCtl.Cleanup(ctx); err != nil {
		t.Fatalf("time Cleanup error: %v", err)
	}
}

func TestChaosMeshIntegration_MonitorStop_PublishWarningPaths(t *testing.T) {
	ctx := context.Background()

	integration, err := NewChaosMeshIntegration(&KubernetesConfig{})
	if err != nil {
		t.Fatalf("NewChaosMeshIntegration error: %v", err)
	}
	integration.eventBus = &ChaosEventBus{
		subscribers: make(map[string][]EventSubscriber),
		eventQueue:  make(chan *ChaosEvent),
	}
	if _, err := integration.MonitorExperiment(ctx, "id", ChaosControllerType("pod-chaos-controller")); err != nil {
		t.Fatalf("MonitorExperiment success path error: %v", err)
	}
	if err := integration.StopExperiment(ctx, "id", ChaosControllerType("pod-chaos-controller")); err != nil {
		t.Fatalf("StopExperiment success path error: %v", err)
	}
}
