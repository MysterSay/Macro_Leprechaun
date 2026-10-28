package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestMissingMacroDoesNotBlockStorage(t *testing.T) {
	root := t.TempDir()
	c := defaults()
	if e := writeStorage(root, c); e != nil {
		t.Fatal(e)
	}
	files, e := filepath.Glob(filepath.Join(root, "macros", "notepad.*.json"))
	if e != nil || len(files) != 1 {
		t.Fatal(files, e)
	}
	if e = os.Remove(files[0]); e != nil {
		t.Fatal(e)
	}
	loaded, e := loadStorage(root)
	if e != nil {
		t.Fatal("missing file blocked startup", e)
	}
	if len(loaded.Macros) != len(c.Macros) {
		t.Fatal("healthy macros lost")
	}
	var index int
	for i, m := range loaded.Macros {
		if m.ID == "notepad" {
			index = i
			if m.MissingFile != filepath.Base(files[0]) {
				t.Fatal(m)
			}
		}
	}
	if e = writeStorage(root, loaded); e != nil {
		t.Fatal(e)
	}
	if _, e = os.Stat(files[0]); !os.IsNotExist(e) {
		t.Fatal("placeholder recreated missing file")
	}
	loaded, e = loadStorage(root)
	if e != nil || loaded.Macros[index].MissingFile == "" {
		t.Fatal("missing notification lost on save", e)
	}
	loaded.Macros[index] = Macro{ID: "notepad", Name: "Recreated", Steps: []Step{{Type: "keys", Value: "A"}}}
	if e = writeStorage(root, loaded); e != nil {
		t.Fatal(e)
	}
	loaded, e = loadStorage(root)
	if e != nil || loaded.Macros[index].MissingFile != "" || loaded.Macros[index].Name != "Recreated" {
		t.Fatal("recreation failed", e)
	}
}
func TestDeletingMissingMacroKeepsUnassignedButtons(t *testing.T) {
	c := defaults()
	c.Macros = c.Macros[1:]
	for _, panels := range [][]Panel{c.Smart, c.Widgets, c.Floating} {
		for i := range panels {
			for j := range panels[i].Controls {
				b := &panels[i].Controls[j]
				if b.Macro == "notepad" {
					b.Macro = ""
				}
			}
		}
	}
	if e := validate(c); e != nil {
		t.Fatal(e)
	}
	root := t.TempDir()
	if e := writeStorage(root, c); e != nil {
		t.Fatal(e)
	}
	if _, e := loadStorage(root); e != nil {
		t.Fatal(e)
	}
}
