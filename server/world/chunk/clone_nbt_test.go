package chunk

import (
	"reflect"
	"testing"
)

func signBlockEntityData() map[string]any {
	side := func(text string) map[string]any {
		return map[string]any{
			"Text": text, "TextOwner": "", "SignTextColor": int32(-16777216),
			"IgnoreLighting": uint8(0), "HideGlowOutline": uint8(0), "PersistFormatting": uint8(1),
		}
	}
	return map[string]any{
		"id": "Sign", "x": int32(1), "y": int32(64), "z": int32(-3), "IsWaxed": uint8(0),
		"FrontText": side("Base below\nkeep out!"), "BackText": side(""),
	}
}

func TestCloneBlockEntityData_DetachesContainersAndKeepsTypes(t *testing.T) {
	data := signBlockEntityData()
	data["Items"] = []any{map[string]any{"Name": "minecraft:diamond", "Count": uint8(64)}}
	data["Ints"] = []int32{1, 2}
	data["Names"] = []string{"a"}
	data["Pages"] = []map[string]any{{"text": "one"}}
	data["Typed"] = map[string]string{"k": "v"}
	data["Bytes"] = [2]byte{1, 2}
	data["NilList"] = []any(nil)
	data["NilCompound"] = map[string]any(nil)
	data["NilInts"] = []int32(nil)
	data["Nil"] = nil

	clone := CloneBlockEntityData(data)
	if !reflect.DeepEqual(clone, data) {
		t.Fatalf("clone = %#v, want %#v", clone, data)
	}
	for k, v := range data {
		if got := reflect.TypeOf(clone[k]); got != reflect.TypeOf(v) {
			t.Errorf("%s: clone type %v, want %v", k, got, reflect.TypeOf(v))
		}
	}

	clone["FrontText"].(map[string]any)["Text"] = "changed"
	clone["Items"].([]any)[0].(map[string]any)["Count"] = uint8(1)
	clone["Ints"].([]int32)[0] = 9
	clone["Names"].([]string)[0] = "b"
	clone["Pages"].([]map[string]any)[0]["text"] = "two"
	clone["Typed"].(map[string]string)["k"] = "w"
	if data["FrontText"].(map[string]any)["Text"] != "Base below\nkeep out!" ||
		data["Items"].([]any)[0].(map[string]any)["Count"] != uint8(64) ||
		data["Ints"].([]int32)[0] != 1 || data["Names"].([]string)[0] != "a" ||
		data["Pages"].([]map[string]any)[0]["text"] != "one" || data["Typed"].(map[string]string)["k"] != "v" {
		t.Fatalf("editing the clone changed the original: %#v", data)
	}
	if CloneBlockEntityData(nil) != nil {
		t.Fatal("a nil compound must clone to nil")
	}
}

// TestCloneBlockEntityData_AllocatesOnlyContainers checks that copying an
// untyped compound allocates its maps and lists, not a new box per scalar.
func TestCloneBlockEntityData_AllocatesOnlyContainers(t *testing.T) {
	data := signBlockEntityData()
	allocs := testing.AllocsPerRun(100, func() { _ = CloneBlockEntityData(data) })
	// Three maps, each of which may allocate its header and one group.
	if allocs > 6 {
		t.Fatalf("cloning a sign allocates %v times, want at most 6", allocs)
	}
}

func BenchmarkCloneBlockEntityData(b *testing.B) {
	data := signBlockEntityData()
	b.ReportAllocs()
	for b.Loop() {
		_ = CloneBlockEntityData(data)
	}
}
