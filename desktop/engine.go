package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

var ErrMacroBusy = errors.New("macro already running or queue full")

type PuzzleCategory struct {
	ID     string             `json:"id"`
	Name   string             `json:"name"`
	Blocks []PuzzleDefinition `json:"blocks"`
}
type PuzzleDefinition struct {
	Editor  string   `json:"editor,omitempty"`
	Options []string `json:"options,omitempty"`
	UI      string   `json:"ui,omitempty"`
	ID      string   `json:"id"`
	Name    string   `json:"name"`
}
type Module struct {
	UsesVariables    []string `json:"usesVariables,omitempty"`
	Runtime          string   `json:"runtime,omitempty"`
	Entry            string   `json:"entry,omitempty"`
	directory        string
	RuntimeError     string           `json:"runtimeError,omitempty"`
	Description      string           `json:"description,omitempty"`
	SettingsUI       string           `json:"settingsUI,omitempty"`
	Variables        []PluginVariable `json:"variables,omitempty"`
	PuzzleCategories []PuzzleCategory `json:"puzzleCategories,omitempty"`
	ID               string           `json:"id"`
	Name             string           `json:"name"`
	Version          string           `json:"version"`
	API              int              `json:"api"`
	Executable       string           `json:"executable"`
	Args             []string         `json:"args"`
	Actions          []string         `json:"actions"`
	Error            string           `json:"error,omitempty"`
}

func (a *App) scanModules() {
	files, _ := filepath.Glob(filepath.Join(a.root, "plagin", "*", "manifest.json"))
	var list []Module
	for _, p := range files {
		var m Module
		b, e := os.ReadFile(p)
		if e != nil {
			continue
		}
		if json.Unmarshal(b, &m) != nil {
			continue
		}
		if !idPattern.MatchString(m.ID) || (m.API != 1 && m.API != 2) {
			m.Error = "Invalid ID or api"
		}
		if len(m.PuzzleCategories) > 1 {
			m.Error = "One addon may declare only one puzzle category"
		}
		base := filepath.Dir(p)
		m.directory = base
		target := m.Executable
		if m.Entry != "" || m.Runtime != "" {
			if m.API != 2 || !oneOf(m.Runtime, "powershell", "python", "javascript") || m.Entry == "" || m.Executable != "" {
				m.Error = "Script addon: api 2, runtime and entry required; omit executable"
			}
			target = m.Entry
		}
		full := filepath.Join(base, target)
		rel, err := filepath.Rel(base, full)
		if target == "" || err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(target) {
			m.Error = "Entry must be a file inside addon directory"
		} else {
			info, err := os.Stat(full)
			if err != nil {
				m.Error = err.Error()
			} else if !info.Mode().IsRegular() {
				m.Error = "Addon entry must be a regular file"
			} else {
				resolved, err := filepath.EvalSymlinks(full)
				resolvedBase, baseErr := filepath.EvalSymlinks(base)
				if err != nil || baseErr != nil {
					m.Error = "Cannot resolve addon entry"
				} else if path, err := filepath.Rel(resolvedBase, resolved); err != nil || path == ".." || strings.HasPrefix(path, ".."+string(filepath.Separator)) {
					m.Error = "Addon entry may not point outside its directory"
				}
			}
		}
		if m.Entry != "" {
			m.Entry = full
		} else {
			m.Executable = full
		}

		list = append(list, m)
	}
	a.mu.Lock()
	a.moduleList = list
	a.mu.Unlock()
}
func (a *App) run(id string, vars map[string]string) error {
	return a.runInput(id, vars, InputEvent{Kind: "button", Value: "click"})
}
func (a *App) runInput(id string, vars map[string]string, event InputEvent) error {
	return a.runScoped(id, vars, event, RunScope{})
}
func (a *App) runScoped(id string, vars map[string]string, event InputEvent, scope RunScope) error {
	if len(scope.Chain) >= 16 {
		return fmt.Errorf("максимальна глибина запуску макросів: 16")
	}
	for _, parent := range scope.Chain {
		if parent == id {
			return fmt.Errorf("циклічний запуск макросів")
		}
	}
	scope.Chain = append(append([]string{}, scope.Chain...), id)
	if id == "" {
		return nil
	}
	loaded, err := a.macroForRun(id)
	if err != nil {
		return err
	}
	macro := &loaded

	if macro != nil && len(macro.Puzzles) > 0 && macro.GraphVersion == 0 {
		return fmt.Errorf("Puzzle macro is a draft; execution is not implemented yet")
	}
	if macro == nil {
		return fmt.Errorf("macro not found")
	}
	if macro.MissingFile != "" {
		return fmt.Errorf("файл макроса відсутній, створіть макрос спочатку")
	}
	var graph []Puzzle
	if macro.GraphVersion == 1 {
		var values map[string]string
		var e error
		graph, values, e = graphPath(*macro, event)
		if e != nil {
			return e
		}
		if vars == nil {
			vars = map[string]string{}
		}
		for k, v := range values {
			vars[k] = v
		}
	}
	runKey := id
	if macro.GraphVersion == 1 {
		count := 0
		for _, p := range macro.Puzzles {
			if p.Type == "input" {
				count++
			}
		}
		if count > 1 {
			runKey = id + "/" + rootInputFor(*macro, event.Input).ID
		}
	}
	a.mu.Lock()
	if a.running[runKey] || len(a.running) >= 8 {
		a.mu.Unlock()
		return ErrMacroBusy
	}
	ctx, cancel := context.WithCancel(context.Background())
	if macro.GraphVersion != 1 {
		cancel()
		ctx, cancel = context.WithTimeout(context.Background(), 60*time.Second)
	}
	if a.cancels == nil {
		a.cancels = map[string]context.CancelFunc{}
	}
	a.cancels[runKey] = cancel
	a.running[runKey] = true
	if macro.GraphVersion != 1 && event.Kind == "slider" {
		for key, value := range vars {
			a.cfg.Variables[key] = value
		}
	}
	a.mu.Unlock()
	go func() {
		defer func() { a.mu.Lock(); delete(a.running, runKey); delete(a.cancels, runKey); a.mu.Unlock() }()

		defer cancel()
		if err := a.warmMacroVariables(ctx, *macro); err != nil {
			a.log(macro.Name + " · variables: " + err.Error())
			return
		}
		if macro.GraphVersion == 1 {
			a.mu.Lock()
			for k, v := range vars {
				a.cfg.Variables[k] = v
			}
			a.mu.Unlock()
			scope.Nodes = macro.Puzzles
			a.executeGraph(ctx, macro.Name, graph, scope)
			return
		}
		c := a.runtimeConfig()
		for k, v := range vars {
			c.Variables[k] = v
		}
		expand := func(s string) string {
			for k, v := range c.Variables {
				s = strings.ReplaceAll(s, "${"+k+"}", v)
			}
			return s
		}
		for i, s := range macro.Steps {
			if ctx.Err() != nil {
				a.log(macro.Name + ": timeout")
				return
			}
			value := expand(s.Value)
			var e error
			switch s.Type {
			case "delay":
				select {
				case <-time.After(time.Duration(s.MS) * time.Millisecond):
				case <-ctx.Done():
					e = ctx.Err()
				}
			case "launch":
				args := append([]string{}, s.Args...)
				for j := range args {
					args[j] = expand(args[j])
				}
				cmd := exec.Command(value, args...)
				hideConsole(cmd)
				e = cmd.Start()
				if e == nil {
					go cmd.Wait()
				}
			case "shell":
				cmd := shellCommand(ctx, value)
				hideConsole(cmd)
				configureCommandCancellation(cmd)
				var output limitedBuffer
				cmd.Stdout = &output
				cmd.Stderr = &output
				e = cmd.Run()
				if output.Len() > 0 {
					a.log(output.String())
				}
			case "text":
				e = typeTextContext(ctx, value)
			case "keys":
				e = sendKeys(value)
			case "variable":
				if len(s.Args) != 1 || !idPattern.MatchString(value) {
					e = fmt.Errorf("variable needs name and one argument")
				} else {
					v := expand(s.Args[0])
					c.Variables[value] = v
					a.mu.Lock()
					a.cfg.Variables[value] = v
					a.mu.Unlock()
				}
			case "module":
				e = a.module(ctx, value, c.Variables)
			}
			if e != nil {
				a.log(fmt.Sprintf("%s · step %d: %v", macro.Name, i+1, e))
				return
			}
		}
		a.log(time.Now().Format("15:04:05") + " ✓ " + macro.Name)
	}()
	return nil
}

type limitedBuffer struct{ bytes.Buffer }

func (b *limitedBuffer) Write(p []byte) (int, error) {
	n := len(p)
	if b.Len() < 8192 {
		left := 8192 - b.Len()
		if len(p) > left {
			p = p[:left]
		}
		b.Buffer.Write(p)
	}
	return n, nil
}
func (a *App) module(ctx context.Context, value string, vars map[string]string) error {
	parts := strings.SplitN(value, "/", 2)
	if len(parts) != 2 {
		return fmt.Errorf("module value: id/action")
	}
	a.mu.RLock()
	enabled := a.cfg.Modules[parts[0]]
	list := append([]Module{}, a.moduleList...)
	a.mu.RUnlock()
	if !enabled {
		return fmt.Errorf("module disabled")
	}
	for _, m := range list {
		if m.ID != parts[0] {
			continue
		}
		if m.Error != "" {
			return fmt.Errorf("%s", m.Error)
		}
		known := false
		for _, action := range m.Actions {
			if action == parts[1] {
				known = true
			}
		}
		if !known {
			return fmt.Errorf("action not declared")
		}
		req, _ := json.Marshal(map[string]any{"api": 1, "action": parts[1], "variables": vars})
		cmd := exec.CommandContext(ctx, m.Executable, m.Args...)
		hideConsole(cmd)
		cmd.Dir = filepath.Dir(m.Executable)
		cmd.Stdin = bytes.NewReader(append(req, '\n'))
		var out limitedBuffer
		cmd.Stdout = &out
		cmd.Stderr = &out
		if e := cmd.Run(); e != nil {
			return e
		}
		var reply struct {
			OK        bool              `json:"ok"`
			Message   string            `json:"message"`
			Variables map[string]string `json:"variables"`
		}
		if e := json.Unmarshal(out.Bytes(), &reply); e != nil {
			return fmt.Errorf("invalid module JSON: %w", e)
		}
		if !reply.OK {
			return fmt.Errorf("%s", reply.Message)
		}
		a.mu.Lock()
		for k, v := range reply.Variables {
			a.cfg.Variables[k] = v
			vars[k] = v
		}
		a.mu.Unlock()
		a.log(reply.Message)
		return nil
	}
	return fmt.Errorf("module not installed")
}
