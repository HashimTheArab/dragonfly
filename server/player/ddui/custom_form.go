package ddui

import "strconv"

// FormOption configures a CustomForm.
type FormOption interface {
	applyForm(f *CustomForm)
}

// CloseButtonOption configures a CustomForm close button.
type CloseButtonOption interface{ applyCloseButton(*closeButtonOption) }

type closeButtonOption struct {
	label   *Observable[string]
	visible *Observable[bool]
}

func (o *closeButtonOption) applyForm(f *CustomForm) { f.closeButton = o }

type closeButtonLabelOption struct{ label *Observable[string] }

func (o closeButtonLabelOption) applyCloseButton(button *closeButtonOption) { button.label = o.label }

// WithCloseButtonLabel changes the close button's default label.
func WithCloseButtonLabel[T string | *Observable[string]](label T) closeButtonLabelOption {
	return closeButtonLabelOption{label: toStringObs(label)}
}

// CloseButton adds the form's visible close controls.
func CloseButton(opts ...CloseButtonOption) FormOption {
	button := &closeButtonOption{
		label:   NewObservable("Close", false),
		visible: NewObservable(true, false),
	}
	for _, opt := range opts {
		opt.applyCloseButton(button)
	}
	return button
}

// CustomForm is a fully customisable data-driven UI form. It supports real-time
// value updates in both directions via Observables while the form is open.
type CustomForm struct {
	title        *Observable[string]
	closeButton  *closeButtonOption
	elements     []element
	closeHandler func(reason int)
}

// New creates a CustomForm with the given title and options.
func New[T string | *Observable[string]](title T, opts ...FormOption) *CustomForm {
	f := &CustomForm{title: toStringObs(title)}
	for _, o := range opts {
		o.applyForm(f)
	}
	return f
}

func (f *CustomForm) ScreenID() string { return "minecraft:custom_form" }

func (f *CustomForm) Describe() FormDescriptor {
	descs := make([]ElementDescriptor, len(f.elements))
	for i, e := range f.elements {
		descs[i] = e.describe()
	}
	desc := FormDescriptor{
		Title:          f.title.Get(),
		HasCloseButton: f.closeButton != nil,
		Elements:       descs,
	}
	if f.closeButton != nil {
		desc.CloseButton = ElementDescriptor{
			Kind: ElementButton, Label: f.closeButton.label.Get(), Visible: f.closeButton.visible.Get(),
		}
	}
	return desc
}

func (f *CustomForm) HandleUpdate(path string, value UpdateValue) UpdateResult {
	if path == "closeButton.onClick" && value.Kind == UpdateKindFloat && f.closeButton != nil && f.closeButton.visible.Get() {
		return UpdateResult{Close: true}
	}
	idx, property, ok := parseLayoutPath(path)
	if !ok || idx < 0 || idx >= len(f.elements) {
		return UpdateResult{}
	}
	return UpdateResult{Complete: f.elements[idx].handleUpdate(property, value)}
}

func (f *CustomForm) BindSend(fn func(UpdateNotification)) func() {
	unbinds := make([]func(), 0, len(f.elements)+3)
	unbinds = append(unbinds, bindString(f.title, "title", fn))
	if f.closeButton != nil {
		unbinds = append(unbinds,
			bindString(f.closeButton.label, "closeButton.label", fn),
			bindVisible(f.closeButton.visible, "closeButton", "button_visible", fn),
		)
	}
	for i, e := range f.elements {
		unbinds = append(unbinds, e.bindSend("layout["+strconv.Itoa(i)+"]", fn))
	}
	return func() {
		for _, unbind := range unbinds {
			unbind()
		}
	}
}

func (f *CustomForm) OnClose(reason int) {
	if f.closeHandler != nil {
		f.closeHandler(reason)
	}
}

func parseLayoutPath(path string) (idx int, property string, ok bool) {
	const prefix = "layout["
	if len(path) <= len(prefix) || path[:len(prefix)] != prefix {
		return 0, "", false
	}
	rest := path[len(prefix):]
	bracket := -1
	for i, c := range rest {
		if c == ']' {
			bracket = i
			break
		}
	}
	if bracket < 0 || bracket+2 >= len(rest) || rest[bracket+1] != '.' {
		return 0, "", false
	}
	n, err := strconv.Atoi(rest[:bracket])
	if err != nil {
		return 0, "", false
	}
	return n, rest[bracket+2:], true
}
