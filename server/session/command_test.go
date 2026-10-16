package session

import (
	"math"
	"reflect"
	"strings"
	"testing"

	"github.com/df-mc/dragonfly/server/cmd"
	"github.com/df-mc/dragonfly/server/world"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

type schemaEnum struct {
	typ     string
	options []string
}

func (e schemaEnum) Type() string                { return e.typ }
func (e schemaEnum) Options(cmd.Source) []string { return e.options }

type schemaRunnable struct{ params []cmd.ParamInfo }

func (schemaRunnable) Run(cmd.Source, *cmd.Output, *world.Tx)      {}
func (r schemaRunnable) DescribeParams(cmd.Source) []cmd.ParamInfo { return r.params }

func schemaCommands(params ...cmd.ParamInfo) map[string]cmd.Command {
	c := cmd.New("Warp", "teleports", []string{"w"}, schemaRunnable{params: params})
	commands := map[string]cmd.Command{c.Name(): c}
	for _, alias := range c.Aliases() {
		commands[alias] = c
	}
	return commands
}

func enumOptions(pk *packet.AvailableCommands, index uint32) []string {
	var options []string
	for _, value := range pk.Enums[index].ValueIndices {
		options = append(options, pk.EnumValues[value])
	}
	return options
}

// Appending must index into the existing tables without writing their shared backing arrays.
func TestAppendAvailableCommands_AppendsWithFinalIndicesWithoutWritingBaseline(t *testing.T) {
	server := &packet.AvailableCommands{
		EnumValues: append(make([]string, 0, 8), "stone", "true"),
		Enums: append(make([]protocol.CommandEnum, 0, 8),
			protocol.CommandEnum{Type: "Block"},
			protocol.CommandEnum{Type: "Boolean", ValueIndices: []uint32{1}},
		),
		Suffixes: append(make([]string, 0, 8), "L"),
		Commands: append(make([]protocol.Command, 0, 8), protocol.Command{Name: "server", AliasesOffset: math.MaxUint32}),
	}
	baseline := *server
	baselineValues := slicesOf(server)

	pk := *server
	_, err := AppendAvailableCommands(&pk, schemaCommands(
		cmd.ParamInfo{Name: "block", Value: schemaEnum{typ: "Block"}},
		cmd.ParamInfo{Name: "flag", Value: true},
		cmd.ParamInfo{Name: "level", Value: 0, Suffix: "L"},
		cmd.ParamInfo{Name: "kind", Value: schemaEnum{typ: "Kind", options: []string{"a", "b"}}},
	), nil, CommandSchemaConfig{Rename: func(s string) string { return "." + s }})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(slicesOf(&baseline), baselineValues) || !reflect.DeepEqual(slicesOf(server), baselineValues) {
		t.Fatal("appending changed the baseline tables")
	}
	if len(pk.Commands) != 2 || pk.Commands[0].Name != "server" {
		t.Fatalf("commands = %+v", pk.Commands)
	}
	added := pk.Commands[1]
	if added.Name != ".warp" || !reflect.DeepEqual(enumOptions(&pk, added.AliasesOffset), []string{".w", ".warp"}) {
		t.Fatalf("command name or aliases not renamed: %+v %v", added, enumOptions(&pk, added.AliasesOffset))
	}
	params := added.Overloads[0].Parameters
	if index := params[0].Type & protocol.CommandArgIndexMask; index != 0 {
		t.Fatalf("empty Block enum not reused: index %v", index)
	}
	if index := params[1].Type & protocol.CommandArgIndexMask; index < 2 || !reflect.DeepEqual(enumOptions(&pk, index), []string{"true", "false"}) {
		t.Fatalf("populated Boolean enum reused or wrong: index %v", index)
	}
	if index := params[2].Type & protocol.CommandArgIndexMask; params[2].Type&protocol.CommandArgSuffixed == 0 || pk.Suffixes[index] != "L" || index == 0 {
		t.Fatalf("suffix not appended: type %x suffixes %v", params[2].Type, pk.Suffixes)
	}
	if index := params[3].Type & protocol.CommandArgIndexMask; !reflect.DeepEqual(enumOptions(&pk, index), []string{"a", "b"}) {
		t.Fatalf("Kind enum = %v", enumOptions(&pk, index))
	}
}

// An index that cannot be packed into a parameter type must leave pk unchanged.
func TestAppendAvailableCommands_RejectsOverflowWithoutChangingPacket(t *testing.T) {
	pk := &packet.AvailableCommands{Suffixes: make([]string, protocol.CommandArgIndexMask+1)}
	before := *pk
	_, err := AppendAvailableCommands(pk, schemaCommands(cmd.ParamInfo{Name: "level", Value: 0, Suffix: "L"}), nil, CommandSchemaConfig{})
	if err == nil || !strings.Contains(err.Error(), "width") {
		t.Fatalf("err = %v", err)
	}
	if len(pk.Suffixes) != len(before.Suffixes) || len(pk.Commands) != 0 || len(pk.Enums) != 0 {
		t.Fatal("failed append changed the packet")
	}
}

func slicesOf(pk *packet.AvailableCommands) []any {
	return []any{
		append([]string(nil), pk.EnumValues[:cap(pk.EnumValues)]...),
		append([]protocol.CommandEnum(nil), pk.Enums[:cap(pk.Enums)]...),
		append([]string(nil), pk.Suffixes[:cap(pk.Suffixes)]...),
		append([]protocol.Command(nil), pk.Commands[:cap(pk.Commands)]...),
	}
}
