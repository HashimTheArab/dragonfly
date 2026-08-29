package cmd

import (
	"testing"

	"github.com/df-mc/dragonfly/server/world"
	"github.com/go-gl/mathgl/mgl64"
)

type testSource struct {
	output *Output
}

func (s *testSource) Position() mgl64.Vec3 { return mgl64.Vec3{} }

func (s *testSource) SendCommandOutput(o *Output) {
	s.output = o
}

type typedTestSource struct {
	testSource
	allowed bool
}

type otherTestSource struct {
	testSource
}

type typedEchoCommand struct {
	Message string
}

var typedEchoRun struct {
	source  *typedTestSource
	output  *Output
	tx      *world.Tx
	message string
}

func (c typedEchoCommand) Run(ctx *Context[*typedTestSource]) {
	typedEchoRun.source = ctx.Source
	typedEchoRun.output = ctx.Output
	typedEchoRun.tx = ctx.Tx
	typedEchoRun.message = c.Message
	ctx.Output.Print(c.Message)
}

func TestNewForRunsWithTypedSource(t *testing.T) {
	typedEchoRun = struct {
		source  *typedTestSource
		output  *Output
		tx      *world.Tx
		message string
	}{}

	command := NewFor[*typedTestSource]("echo", "echoes a message", nil, typedEchoCommand{})
	source := &typedTestSource{}

	command.Execute("hello", source, nil)

	if typedEchoRun.source != source {
		t.Fatalf("typed command source = %v, want %v", typedEchoRun.source, source)
	}
	if typedEchoRun.output == nil {
		t.Fatal("typed command output was nil")
	}
	if typedEchoRun.tx != nil {
		t.Fatalf("typed command tx = %v, want nil", typedEchoRun.tx)
	}
	if typedEchoRun.message != "hello" {
		t.Fatalf("typed command message = %q, want hello", typedEchoRun.message)
	}
	if source.output == nil || source.output.MessageCount() != 1 || source.output.Messages()[0].String() != "hello" {
		t.Fatalf("source output = %#v, want one hello message", source.output)
	}
}

func TestNewForFiltersOtherSources(t *testing.T) {
	typedEchoRun = struct {
		source  *typedTestSource
		output  *Output
		tx      *world.Tx
		message string
	}{}

	command := NewFor[*typedTestSource]("echo", "echoes a message", nil, typedEchoCommand{})
	source := &otherTestSource{}

	if params := command.Params(source); len(params) != 0 {
		t.Fatalf("Params returned %d overloads for wrong source type, want 0", len(params))
	}
	if runnables := command.Runnables(source); len(runnables) != 0 {
		t.Fatalf("Runnables returned %d overloads for wrong source type, want 0", len(runnables))
	}

	command.Execute("hello", source, nil)

	if typedEchoRun.source != nil {
		t.Fatalf("typed command ran with wrong source type: %v", typedEchoRun.source)
	}
	if source.output == nil || source.output.ErrorCount() == 0 {
		t.Fatalf("source output = %#v, want command error", source.output)
	}
}

type typedAllowedCommand struct{}

var typedAllowedRan bool

func (typedAllowedCommand) Allow(src *typedTestSource) bool {
	return src.allowed
}

func (typedAllowedCommand) Run(ctx *Context[*typedTestSource]) {
	typedAllowedRan = true
	ctx.Output.Print("allowed")
}

func TestNewForHonoursTypedAllower(t *testing.T) {
	typedAllowedRan = false
	command := NewFor[*typedTestSource]("allowed", "checks typed source permissions", nil, typedAllowedCommand{})
	disallowed := &typedTestSource{}

	if params := command.Params(disallowed); len(params) != 0 {
		t.Fatalf("Params returned %d overloads for disallowed source, want 0", len(params))
	}
	if runnables := command.Runnables(disallowed); len(runnables) != 0 {
		t.Fatalf("Runnables returned %d overloads for disallowed source, want 0", len(runnables))
	}
	command.Execute("", disallowed, nil)
	if typedAllowedRan {
		t.Fatal("typed command ran for disallowed source")
	}

	allowed := &typedTestSource{allowed: true}
	command.Execute("", allowed, nil)
	if !typedAllowedRan {
		t.Fatal("typed command did not run for allowed source")
	}
	if allowed.output == nil || allowed.output.MessageCount() != 1 || allowed.output.Messages()[0].String() != "allowed" {
		t.Fatalf("allowed output = %#v, want one allowed message", allowed.output)
	}
}

type typedPointerCommand struct {
	Message string
}

var typedPointerMessage string

func (c *typedPointerCommand) Run(ctx *Context[*typedTestSource]) {
	typedPointerMessage = c.Message
	ctx.Output.Print(c.Message)
}

func TestNewForSupportsPointerReceiver(t *testing.T) {
	typedPointerMessage = ""
	command := NewFor[*typedTestSource]("pointer", "uses a pointer receiver", nil, &typedPointerCommand{})
	source := &typedTestSource{}

	command.Execute("hello", source, nil)

	if typedPointerMessage != "hello" {
		t.Fatalf("typed pointer command message = %q, want hello", typedPointerMessage)
	}
	if source.output == nil || source.output.MessageCount() != 1 || source.output.Messages()[0].String() != "hello" {
		t.Fatalf("source output = %#v, want one hello message", source.output)
	}
}

type legacyPointerCommand struct {
	Message string
}

var legacyPointerMessage string

func (c *legacyPointerCommand) Run(_ Source, o *Output, _ *world.Tx) {
	legacyPointerMessage = c.Message
	o.Print(c.Message)
}

func TestNewStillSupportsLegacyPointerReceiver(t *testing.T) {
	legacyPointerMessage = ""
	command := New("legacy", "uses the legacy Runnable signature", nil, &legacyPointerCommand{})
	source := &typedTestSource{}

	command.Execute("hello", source, nil)

	if legacyPointerMessage != "hello" {
		t.Fatalf("legacy pointer command message = %q, want hello", legacyPointerMessage)
	}
	if source.output == nil || source.output.MessageCount() != 1 || source.output.Messages()[0].String() != "hello" {
		t.Fatalf("source output = %#v, want one hello message", source.output)
	}
}
