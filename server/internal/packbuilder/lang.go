package packbuilder

import (
	"os"
	"path/filepath"
	"strings"
)

// buildLanguageFile appends generated entries while preserving entity translations.
func buildLanguageFile(dir string, lang []string) {
	if err := os.MkdirAll(filepath.Join(dir, "texts"), os.ModePerm); err != nil {
		panic(err)
	}
	file := filepath.Join(dir, "texts/en_US.lang")
	data, err := os.ReadFile(file)
	if err != nil && !os.IsNotExist(err) {
		panic(err)
	}
	if len(lang) > 0 && len(data) > 0 && data[len(data)-1] != '\n' {
		data = append(data, '\n')
	}
	data = append(data, strings.Join(lang, "\n")...)
	if err := os.WriteFile(file, data, 0666); err != nil {
		panic(err)
	}
}
