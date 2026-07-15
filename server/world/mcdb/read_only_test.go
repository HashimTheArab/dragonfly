package mcdb

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/df-mc/goleveldb/leveldb/opt"
)

func TestReadOnlyCloseDoesNotRewriteWorldMetadata(t *testing.T) {
	dir := t.TempDir()
	db, err := (Config{}).Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	db.ldat.LevelName = "before"
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}

	path := filepath.Join(dir, "level.dat")
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	readOnly, err := (Config{LDBOptions: &opt.Options{ReadOnly: true}}).Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	readOnly.ldat.LevelName = "after"
	if err := readOnly.Close(); err != nil {
		t.Fatal(err)
	}
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(after, before) {
		t.Fatal("closing a read-only DB rewrote level.dat")
	}
}

func TestReadOnlyOpenDoesNotCreateWorldDirectory(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "missing")
	_, err := (Config{LDBOptions: &opt.Options{ReadOnly: true}}).Open(dir)
	if err == nil {
		t.Fatal("opening a missing read-only DB succeeded")
	}
	if _, statErr := os.Stat(dir); !os.IsNotExist(statErr) {
		t.Fatalf("opening a missing read-only DB created %s", dir)
	}
}
