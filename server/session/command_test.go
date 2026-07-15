package session

import (
	"reflect"
	"testing"

	"github.com/df-mc/dragonfly/server/cmd"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
)

type customCommandParameter struct{}

func (customCommandParameter) Parse(*cmd.Line, reflect.Value) error { return nil }
func (customCommandParameter) Type() string                         { return "custom" }

func TestValueToParamTypeUsesStringForCustomParameter(t *testing.T) {
	typ, _ := valueToParamType(cmd.ParamInfo{Value: customCommandParameter{}}, nil)
	if typ != protocol.CommandArgTypeString {
		t.Fatalf("custom parameter type = %d, want string type %d", typ, protocol.CommandArgTypeString)
	}
}
