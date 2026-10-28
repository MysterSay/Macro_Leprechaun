package main

import (
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	_ "modernc.org/sqlite"
	"os"
	"path/filepath"
)

func openStorage(root string) (*sql.DB, error) {
	for _, dir := range []string{"macros", "setting", "icon", "image", "plagin"} {
		if e := os.MkdirAll(filepath.Join(root, dir), 0700); e != nil {
			return nil, e
		}
	}
	schema := `CREATE TABLE settings(key TEXT PRIMARY KEY, value TEXT NOT NULL);
CREATE TABLE panels(mode INTEGER NOT NULL, position INTEGER NOT NULL, id TEXT NOT NULL, value TEXT NOT NULL, PRIMARY KEY(mode,id));
CREATE TABLE buttons(mode INTEGER NOT NULL,panel_id TEXT NOT NULL,position INTEGER NOT NULL,id TEXT NOT NULL,value TEXT NOT NULL,PRIMARY KEY(mode,panel_id,id));
CREATE TABLE macro_files(position INTEGER PRIMARY KEY,id TEXT UNIQUE NOT NULL,filename TEXT NOT NULL);
CREATE TABLE macro_metadata(id TEXT PRIMARY KEY,filename TEXT NOT NULL,value TEXT NOT NULL);
`
	if e := os.WriteFile(filepath.Join(root, "setting", "schema.sql"), []byte(schema), 0600); e != nil {
		return nil, e
	}
	db, e := sql.Open("sqlite", filepath.Join(root, "setting", "settings.sqlite"))
	if e != nil {
		return nil, e
	}
	db.SetMaxOpenConns(1)
	_, e = db.Exec(`PRAGMA busy_timeout=5000; CREATE TABLE IF NOT EXISTS settings(key TEXT PRIMARY KEY, value TEXT NOT NULL); CREATE TABLE IF NOT EXISTS panels(mode INTEGER NOT NULL, position INTEGER NOT NULL, id TEXT NOT NULL, value TEXT NOT NULL, PRIMARY KEY(mode,id)); CREATE TABLE IF NOT EXISTS buttons(mode INTEGER NOT NULL,panel_id TEXT NOT NULL,position INTEGER NOT NULL,id TEXT NOT NULL,value TEXT NOT NULL, PRIMARY KEY(mode,panel_id,id)); CREATE TABLE IF NOT EXISTS macro_files(position INTEGER PRIMARY KEY,id TEXT UNIQUE NOT NULL,filename TEXT NOT NULL); CREATE TABLE IF NOT EXISTS macro_metadata(id TEXT PRIMARY KEY,filename TEXT NOT NULL,value TEXT NOT NULL);`)
	if e != nil {
		db.Close()
		return nil, e
	}
	return db, nil
}
func writeStorage(root string, c Config) error {
	db, e := openStorage(root)
	if e != nil {
		return e
	}
	defer db.Close()
	tx, e := db.Begin()
	if e != nil {
		return e
	}
	defer tx.Rollback()
	// Immutable macro files are staged before the SQLite index commit. A failed
	// commit leaves the previously committed profile fully readable.
	used := map[string]bool{}
	if _, e = tx.Exec("DELETE FROM macro_files"); e != nil {
		return e
	}
	for i, m := range c.Macros {
		if m.MissingFile != "" {
			used[m.MissingFile] = true
			if _, e = tx.Exec("INSERT INTO macro_files VALUES(?,?,?)", i, m.ID, m.MissingFile); e != nil {
				return e
			}
			continue
		}
		name := m.File
		var b []byte
		if !m.Lazy {
			m.File = ""
			m.Summary = nil
			m = annotateMacro(m)
			c.Macros[i] = m
			name = m.File
			b, e = macroBodyBytes(m)
			if e != nil {
				return e
			}
		}
		if m.Lazy && name == "" {
			return fmt.Errorf("missing macro filename")
		}

		used[name] = true
		p := filepath.Join(root, "macros", name)
		if _, e = os.Stat(p); os.IsNotExist(e) && !m.Lazy {
			if e = os.WriteFile(p+".tmp", b, 0600); e != nil {
				return e
			}
			if e = os.Rename(p+".tmp", p); e != nil {
				return e
			}
		} else if e != nil {
			return e
		}
		meta, _ := json.Marshal(catalogMacro(m))
		if _, err := tx.Exec("INSERT OR REPLACE INTO macro_metadata VALUES(?,?,?)", m.ID, name, string(meta)); err != nil {
			return err
		}
		if _, e = tx.Exec("INSERT INTO macro_files VALUES(?,?,?)", i, m.ID, name); e != nil {
			return e
		}
	}
	base := c
	base.Macros = nil
	base.Smart = nil
	base.Widgets = nil
	base.Floating = nil
	b, e := json.Marshal(base)
	if e != nil {
		return e
	}
	if _, e = tx.Exec("INSERT OR REPLACE INTO settings VALUES('config',?)", string(b)); e != nil {
		return e
	}
	if _, e = tx.Exec("DELETE FROM macro_metadata WHERE id NOT IN (SELECT id FROM macro_files); DELETE FROM panels; DELETE FROM buttons;"); e != nil {
		return e
	}
	for mode, panels := range [][]Panel{c.Smart, c.Widgets, c.Floating} {
		for i, p := range panels {
			controls := p.Controls
			p.Controls = nil
			b, _ := json.Marshal(p)
			if _, e = tx.Exec("INSERT INTO panels VALUES(?,?,?,?)", mode, i, p.ID, string(b)); e != nil {
				return e
			}
			for j, b := range controls {
				data, _ := json.Marshal(b)
				if _, e = tx.Exec("INSERT INTO buttons VALUES(?,?,?,?,?)", mode, p.ID, j, b.ID, string(data)); e != nil {
					return e
				}
			}
		}
	}
	if e = tx.Commit(); e != nil {
		return e
	}
	// Only files produced by the content-addressed writer are garbage collected.
	files, _ := filepath.Glob(filepath.Join(root, "macros", "*.json"))
	for _, p := range files {
		if !used[filepath.Base(p)] {
			var m Macro
			if b, e := os.ReadFile(p); e == nil && json.Unmarshal(b, &m) == nil && idPattern.MatchString(m.ID) {
				hash := sha256.Sum256(b)
				if filepath.Base(p) == fmt.Sprintf("%s.%x.json", m.ID, hash[:6]) {
					_ = os.Remove(p)
				}
			}
		}
	}
	return nil
}
func loadStorage(root string) (Config, error) {
	var c Config
	db, e := openStorage(root)
	if e != nil {
		return c, e
	}
	defer db.Close()
	var data string
	if e = db.QueryRow("SELECT value FROM settings WHERE key='config'").Scan(&data); e != nil {
		return c, e
	}
	if e = json.Unmarshal([]byte(data), &c); e != nil {
		return c, e
	}
	c.Macros = []Macro{}
	c.Smart = []Panel{}
	c.Widgets = []Panel{}
	c.Floating = []Panel{}
	rows, e := db.Query("SELECT id,filename FROM macro_files ORDER BY position")
	if e != nil {
		return c, e
	}
	type macroRecord struct{ id, name string }
	var names []macroRecord
	for rows.Next() {
		var n macroRecord
		if e = rows.Scan(&n.id, &n.name); e != nil {
			rows.Close()
			return c, e
		}
		names = append(names, n)
	}
	e = rows.Err()
	rows.Close()
	if e != nil {
		return c, e
	}
	for _, record := range names {
		n := record.name
		if filepath.Base(n) != n {
			return c, fmt.Errorf("invalid macro filename")
		}
		if _, err := os.Stat(filepath.Join(root, "macros", n)); os.IsNotExist(err) {
			c.Macros = append(c.Macros, Macro{ID: record.id, Name: record.id, Icon: "⚠", Steps: []Step{}, MissingFile: n})
			log.Printf("Missing macro file: %s (id=%s); skipped", n, record.id)
			continue
		} else if err != nil {
			return c, err
		}
		var cached string
		var m Macro
		err := db.QueryRow("SELECT value FROM macro_metadata WHERE id=? AND filename=?", record.id, n).Scan(&cached)
		if err == nil && json.Unmarshal([]byte(cached), &m) == nil && m.ID == record.id && m.Summary != nil {
			m.Lazy = true
			m.File = n
			m.Puzzles = nil
			m.Steps = []Step{}
		} else {
			m = Macro{ID: record.id, File: n, Lazy: true}
			m, e = readMacroBody(root, m)
			if e != nil {
				return c, e
			}
			m = annotateMacro(m)
			meta, _ := json.Marshal(catalogMacro(m))
			if _, e = db.Exec("INSERT OR REPLACE INTO macro_metadata VALUES(?,?,?)", m.ID, n, string(meta)); e != nil {
				return c, e
			}
		}
		if c.Settings.CacheMacros {
			m, e = readMacroBody(root, m)
			if e != nil {
				return c, e
			}
		} else {
			m = catalogMacro(m)
		}
		c.Macros = append(c.Macros, m)
	}

	rows, e = db.Query("SELECT mode,value FROM panels ORDER BY mode,position")
	if e != nil {
		return c, e
	}
	type record struct {
		mode int
		p    Panel
	}
	var records []record
	for rows.Next() {
		var r record
		var s string
		if e = rows.Scan(&r.mode, &s); e != nil {
			rows.Close()
			return c, e
		}
		if e = json.Unmarshal([]byte(s), &r.p); e != nil {
			rows.Close()
			return c, e
		}
		records = append(records, r)
	}
	e = rows.Err()
	rows.Close()
	if e != nil {
		return c, e
	}
	for _, r := range records {
		r.p.Controls = []Control{}
		rows, e = db.Query("SELECT value FROM buttons WHERE mode=? AND panel_id=? ORDER BY position", r.mode, r.p.ID)
		if e != nil {
			return c, e
		}
		for rows.Next() {
			var s string
			var b Control
			if e = rows.Scan(&s); e != nil {
				rows.Close()
				return c, e
			}
			if e = json.Unmarshal([]byte(s), &b); e != nil {
				rows.Close()
				return c, e
			}
			r.p.Controls = append(r.p.Controls, b)
		}
		e = rows.Err()
		rows.Close()
		if e != nil {
			return c, e
		}
		switch r.mode {
		case 0:
			c.Smart = append(c.Smart, r.p)
		case 1:
			c.Widgets = append(c.Widgets, r.p)
		case 2:
			c.Floating = append(c.Floating, r.p)
		}
	}
	for i := range c.Macros {
		m := &c.Macros[i]
		if m.MissingFile == "" {
			continue
		}
		for _, panels := range [][]Panel{c.Smart, c.Widgets, c.Floating} {
			for _, p := range panels {
				for _, b := range p.Controls {
					if b.Macro == m.ID || b.OffMacro == m.ID {
						m.Name = b.Label
					}
				}
			}
		}
	}
	normalizePanelRoots(&c)
	return c, validate(c)
}

func mediaName(n string) bool {
	switch filepath.Ext(n) {
	case ".png", ".jpg", ".jpeg", ".gif", ".webp", ".svg":
		return true
	}
	return false
}
func mediaFiles(root string) map[string][]string {
	out := map[string][]string{"icon": {}, "image": {}}
	for kind := range out {
		entries, _ := os.ReadDir(filepath.Join(root, kind))
		for _, e := range entries {
			if !e.IsDir() && mediaName(e.Name()) {
				out[kind] = append(out[kind], e.Name())
			}
		}
	}
	return out
}
