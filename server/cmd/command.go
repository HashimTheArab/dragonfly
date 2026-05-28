package cmd

import (
	"encoding/csv"
	"fmt"
	"go/ast"
	"reflect"
	"slices"
	"strings"

	"github.com/df-mc/dragonfly/server/world"
)

// Runnable represents a Command that may be run by any Command source. The Command must be a struct type and
// its fields represent the parameters of the Command. Commands that only support a specific source type may
// implement RunnableFor and be registered with NewFor instead. When the Run method is called, these fields
// are set and may be used for behaviour in the Command. Fields unexported or ignored using the `cmd:"-"`
// struct tag (see below) have their values copied but retained.
// A Runnable or RunnableFor may have exported fields only of the following types:
// int8, int16, int32, int64, int, uint8, uint16, uint32, uint64, uint,
// float32, float64, string, bool, mgl64.Vec3, Varargs, []Target, cmd.SubCommand, Optional[T] (to make a parameter
// optional), or a type that implements the cmd.Parameter or cmd.Enum interface. cmd.Enum implementations must be of the
// type string.
// Fields in the Runnable struct may have `cmd:` struct tag to specify the name and suffix of a parameter as such:
//
//	type T struct {
//	    Param int `cmd:"name,suffix"`
//	}
//
// If no name is set, the field name is used. Additionally, the name as specified in the struct tag may be '-' to make
// the parser ignore the field. In this case, the field does not have to be of one of the types above.
type Runnable interface {
	// Run runs the Command, using the arguments passed to the Command. The source is passed to the method,
	// which is the source of the Command execution, and the output is passed, to which messages may be
	// added which get sent to the source.
	Run(src Source, o *Output, tx *world.Tx)
}

// Context holds the values available while running a typed command.
type Context[S Source] struct {
	// Source is the typed source that executed the command.
	Source S
	// Output holds messages that will be sent to Source after the command has run.
	Output *Output
	// Tx is the world transaction the command is currently running in.
	Tx *world.Tx
}

// RunnableFor represents a Command that may only be run by command sources assignable to S.
// RunnableFor implementations are registered using NewFor.
type RunnableFor[S Source] interface {
	// Run runs the Command, using the arguments passed to the Command. The Context passed contains the
	// typed source of the Command execution and the output to which messages may be added.
	Run(ctx *Context[S])
}

// Allower may be implemented by a type also implementing Runnable to limit the sources that may run the
// command.
type Allower interface {
	// Allow checks if the Source passed is allowed to execute the command. True is returned if the Source is
	// allowed to execute the command.
	Allow(src Source) bool
}

// AllowerFor may be implemented by a type also implementing RunnableFor to limit the typed sources that may
// run the command.
type AllowerFor[S Source] interface {
	// Allow checks if the source passed is allowed to execute the command. True is returned if the source is
	// allowed to execute the command.
	Allow(src S) bool
}

type commandRunnable struct {
	value    reflect.Value
	run      func(v reflect.Value, src Source, o *Output, tx *world.Tx)
	allow    func(v reflect.Value, src Source) bool
	runnable func(v reflect.Value) Runnable
}

// Command is a wrapper around a Runnable. It provides additional identity and utility methods for the actual
// runnable command so that it may be identified more easily.
type Command struct {
	v           []commandRunnable
	name        string
	description string
	usage       string
	aliases     []string
}

// New returns a new Command using the name and description passed. Command
// names and aliases are all converted to lowercase. The Runnable passed must
// be a (pointer to a) struct, with its fields representing the parameters of
// the command. When the command is run, the Run method of the Runnable will be
// called after all fields have their values from the parsed command set. If r
// is not a struct or a pointer to a struct, New panics.
func New(name, description string, aliases []string, r ...Runnable) Command {
	runnables := make([]commandRunnable, len(r))
	for i, runnable := range r {
		value := runnableValue(runnable)
		runnables[i] = commandRunnable{
			value: value,
			run: func(v reflect.Value, src Source, o *Output, tx *world.Tx) {
				v.Interface().(Runnable).Run(src, o, tx)
			},
			allow: func(v reflect.Value, src Source) bool {
				a, ok := v.Interface().(Allower)
				return !ok || a.Allow(src)
			},
			runnable: func(v reflect.Value) Runnable {
				if r, ok := interfaceFromValue[Runnable](value); ok {
					return r
				}
				return v.Interface().(Runnable)
			},
		}
	}
	return newCommand(name, description, aliases, runnables)
}

// NewFor returns a new Command that may only be run by sources assignable to S. Command names and aliases
// are all converted to lowercase. The RunnableFor values passed must be a (pointer to a) struct, with their
// fields representing the parameters of the command. When the command is run, the Run method of the
// RunnableFor will be called with a Context holding the typed source and parsed output. If r is not a struct
// or a pointer to a struct, NewFor panics.
func NewFor[S Source](name, description string, aliases []string, r ...RunnableFor[S]) Command {
	runnables := make([]commandRunnable, len(r))
	for i, runnable := range r {
		runnables[i] = commandRunnable{
			value: runnableValue(runnable),
			run: func(v reflect.Value, src Source, o *Output, tx *world.Tx) {
				typedSrc, ok := src.(S)
				if !ok {
					return
				}
				v.Interface().(RunnableFor[S]).Run(&Context[S]{Source: typedSrc, Output: o, Tx: tx})
			},
			allow: func(v reflect.Value, src Source) bool {
				typedSrc, ok := src.(S)
				if !ok {
					return false
				}
				if a, ok := v.Interface().(AllowerFor[S]); ok && !a.Allow(typedSrc) {
					return false
				}
				if a, ok := v.Interface().(Allower); ok && !a.Allow(src) {
					return false
				}
				return true
			},
			runnable: func(v reflect.Value) Runnable {
				return typedRunnable[S]{v: v}
			},
		}
	}
	return newCommand(name, description, aliases, runnables)
}

func newCommand(name, description string, aliases []string, r []commandRunnable) Command {
	name = strings.ToLower(name)
	for i, alias := range aliases {
		aliases[i] = strings.ToLower(alias)
	}

	usages := make([]string, len(r))

	if len(aliases) > 0 && slices.Index(aliases, name) == -1 {
		aliases = append(aliases, name)
	}

	for i, runnable := range r {
		cp := reflect.New(runnable.value.Type()).Elem()
		if err := verifySignature(cp); err != nil {
			panic(err.Error())
		}
		usages[i] = parseUsage(name, cp)
	}

	return Command{name: name, description: description, aliases: aliases, v: r, usage: strings.Join(usages, "\n")}
}

func runnableValue(r any) reflect.Value {
	if r == nil {
		panic("Runnable r must be struct or pointer to struct, but got <nil>")
	}
	t := reflect.TypeOf(r)
	if t.Kind() != reflect.Struct && (t.Kind() != reflect.Ptr || t.Elem().Kind() != reflect.Struct) {
		panic(fmt.Sprintf("Runnable r must be struct or pointer to struct, but got %v", t.Kind()))
	}
	original := reflect.ValueOf(r)
	if t.Kind() == reflect.Ptr {
		if original.IsNil() {
			panic("Runnable r must be struct or pointer to struct, but got nil pointer")
		}
		original = original.Elem()
	}
	return original
}

func newRunnableValue(original reflect.Value) reflect.Value {
	cp := reflect.New(original.Type())
	cp.Elem().Set(original)
	return cp
}

func interfaceFromValue[T any](v reflect.Value) (T, bool) {
	if v.CanInterface() {
		if t, ok := v.Interface().(T); ok {
			return t, true
		}
	}
	if v.CanAddr() {
		if t, ok := v.Addr().Interface().(T); ok {
			return t, true
		}
	}
	cp := newRunnableValue(v)
	if t, ok := cp.Interface().(T); ok {
		return t, true
	}
	var zero T
	return zero, false
}

type typedRunnable[S Source] struct {
	v reflect.Value
}

// Run adapts a RunnableFor to Runnable for APIs that still expose runnable values.
func (r typedRunnable[S]) Run(src Source, o *Output, tx *world.Tx) {
	typedSrc, ok := src.(S)
	if !ok {
		return
	}
	r.v.Interface().(RunnableFor[S]).Run(&Context[S]{Source: typedSrc, Output: o, Tx: tx})
}

// Name returns the name of the command. The name is guaranteed to be lowercase and will never have spaces in
// it. This name is used to call the command, and is shown in the /help list.
func (cmd Command) Name() string {
	return cmd.name
}

// Description returns the description of the command. The description is shown in the /help list, and
// provides information on the functionality of a command.
func (cmd Command) Description() string {
	return cmd.description
}

// Usage returns the usage of the command. The usage will be roughly equal to the one showed by the client
// in-game.
func (cmd Command) Usage() string {
	return cmd.usage
}

// Aliases returns a list of aliases for the command. In addition to the name of the command, the command may
// be called using one of these aliases.
func (cmd Command) Aliases() []string {
	return cmd.aliases
}

// Execute executes the Command as a source with the args passed. The args are parsed assuming they do not
// start with the command name. Execute will attempt to parse and execute one Runnable at a time. If one of
// the Runnable was able to parse args correctly, it will be executed and no more Runnables will be attempted
// to be run.
// If parsing of all Runnables was unsuccessful, a command output with an error message is sent to the Source
// passed, and the Run method of the Runnables are not called.
// The Source passed must not be nil. The method will panic if a nil Source is passed.
func (cmd Command) Execute(args string, source Source, tx *world.Tx) {
	if source == nil {
		panic("execute: invalid command source: source must not be nil")
	}
	output := &Output{}
	defer source.SendCommandOutput(output)

	var leastErroneous error
	var leastArgsLeft *Line

	for _, runnable := range cmd.v {
		line, err := cmd.executeRunnable(runnable, args, source, output, tx)
		if err == nil {
			// Command was executed successfully: We won't execute any of the other Runnable values passed, as
			// we've already found an overload that works.
			return
		}
		if line == nil {
			// This Runnable was not runnable by the source passed. Only if no error was yet set, we set an
			// error for the wrong source.
			if leastErroneous == nil {
				leastErroneous = err
			}
			continue
		}
		if leastArgsLeft == nil || line.Len() <= leastArgsLeft.Len() {
			// If the line had less (or equal) arguments left than the previous lowest, we update the error,
			// so that we can return an error that applies for the most successful Runnable.
			leastErroneous = err
			leastArgsLeft = line
		}
	}
	// No working Runnable found for the arguments passed. We add the most
	// applicable error to the output and stop there.
	if leastArgsLeft != nil {
		output.Error(leastArgsLeft.SyntaxError())
	}
	output.Error(leastErroneous)
}

// ParamInfo holds the information of a parameter in a Runnable. Information of a parameter may be obtained
// by calling Command.Params().
type ParamInfo struct {
	Name     string
	Value    any
	Optional bool
	Suffix   string
}

// Params returns a list of all parameters of the runnables. No assumptions should be done on the values that
// they hold: Only the types are guaranteed to be consistent.
func (cmd Command) Params(src Source) [][]ParamInfo {
	params := make([][]ParamInfo, 0, len(cmd.v))
	for _, runnable := range cmd.v {
		if !runnable.allow(newRunnableValue(runnable.value), src) {
			// This source cannot execute this runnable.
			continue
		}

		// If the runnable can describe its own parameters, prefer that over reflection.
		if d, ok := interfaceFromValue[ParamDescriber](runnable.value); ok {
			params = append(params, d.DescribeParams(src))
			continue
		}

		elem := reflect.New(runnable.value.Type()).Elem()
		elem.Set(runnable.value)

		var fields []ParamInfo
		for _, t := range exportedFields(elem) {
			field := elem.FieldByName(t.Name)
			fields = append(fields, ParamInfo{
				Name:     name(t),
				Value:    unwrap(field).Interface(),
				Optional: optional(field),
				Suffix:   suffix(t),
			})
		}
		params = append(params, fields)
	}
	return params
}

// Runnables returns a map of all Runnable implementations of the Command that a Source can execute.
func (cmd Command) Runnables(src Source) map[int]Runnable {
	m := make(map[int]Runnable, len(cmd.v))
	for i, runnable := range cmd.v {
		v := newRunnableValue(runnable.value)
		if runnable.allow(v, src) {
			m[i] = runnable.runnable(v)
		}
	}
	return m
}

// String returns the usage of the command. The usage will be roughly equal to the one showed by the client
// in-game.
func (cmd Command) String() string {
	return cmd.usage
}

// executeRunnable executes a Runnable v, by parsing the args passed using the source and output obtained. If
// parsing was not successful or the Runnable could not be run by this source, an error is returned, and the
// leftover command line.
func (cmd Command) executeRunnable(runnable commandRunnable, args string, source Source, output *Output, tx *world.Tx) (*Line, error) {
	v := newRunnableValue(runnable.value)
	if !runnable.allow(v, source) {
		return nil, MessageUnknown.F(cmd.name)
	}

	var argFrags []string
	if args != "" {
		r := csv.NewReader(strings.NewReader(args))
		r.Comma, r.LazyQuotes = ' ', true
		record, err := r.Read()
		if err != nil {
			// When LazyQuotes is enabled, this really never appears to return
			// an error when we read only one line. Just in case it does though,
			// we return the command usage.
			return nil, MessageUsage.F(cmd.Usage())
		}
		argFrags = record
	}
	parser := parser{}
	arguments := &Line{args: argFrags, src: source, seen: []string{"/" + cmd.name}, cmd: cmd}

	// We iterate over all the fields of the struct: Each of the fields will have an argument parsed to
	// produce its value.
	signature := v.Elem()
	for _, t := range exportedFields(signature) {
		field := signature.FieldByName(t.Name)
		parser.currentField = t.Name
		opt := optional(field)

		val := field
		if opt {
			val = reflect.New(field.Field(0).Type()).Elem()
		}

		err, success := parser.parseArgument(arguments, val, opt, name(t), source, tx)
		if err != nil {
			// Parsing was not successful, we return immediately as we don't
			// need to call the Runnable.
			return arguments, err
		}
		if success && opt {
			field.Set(reflect.ValueOf(field.Interface().(optionalT).with(val.Interface())))
		}
	}
	if arguments.Len() != 0 {
		return arguments, arguments.UsageError()
	}

	runnable.run(v, source, output, tx)
	return arguments, nil
}

// parseUsage parses the usage of a command found in value v using the name passed. It accounts for optional
// parameters and converts types to a more friendly representation.
func parseUsage(commandName string, command reflect.Value) string {
	parts := make([]string, 0, command.NumField()+1)
	parts = append(parts, "/"+commandName)

	for _, t := range exportedFields(command) {
		field := command.FieldByName(t.Name)

		typeName := typeNameOf(field.Interface(), name(t))
		if _, ok := field.Interface().(optionalT); ok {
			typeName = typeNameOf(reflect.New(field.Field(0).Type()).Elem().Interface(), name(t))
		}
		if _, ok := field.Interface().(SubCommand); ok {
			parts = append(parts, typeName)
			continue
		}
		if optional(field) {
			parts = append(parts, "["+name(t)+": "+typeName+"]"+suffix(t))
			continue
		}
		parts = append(parts, "<"+name(t)+": "+typeName+">"+suffix(t))
	}
	return strings.Join(parts, " ")
}

// verifySignature verifies the passed struct pointer value signature to ensure it is a valid command,
// checking things such as the validity of the optional struct tags.
// If not valid, an error is returned.
func verifySignature(command reflect.Value) error {
	optionalField := false
	for _, t := range exportedFields(command) {
		field := command.FieldByName(t.Name)

		// If the field is not optional, while the last field WAS optional, we return an error, as this is
		// not parsable in an expected way.
		opt := optional(field)
		if !opt && optionalField {
			return fmt.Errorf("command must only have optional parameters at the end")
		}
		val := field
		if opt {
			val = reflect.New(field.Field(0).Type()).Elem()
		}
		if _, ok := val.Interface().(Enum); ok && val.Kind() != reflect.String {
			return fmt.Errorf("parameters implementing Enum must be of the type string")
		}
		optionalField = opt
	}
	return nil
}

// exportedFields returns all exported struct fields of the reflect.Value passed. It returns the fields as returned by
// reflect.VisibleFields, but filters out unexported fields, anonymous fields and fields that have a name value in the
// 'cmd' tag of '-'.
func exportedFields(command reflect.Value) []reflect.StructField {
	visible := reflect.VisibleFields(command.Type())
	fields := make([]reflect.StructField, 0, len(visible))

	for _, t := range visible {
		if !ast.IsExported(t.Name) || name(t) == "-" || t.Anonymous {
			continue
		}
		field := command.FieldByName(t.Name)
		if !field.CanSet() {
			continue
		}
		fields = append(fields, t)
	}
	return fields
}
