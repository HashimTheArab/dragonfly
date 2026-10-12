package effect

import (
	"go/ast"
	"go/constant"
	"go/token"
	"go/types"
	"regexp"
	"testing"

	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
	"golang.org/x/tools/go/packages"
)

// New protocol effects must have metadata before the catalog can be published.
func TestMetadata_CoversProtocolEffects(t *testing.T) {
	pkgs, err := packages.Load(&packages.Config{
		Mode: packages.NeedName | packages.NeedFiles | packages.NeedSyntax | packages.NeedTypes | packages.NeedTypesInfo,
	}, "github.com/sandertv/gophertunnel/minecraft/protocol/packet")
	if err != nil {
		t.Fatal(err)
	}
	if packages.PrintErrors(pkgs) != 0 || len(pkgs) != 1 {
		t.Fatal("could not load the protocol effect declarations")
	}
	count := 0
	for _, file := range pkgs[0].Syntax {
		for _, decl := range file.Decls {
			group, ok := decl.(*ast.GenDecl)
			if !ok || group.Tok != token.CONST || len(group.Specs) == 0 {
				continue
			}
			first := group.Specs[0].(*ast.ValueSpec)
			if first.Names[0].Name != "EffectSpeed" {
				continue
			}
			for _, spec := range group.Specs {
				for _, name := range spec.(*ast.ValueSpec).Names {
					value := pkgs[0].TypesInfo.Defs[name].(*types.Const)
					id, valid := constant.Int64Val(value.Val())
					if !valid || id < 1 || id > 1<<31-1 {
						t.Fatalf("invalid effect ID %s: %s", name.Name, value.Val())
					}
					count++
					if metadata, found := MetadataByID(int(id)); !found || metadata.Name == "" || metadata.Icon == "" {
						t.Errorf("effect %s (%d) lacks metadata", name.Name, id)
					}
				}
			}
		}
	}
	if count == 0 || len(effects) != count {
		t.Fatalf("registry entries = %d, protocol effects = %d", len(effects), count)
	}
}

// Metadata must describe client effects without claiming unimplemented server behaviour.
func TestMetadata_LookupsPreserveImplementationAvailability(t *testing.T) {
	for _, tc := range []struct {
		id   int
		name string
		icon string
		impl Type
	}{
		{packet.EffectSpeed, "speed", "speed_effect", Speed},
		{packet.EffectFatalPoison, "fatal_poison", "poison_effect", FatalPoison},
		{packet.EffectBadOmen, "bad_omen", "bad_omen_effect", nil},
		{packet.EffectTrialOmen, "trial_omen", "trial_omen_effect", nil},
		{packet.EffectRaidOmen, "raid_omen", "raid_omen_effect", nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			metadata, ok := MetadataByID(tc.id)
			if !ok || metadata.Name != tc.name || metadata.Icon != tc.icon {
				t.Fatalf("metadata = %+v, found = %v", metadata, ok)
			}
			impl, ok := ByID(tc.id)
			if impl != tc.impl || ok != (tc.impl != nil) {
				t.Fatalf("implementation = %v, found = %v", impl, ok)
			}
			if ok {
				if id, found := ID(impl); !found || id != tc.id {
					t.Fatalf("reverse lookup = %d, found = %v", id, found)
				}
			}
		})
	}
	for _, id := range []int{-1, 0, 10000} {
		if metadata, ok := MetadataByID(id); ok || metadata != (Metadata{}) {
			t.Fatalf("unknown ID %d has metadata: %+v", id, metadata)
		}
		if impl, ok := ByID(id); ok || impl != nil {
			t.Fatalf("unknown ID %d has an implementation: %v", id, impl)
		}
	}
	if _, ok := ID(nil); ok {
		t.Fatal("metadata-only registrations assigned an ID to nil")
	}
}

// Enumeration must expose valid, unique names and a copy of the registry.
func TestMetadata_EnumerationIsIndependent(t *testing.T) {
	all := AllMetadata()
	if len(all) == 0 {
		t.Fatal("empty effect catalog")
	}
	identifier := regexp.MustCompile(`^[a-z]+(?:_[a-z]+)*$`)
	names := map[string]int{}
	for id, metadata := range all {
		if !identifier.MatchString(metadata.Name) || !identifier.MatchString(metadata.Icon) {
			t.Errorf("effect %d has invalid metadata: %+v", id, metadata)
		}
		if previous, ok := names[metadata.Name]; ok {
			t.Errorf("effects %d and %d share name %q", previous, id, metadata.Name)
		}
		names[metadata.Name] = id
		if impl, ok := ByID(id); ok {
			if reverse, found := ID(impl); !found || reverse != id {
				t.Errorf("effect %d reverse lookup = %d, found = %v", id, reverse, found)
			}
		}
		delete(all, id)
	}
	if got := len(AllMetadata()); got != len(names) {
		t.Fatalf("editing the returned map changed the registry: %d effects", got)
	}
}

// Replacing behaviour with metadata alone must also retire the reverse lookup.
func TestMetadata_ReplacementClearsImplementation(t *testing.T) {
	id := packet.EffectSpeed
	previous := effects[id]
	t.Cleanup(func() { Register(id, previous.implementation, previous.metadata) })
	metadata := Metadata{"custom_speed", "speed_effect"}
	Register(id, nil, metadata)
	if impl, ok := ByID(id); ok || impl != nil {
		t.Fatal("replaced implementation is still available")
	}
	if _, ok := ID(Speed); ok {
		t.Fatal("replaced implementation retained its reverse lookup")
	}
	if got, ok := MetadataByID(id); !ok || got != metadata {
		t.Fatalf("replacement metadata = %+v, found = %v", got, ok)
	}
}

// Replacing one ID must not retire an implementation's more recent alias.
func TestMetadata_ReplacementPreservesOtherReverseLookup(t *testing.T) {
	id, alias := packet.EffectSpeed, 10000
	previous := effects[id]
	t.Cleanup(func() {
		delete(effects, alias)
		Register(id, previous.implementation, previous.metadata)
	})
	Register(alias, Speed, Metadata{"custom_speed", "speed_effect"})
	Register(id, nil, previous.metadata)
	if got, ok := ID(Speed); !ok || got != alias {
		t.Fatalf("other reverse lookup = %d, found = %v", got, ok)
	}
}
