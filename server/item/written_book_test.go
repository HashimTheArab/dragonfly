package item

import (
	"slices"
	"testing"
)

// TestWrittenBook_DecodeNBT preserves page positions when malformed text is ignored.
func TestWrittenBook_DecodeNBT(t *testing.T) {
	for _, test := range []struct {
		name  string
		pages any
		want  []string
	}{
		{name: "valid pages", pages: []any{map[string]any{"text": "first"}, map[string]any{"text": "last"}}, want: []string{"first", "last"}},
		{name: "malformed text", pages: []any{map[string]any{"text": "first"}, map[string]any{"text": int32(1)}, map[string]any{}, map[string]any{"text": "last"}}, want: []string{"first", "", "", "last"}},
		{name: "non-compound pages", pages: []any{nil, "page", int32(1)}, want: []string{"", "", ""}},
		{name: "wrong list type", pages: []int32{1}},
	} {
		for _, mode := range []string{"item tags", "disk stack"} {
			t.Run(test.name+"/"+mode, func(t *testing.T) {
				tag := map[string]any{"pages": test.pages, "title": "Title", "author": "Author", "generation": byte(2)}
				var book WrittenBook
				if mode == "item tags" {
					book = (WrittenBook{}).DecodeNBT(tag).(WrittenBook)
				} else {
					stack := ReadNBT(map[string]any{"Name": "minecraft:written_book", "Count": byte(1), "tag": tag}, nil)
					var ok bool
					book, ok = stack.Item().(WrittenBook)
					if !ok || stack.Count() != 1 {
						t.Fatalf("decoded book stack = %v", stack)
					}
				}
				if !slices.Equal(book.Pages, test.want) || book.Title != "Title" || book.Author != "Author" || book.Generation != CopyOfCopyGeneration() {
					t.Fatalf("decoded book = %+v; want pages %v with original metadata", book, test.want)
				}
			})
		}
	}
}
