package ddui

import "testing"

func TestCustomFormCloseButtonClosesForm(t *testing.T) {
	f := New("title", CloseButton())

	if !f.HandleUpdate("closeButton.onClick", UpdateValue{Kind: UpdateKindFloat}) {
		t.Fatal("close button update did not close form")
	}
}

func TestMessageBoxClearsSelectionAfterClose(t *testing.T) {
	var selections []int
	m := NewMessageBox("title", Handler(func(selection int) {
		selections = append(selections, selection)
	}))

	m.HandleUpdate("button1.onClick", UpdateValue{Kind: UpdateKindFloat})
	m.OnClose(Closed)
	m.OnClose(Closed)

	if got, want := selections, []int{1, 0}; len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Fatalf("selections = %v, want %v", got, want)
	}
}

func TestObservableBindSendSupportsMultipleBindings(t *testing.T) {
	obs := NewObservable(0, false)
	var first, second int
	unbindFirst := obs.bindSend(func(value int) { first = value })
	obs.bindSend(func(value int) { second = value })

	obs.Set(7)
	unbindFirst()
	obs.Set(9)

	if first != 7 || second != 9 {
		t.Fatalf("bound values = (%v, %v), want (7, 9)", first, second)
	}
}

func TestSliderUsesIntegerValues(t *testing.T) {
	value := NewObservable(int64(3), true)
	f := New("title", Slider("value", value, 1, 5, WithStep(2)))

	desc := f.Describe().Elements[0]
	if desc.Int64Value != 3 || desc.Min != 1 || desc.Max != 5 || desc.Step != 2 {
		t.Fatalf("slider descriptor = value %v, range [%v,%v], step %v", desc.Int64Value, desc.Min, desc.Max, desc.Step)
	}
}
