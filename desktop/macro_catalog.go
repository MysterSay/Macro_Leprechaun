package main

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type MacroSummary struct {
	Plugins     []string `json:"plugins,omitempty"`
	Inputs      []Puzzle `json:"inputs"`
	References  []string `json:"references"`
	Calls       []string `json:"calls"`
	PuzzleCount int      `json:"puzzleCount"`
	StepCount   int      `json:"stepCount"`
}

func summarizeMacro(m Macro) *MacroSummary {
	s := &MacroSummary{Inputs: []Puzzle{}, References: []string{}, Calls: []string{}, PuzzleCount: len(m.Puzzles), StepCount: len(m.Steps)}
	refs, calls := map[string]bool{}, map[string]bool{}
	plugins := map[string]bool{}
	var scan func(any)
	scan = func(v any) {
		switch x := v.(type) {
		case string:
			for _, token := range labelReference.FindAllString(x, -1) {
				refs[token[2:len(token)-1]] = true
			}
			if idPattern.MatchString(x) {
				refs[x] = true
			}
			if strings.HasPrefix(strings.TrimSpace(x), "{") || strings.HasPrefix(strings.TrimSpace(x), "[") {
				var nested any
				if json.Unmarshal([]byte(x), &nested) == nil {
					scan(nested)
				}
			}
		case map[string]any:
			for _, value := range x {
				scan(value)
			}
		case []any:
			for _, value := range x {
				scan(value)
			}
		}
	}
	for _, p := range m.Puzzles {
		if p.Type == "input" {
			input := p
			input.Value = ""
			input.Next = ""
			input.TrueNext = ""
			input.FalseNext = ""
			input.Body = ""
			s.Inputs = append(s.Inputs, input)
		}
		if strings.HasPrefix(p.Type, "plugin:") {
			parts := strings.Split(strings.TrimPrefix(p.Type, "plugin:"), "/")
			plugins[parts[0]] = true
		}
		scan(p.Value)
		if p.Variable != "" {
			refs[p.Variable] = true
		}
		if p.Type == "actions.macro" {
			if q, err := parseAppAction(p); err == nil && q.Macro != "" {
				calls[q.Macro] = true
			}
		}
	}
	for _, step := range m.Steps {
		scan(step.Value)
		for _, arg := range step.Args {
			scan(arg)
		}
	}
	for key := range plugins {
		s.Plugins = append(s.Plugins, key)
	}
	for key := range refs {
		s.References = append(s.References, key)
	}
	for key := range calls {
		s.Calls = append(s.Calls, key)
	}
	sort.Strings(s.References)
	sort.Strings(s.Calls)
	return s
}
func macroBodyBytes(m Macro) ([]byte, error) {
	m.File = ""
	m.Lazy = false
	m.Summary = nil
	return json.MarshalIndent(m, "", "  ")
}
func annotateMacro(m Macro) Macro {
	if m.MissingFile != "" {
		return m
	}
	if m.Summary == nil {
		m.Summary = summarizeMacro(m)
	}
	if m.File == "" {
		raw, _ := macroBodyBytes(m)
		hash := sha256.Sum256(raw)
		m.File = fmt.Sprintf("%s.%x.json", m.ID, hash[:6])
	}
	return m
}
func catalogMacro(m Macro) Macro {
	if m.MissingFile != "" {
		return m
	}
	m = annotateMacro(m)
	m.Lazy = true
	m.Puzzles = nil
	m.Steps = []Step{}
	return m
}
func readMacroBody(root string, m Macro) (Macro, error) {
	if !m.Lazy {
		return m, nil
	}
	if m.File == "" || filepath.Base(m.File) != m.File {
		return Macro{}, fmt.Errorf("invalid macro file")
	}
	f, err := os.Open(filepath.Join(root, "macros", m.File))
	if err != nil {
		return Macro{}, err
	}
	defer f.Close()
	raw, err := io.ReadAll(io.LimitReader(f, 4<<20+1))
	if err != nil {
		return Macro{}, err
	}
	if len(raw) > 4<<20 {
		return Macro{}, fmt.Errorf("macro exceeds 4 MiB")
	}
	var body Macro
	if err = json.Unmarshal(raw, &body); err != nil {
		return Macro{}, err
	}
	if body.ID != m.ID {
		return Macro{}, fmt.Errorf("macro ID mismatch")
	}
	if body.GraphVersion == 1 {
		if err = validateGraph(body); err != nil {
			return Macro{}, err
		}
	}
	body.File = m.File
	body.Summary = summarizeMacro(body)
	return body, nil
}
func (a *App) macroForRun(id string) (Macro, error) {
	a.mu.RLock()
	var found Macro
	for _, m := range a.cfg.Macros {
		if m.ID == id {
			found = m
			break
		}
	}
	a.mu.RUnlock()
	if found.ID == "" {
		return Macro{}, fmt.Errorf("macro not found")
	}
	return readMacroBody(a.root, found)
}

// Only an unchanged server catalog entry may preserve a body on disk. Client
// supplied paths and summaries are never trusted as new macro definitions.
func (a *App) canonicalizeCatalog(c *Config) error {
	known := map[string]Macro{}
	for _, m := range a.cfg.Macros {
		known[m.ID] = catalogMacro(m)
	}
	for i, m := range c.Macros {
		if !m.Lazy {
			continue
		}
		old, ok := known[m.ID]
		if !ok || old.File != m.File || old.MissingFile != "" {
			return fmt.Errorf("macro catalog changed; reload settings")
		}
		c.Macros[i] = old
	}
	return nil
}
func applyMacroPolicy(root string, c *Config) error {
	for i, m := range c.Macros {
		if m.MissingFile != "" {
			continue
		}
		if c.Settings.CacheMacros {
			full, err := readMacroBody(root, m)
			if err != nil {
				return err
			}
			c.Macros[i] = annotateMacro(full)
		} else {
			c.Macros[i] = catalogMacro(m)
		}
	}
	return nil
}
func (a *App) runtimeConfig() Config {
	a.mu.RLock()
	base := a.cfg
	base.Macros = nil
	raw, _ := json.Marshal(base)
	var c Config
	json.Unmarshal(raw, &c)
	c.Macros = make([]Macro, 0, len(a.cfg.Macros))
	for _, m := range a.cfg.Macros {
		c.Macros = append(c.Macros, catalogMacro(m))
	}
	a.addVariableValuesLocked(&c)
	a.mu.RUnlock()
	return c
}
