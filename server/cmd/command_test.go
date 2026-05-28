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

func (s *testSource) SendCommandOutput(o *Output) { s.output = o }

type nilContextCommand struct {
	Message string
	ran     *bool `cmd:"-"`
}

func (c *nilContextCommand) Run(_ Source, _ *Output, ctx *world.Context) {
	if ctx != nil {
		panic("expected nil world context")
	}
	*c.ran = true
}

func TestExecuteAllowsNilContextForNonWorldCommand(t *testing.T) {
	ran := false
	source := &testSource{}
	command := New("nilctx", "", nil, &nilContextCommand{ran: &ran})

	command.Execute("hello", source, nil)

	if !ran {
		t.Fatal("command did not run")
	}
	if source.output == nil {
		t.Fatal("source did not receive command output")
	}
	if source.output.ErrorCount() != 0 {
		t.Fatalf("unexpected command errors: %v", source.output.Errors())
	}
}

type nilContextTargetCommand struct {
	Targets []Target
	ran     *bool `cmd:"-"`
}

func (c *nilContextTargetCommand) Run(Source, *Output, *world.Context) {
	*c.ran = true
}

func TestExecuteAllowsNilContextTargetSelectorError(t *testing.T) {
	ran := false
	source := &testSource{}
	command := New("nilctx-target", "", nil, &nilContextTargetCommand{ran: &ran})

	command.Execute("@e", source, nil)

	if ran {
		t.Fatal("command ran despite missing world context for target selector")
	}
	if source.output == nil {
		t.Fatal("source did not receive command output")
	}
	if source.output.ErrorCount() == 0 {
		t.Fatal("expected command error for target selector without world context")
	}
	for _, err := range source.output.Errors() {
		if err == nil {
			t.Fatalf("unexpected nil command error: %v", source.output.Errors())
		}
	}
}
