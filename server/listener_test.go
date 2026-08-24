package server

import (
	"testing"

	"github.com/sandertv/gophertunnel/minecraft"
)

type listenerTestProtocol struct {
	minecraft.Protocol
	id      int32
	version string
}

func (p listenerTestProtocol) ID() int32   { return p.id }
func (p listenerTestProtocol) Ver() string { return p.version }

func TestMinecraftListenConfig_AcceptedProtocols(t *testing.T) {
	accepted := []minecraft.Protocol{
		listenerTestProtocol{id: 1, version: "test-one"},
		listenerTestProtocol{id: 2, version: "test-two"},
	}
	cfg := minecraftListenConfig(Config{
		Allower:           allower{},
		AcceptedProtocols: accepted,
	})

	if len(cfg.AcceptedProtocols) != len(accepted) {
		t.Fatalf("accepted protocol count = %d, want %d", len(cfg.AcceptedProtocols), len(accepted))
	}
	for i, protocol := range cfg.AcceptedProtocols {
		if protocol != accepted[i] {
			t.Fatalf("accepted protocol %d was not preserved", i)
		}
	}
}
