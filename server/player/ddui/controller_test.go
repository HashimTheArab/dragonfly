package ddui

import (
	"testing"

	"github.com/sandertv/gophertunnel/minecraft/protocol"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

func TestController_HandlesOnlyOwnedDataStoreUpdates(t *testing.T) {
	var packets []packet.Packet
	controller := NewController(func(pk packet.Packet) {
		packets = append(packets, pk)
	})

	value := NewObservable("before", true)
	controller.Show(New("Settings", TextField("Name", value)))

	show := packets[1].(*packet.ClientBoundDataDrivenUIShowScreen)
	property := packets[0].(*packet.ClientBoundDataStore).Updates[0].Change.Property
	instanceID, ok := show.DataInstanceID.Value()
	if !ok || instanceID == 0 {
		t.Fatal("controller allocated a zero data instance ID")
	}

	foreign := &packet.ServerBoundDataStore{Update: protocol.DataStoreUpdate{
		DataStoreName: "minecraft",
		Property:      property + "_foreign",
		Path:          "layout[0].text",
		ControlType:   protocol.DataStoreControlString,
		StringValue:   "foreign",
	}}
	if controller.HandleDataStore(foreign) {
		t.Fatal("controller claimed a foreign data-store update")
	}
	if got := value.Get(); got != "before" {
		t.Fatalf("foreign update changed value to %q", got)
	}

	owned := &packet.ServerBoundDataStore{Update: protocol.DataStoreUpdate{
		DataStoreName: "minecraft",
		Property:      property,
		Path:          "layout[0].text",
		ControlType:   protocol.DataStoreControlString,
		StringValue:   "after",
	}}
	if !controller.HandleDataStore(owned) {
		t.Fatal("controller did not claim its own data-store update")
	}
	if got := value.Get(); got != "after" {
		t.Fatalf("owned update changed value to %q, want after", got)
	}
}

func TestController_HandlesOnlyOwnedScreenClosures(t *testing.T) {
	var packets []packet.Packet
	controller := NewController(func(pk packet.Packet) {
		packets = append(packets, pk)
	})

	closed := false
	controller.Show(New("Settings", Handler(func(int) { closed = true })))
	show := packets[1].(*packet.ClientBoundDataDrivenUIShowScreen)

	if controller.HandleScreenClosed(&packet.ServerBoundDataDrivenScreenClosed{FormID: show.FormID + 1}) {
		t.Fatal("controller claimed a foreign screen closure")
	}
	if closed {
		t.Fatal("foreign screen closure closed the owned form")
	}

	if !controller.HandleScreenClosed(&packet.ServerBoundDataDrivenScreenClosed{
		FormID:      show.FormID,
		CloseReason: packet.DataDrivenScreenCloseReasonClientCanceled,
	}) {
		t.Fatal("controller did not claim its own screen closure")
	}
	if !closed {
		t.Fatal("owned screen closure did not close the form")
	}
}
