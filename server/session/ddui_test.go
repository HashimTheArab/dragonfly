package session

import (
	"testing"

	"github.com/df-mc/dragonfly/server/player/ddui"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

func TestSerializeFormUsesMessageBoxSchemaWhenFieldsAreEmpty(t *testing.T) {
	m := ddui.NewMessageBox("title")

	value := serializeForm(m.ScreenID(), m.Describe())
	if _, ok := dataStoreMapEntry(value, "body"); !ok {
		t.Fatal("message box was not serialized with message-box schema")
	}
	if _, ok := dataStoreMapEntry(value, "layout"); ok {
		t.Fatal("message box was serialized with custom-form schema")
	}
}

func TestSerializeFormUsesFormScreenID(t *testing.T) {
	f := &recordingDDUIForm{}

	value := serializeForm(f.ScreenID(), f.Describe())
	if _, ok := dataStoreMapEntry(value, "body"); !ok {
		t.Fatal("external message-box form was not serialized with message-box schema")
	}
}

func TestServerBoundDataStoreRejectsMismatchedStoreAndProperty(t *testing.T) {
	tests := []struct {
		name       string
		store      string
		property   string
		wantUpdate bool
	}{
		{name: "matching", store: "minecraft", property: "message_box_data_1", wantUpdate: true},
		{name: "wrong store", store: "other", property: "message_box_data_1"},
		{name: "wrong property", store: "minecraft", property: "custom_form_data_1"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := &recordingDDUIForm{}
			h := &DDUIFormHandler{forms: map[uint32]*activeDDUIForm{
				1: {form: f, instanceID: 1, property: "message_box_data_1"},
			}}
			s := &Session{packets: make(chan packet.Packet, 1), closeBackground: make(chan struct{})}
			pk := &packet.ServerBoundDataStore{Update: protocol.DataStoreUpdate{
				DataStoreName: tt.store,
				Property:      tt.property,
				Path:          "body",
				ControlType:   protocol.DataStoreControlString,
				StringValue:   "updated",
			}}

			if err := (&ServerBoundDataStoreHandler{h: h}).Handle(pk, s, nil, nil); err != nil {
				t.Fatalf("Handle() error = %v", err)
			}
			if got := f.updates != 0; got != tt.wantUpdate {
				t.Fatalf("updated = %v, want %v", got, tt.wantUpdate)
			}
		})
	}
}

func TestServerBoundDataStoreRejectsCoveredForm(t *testing.T) {
	covered := &recordingDDUIForm{}
	h := &DDUIFormHandler{forms: map[uint32]*activeDDUIForm{
		1: {form: covered, instanceID: 1, property: "custom_form_data_1"},
		2: {form: &recordingDDUIForm{}, instanceID: 2, property: "custom_form_data_2"},
	}}
	s := &Session{packets: make(chan packet.Packet, 1), closeBackground: make(chan struct{})}
	pk := &packet.ServerBoundDataStore{Update: protocol.DataStoreUpdate{
		DataStoreName: "minecraft",
		Property:      "custom_form_data_1",
		Path:          "layout[0].onClick",
		ControlType:   protocol.DataStoreControlDouble,
	}}

	if err := (&ServerBoundDataStoreHandler{h: h}).Handle(pk, s, nil, nil); err != nil {
		t.Fatalf("Handle() error = %v", err)
	}
	if covered.updates != 0 {
		t.Fatal("covered form accepted client update")
	}
}

func TestCloseDDUIFormsUnbindsForm(t *testing.T) {
	f := &recordingDDUIForm{}
	h := &DDUIFormHandler{forms: make(map[uint32]*activeDDUIForm)}
	s := &Session{packets: make(chan packet.Packet, 8), closeBackground: make(chan struct{})}

	h.SendDDUIForm(f, s)
	h.CloseDDUIForms(s)

	if !f.unbound {
		t.Fatal("form bindings were not removed")
	}
}

func TestDiscardDDUIFormsUnbindsForm(t *testing.T) {
	f := &recordingDDUIForm{}
	h := &DDUIFormHandler{forms: make(map[uint32]*activeDDUIForm)}
	s := &Session{packets: make(chan packet.Packet, 8), closeBackground: make(chan struct{})}

	h.SendDDUIForm(f, s)
	h.discardDDUIForms()

	if !f.unbound {
		t.Fatal("discarded form bindings were not removed")
	}
}

type recordingDDUIForm struct {
	updates int
	unbound bool
}

func (*recordingDDUIForm) OnClose(int) {}

func (*recordingDDUIForm) ScreenID() string { return "minecraft:message_box" }

func (*recordingDDUIForm) Describe() ddui.FormDescriptor { return ddui.FormDescriptor{} }

func (f *recordingDDUIForm) HandleUpdate(string, ddui.UpdateValue) bool {
	f.updates++
	return false
}

func (f *recordingDDUIForm) BindSend(func(ddui.UpdateNotification)) func() {
	return func() { f.unbound = true }
}

func dataStoreMapEntry(value protocol.DataStorePropertyValue, key string) (protocol.DataStorePropertyValue, bool) {
	for _, entry := range value.MapValue {
		if entry.Key == key {
			return entry.Value, true
		}
	}
	return protocol.DataStorePropertyValue{}, false
}
