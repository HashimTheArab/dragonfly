package session

import (
	"fmt"
	"math"
	"slices"

	"github.com/df-mc/dragonfly/server/cmd"
	"github.com/go-gl/mathgl/mgl64"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
	"golang.org/x/text/language"
)

// SendCommandOutput sends the output of a command to the player. It will be shown to the caller of the
// command, which might be the player or a websocket server.
func (s *Session) SendCommandOutput(output *cmd.Output, l language.Tag) {
	if s == Nop {
		return
	}
	messages := make([]protocol.CommandOutputMessage, 0, output.MessageCount()+output.ErrorCount())
	for _, message := range output.Messages() {
		om := protocol.CommandOutputMessage{Success: true, Message: message.String()}
		if t, ok := message.(translation); ok {
			om.Message, om.Parameters = t.Resolve(l), t.Params(l)
		}
		messages = append(messages, om)
	}
	for _, err := range output.Errors() {
		om := protocol.CommandOutputMessage{Message: err.Error()}
		if t, ok := err.(translation); ok {
			om.Message, om.Parameters = t.Resolve(l), t.Params(l)
		}
		messages = append(messages, om)
	}

	s.writePacket(&packet.CommandOutput{
		CommandOrigin:  s.handlers[packet.IDCommandRequest].(*CommandRequestHandler).origin,
		OutputType:     packet.CommandOutputTypeAllOutput,
		SuccessCount:   uint32(output.MessageCount()),
		OutputMessages: messages,
	})
}

type translation interface {
	Resolve(l language.Tag) string
	Params(l language.Tag) []string
}

// CommandSchemaConfig adjusts how AppendAvailableCommands advertises commands.
type CommandSchemaConfig struct {
	// SoftEnums names the enums advertised as dynamic enums.
	SoftEnums map[string]struct{}
	// Rename maps advertised command names and alias options. Nil keeps them.
	Rename func(string) string
}

// AppendAvailableCommands appends the commands src can run to pk with indices into pk's tables and returns
// the runnable commands by name. Existing entries of pk are never written; an enum requested without options
// reuses pk's existing enum of that type. On error, such as an index beyond the packed parameter width, pk is
// unchanged.
func AppendAvailableCommands(
	pk *packet.AvailableCommands,
	commands map[string]cmd.Command,
	src cmd.Source,
	conf CommandSchemaConfig,
) (map[string]map[int]cmd.Runnable, error) {
	rename := conf.Rename
	if rename == nil {
		rename = func(s string) string { return s }
	}
	// Clipping makes every append below copy instead of writing into a backing array shared with pk.
	out := *pk
	out.EnumValues, out.Enums = slices.Clip(out.EnumValues), slices.Clip(out.Enums)
	out.Suffixes, out.DynamicEnums, out.Commands = slices.Clip(out.Suffixes), slices.Clip(out.DynamicEnums), slices.Clip(out.Commands)

	existingEnums := make(map[string]uint32, len(pk.Enums))
	for i, enum := range pk.Enums {
		existingEnums[enum.Type] = uint32(i)
	}
	enumIndices, valueIndices := map[string]uint32{}, map[string]uint32{}
	suffixIndices, dynamicEnumIndices := map[string]uint32{}, map[string]uint32{}
	appendEnum := func(enum commandEnum) uint32 {
		index := uint32(len(out.Enums))
		protoEnum := protocol.CommandEnum{Type: enum.Type}
		for _, opt := range enum.Options {
			valueIndex, ok := valueIndices[opt]
			if !ok {
				valueIndex = uint32(len(out.EnumValues))
				valueIndices[opt] = valueIndex
				out.EnumValues = append(out.EnumValues, opt)
			}
			protoEnum.ValueIndices = append(protoEnum.ValueIndices, valueIndex)
		}
		out.Enums = append(out.Enums, protoEnum)
		enumIndices[enum.Type] = index
		return index
	}

	m := make(map[string]map[int]cmd.Runnable, len(commands))
	for alias, c := range commands {
		if c.Name() != alias {
			// Don't add duplicate entries for aliases.
			continue
		}
		run := c.Runnables(src)
		if len(run) == 0 {
			continue
		}
		m[alias] = run

		aliasesIndex := uint32(math.MaxUint32)
		if aliases := c.Aliases(); len(aliases) > 0 {
			options := make([]string, len(aliases))
			for i, a := range aliases {
				options[i] = rename(a)
			}
			aliasesIndex = appendEnum(commandEnum{Type: c.Name() + "Aliases", Options: options})
		}

		params := c.Params(src)
		overloads := make([]protocol.CommandOverload, len(params))
		for i, params := range params {
			for _, paramInfo := range params {
				t, enum := valueToParamType(paramInfo, src)
				var index uint32
				if suffix := paramInfo.Suffix; suffix != "" {
					var ok bool
					if index, ok = suffixIndices[suffix]; !ok {
						index = uint32(len(out.Suffixes))
						suffixIndices[suffix] = index
						out.Suffixes = append(out.Suffixes, suffix)
					}
					t = protocol.CommandArgSuffixed | index
				} else {
					t |= protocol.CommandArgValid
					if len(enum.Options) > 0 || enum.Type != "" {
						if _, dynamic := conf.SoftEnums[enum.Type]; dynamic {
							var ok bool
							if index, ok = dynamicEnumIndices[enum.Type]; !ok {
								index = uint32(len(out.DynamicEnums))
								dynamicEnumIndices[enum.Type] = index
								out.DynamicEnums = append(out.DynamicEnums, protocol.DynamicEnum{Type: enum.Type, Values: enum.Options})
							}
							t |= protocol.CommandArgSoftEnum | index
						} else {
							var ok bool
							if index, ok = enumIndices[enum.Type]; !ok {
								if existing, found := existingEnums[enum.Type]; found && len(enum.Options) == 0 {
									index = existing
									enumIndices[enum.Type] = index
								} else {
									index = appendEnum(enum)
								}
							}
							t |= protocol.CommandArgEnum | index
						}
					}
				}
				if index > protocol.CommandArgIndexMask {
					return nil, fmt.Errorf("command %v: table index %v exceeds the parameter type width", c.Name(), index)
				}
				overloads[i].Parameters = append(overloads[i].Parameters, protocol.CommandParameter{
					Name:     paramInfo.Name,
					Type:     t,
					Optional: paramInfo.Optional,
				})
			}
		}
		out.Commands = append(out.Commands, protocol.Command{
			Name:            rename(c.Name()),
			Description:     c.Description(),
			AliasesOffset:   aliasesIndex,
			PermissionLevel: protocol.CommandPermissionLevelAny,
			Overloads:       overloads,
		})
	}
	*pk = out
	return m, nil
}

// sendAvailableCommands sends all available commands of the server. Once sent, they will be visible in the
// /help list and will be auto-completed.
func (s *Session) sendAvailableCommands(co Controllable, softEnums map[string]struct{}) map[string]map[int]cmd.Runnable {
	pk := &packet.AvailableCommands{}
	m, err := AppendAvailableCommands(pk, cmd.Commands(), co, CommandSchemaConfig{SoftEnums: softEnums})
	if err != nil {
		s.conf.Log.Error("build available commands", "err", err)
		return nil
	}
	s.writePacket(pk)
	return m
}

type commandEnum struct {
	Type    string
	Options []string
}

// valueToParamType finds the command argument type of the value passed and returns it, in addition to creating
// an enum if applicable.
func valueToParamType(i cmd.ParamInfo, source cmd.Source) (t uint32, enum commandEnum) {
	switch i.Value.(type) {
	case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64:
		return protocol.CommandArgTypeInt, enum
	case float32, float64:
		return protocol.CommandArgTypeFloat, enum
	case string:
		return protocol.CommandArgTypeString, enum
	case cmd.Varargs:
		return protocol.CommandArgTypeRawText, enum
	case cmd.Target, []cmd.Target:
		return protocol.CommandArgTypeTarget, enum
	case bool:
		return 0, commandEnum{
			Type:    "Boolean",
			Options: []string{"true", "false"},
		}
	case mgl64.Vec3:
		return protocol.CommandArgTypePosition, enum
	case cmd.SubCommand:
		return 0, commandEnum{
			Type:    "SubCommand" + i.Name,
			Options: []string{i.Name},
		}
	}
	if enum, ok := i.Value.(cmd.Enum); ok {
		return 0, commandEnum{
			Type:    enum.Type(),
			Options: enum.Options(source),
		}
	}
	return protocol.CommandArgTypeRValue, enum
}

// resendCommands resends all commands that a Session has access to if the map of runnable commands passed does not
// match with the commands that the Session is currently allowed to execute.
// True is returned if the commands were resent.
func (s *Session) resendCommands(
	before map[string]map[int]cmd.Runnable,
	co Controllable,
	softEnums map[string]struct{},
) (map[string]map[int]cmd.Runnable, bool) {
	commands := cmd.Commands()
	m := make(map[string]map[int]cmd.Runnable, len(commands))

	for alias, c := range commands {
		if c.Name() == alias {
			if run := c.Runnables(co); len(run) > 0 {
				m[alias] = run
			}
		}
	}
	if len(before) != len(m) {
		return s.sendAvailableCommands(co, softEnums), true
	}
	// First check for commands that were newly added.
	for name, r := range m {
		for k := range r {
			if _, ok := before[name][k]; !ok {
				return s.sendAvailableCommands(co, softEnums), true
			}
		}
	}
	// Then check for commands that a player could execute before, but no longer can.
	for name, r := range before {
		for k := range r {
			if _, ok := m[name][k]; !ok {
				return s.sendAvailableCommands(co, softEnums), true
			}
		}
	}
	return m, false
}

// enums returns a map of all enums exposed to the Session and records the values those enums currently hold.
func (s *Session) enums(co Controllable) (map[string]cmd.Enum, map[string][]string) {
	enums, enumValues := make(map[string]cmd.Enum), make(map[string][]string)
	for alias, c := range cmd.Commands() {
		if c.Name() == alias {
			for _, params := range c.Params(co) {
				for _, paramInfo := range params {
					if enum, ok := paramInfo.Value.(cmd.Enum); ok {
						enums[enum.Type()] = enum
						enumValues[enum.Type()] = enum.Options(co)
					}
				}
			}
		}
	}
	return enums, enumValues
}

// resendEnums checks the options of the enums passed against the values that were previously recorded. If they do not
// match, and the enum is in softEnums, the enum is resent via UpdateSoftEnum. If the enum is not yet in softEnums,
// it is added and the full AvailableCommands packet is resent.
func (s *Session) resendEnums(
	enums map[string]cmd.Enum,
	before map[string][]string,
	softEnums map[string]struct{},
	r map[string]map[int]cmd.Runnable,
	c Controllable,
) map[string]map[int]cmd.Runnable {
	for name, enum := range enums {
		valuesBefore := before[name]
		values := enum.Options(c)
		before[name] = values

		changed := false
		if len(valuesBefore) != len(values) {
			changed = true
		} else {
			for k, v := range values {
				if valuesBefore[k] != v {
					changed = true
					break
				}
			}
		}
		if !changed {
			continue
		}

		if _, dynamic := softEnums[name]; dynamic {
			s.writePacket(&packet.UpdateSoftEnum{EnumType: name, Options: values, ActionType: packet.SoftEnumActionSet})
		} else {
			softEnums[name] = struct{}{}
			r = s.sendAvailableCommands(c, softEnums)
		}
	}
	return r
}
