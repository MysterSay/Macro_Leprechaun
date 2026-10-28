package main

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"
	"unicode/utf16"
	"unicode/utf8"
)

type CompareOperand struct {
	Variable      string `json:"variable"`
	Value         string `json:"value"`
	ValueVariable string `json:"valueVariable,omitempty"`
}
type Condition struct {
	Operator string         `json:"operator"`
	First    CompareOperand `json:"first"`
	Second   CompareOperand `json:"second"`
}
type AppAction struct {
	Interval     int64     `json:"interval,omitempty"`
	All          bool      `json:"all,omitempty"`
	Macros       []string  `json:"macros,omitempty"`
	Panel        string    `json:"panel,omitempty"`
	Macro        string    `json:"macro,omitempty"`
	Variable     string    `json:"variable,omitempty"`
	Source       string    `json:"source,omitempty"`
	Destination  string    `json:"destination,omitempty"`
	Path         string    `json:"path,omitempty"`
	Value        string    `json:"value,omitempty"`
	Milliseconds int64     `json:"milliseconds,omitempty"`
	Condition    Condition `json:"condition,omitempty"`
}
type RunScope struct {
	Nodes                      []Puzzle
	Session, Mode, Host, Panel string
	Chain                      []string
}
type Navigation struct {
	Current string
	History []string
}

func parseAppAction(p Puzzle) (AppAction, error) {
	var q AppAction
	if strings.HasPrefix(strings.TrimSpace(p.Value), "{") {
		if e := json.Unmarshal([]byte(p.Value), &q); e != nil {
			return q, e
		}
	} else {
		switch p.Type {
		case "actions.delay":
			n, e := strconv.ParseInt(p.Value, 10, 64)
			if e != nil && p.Value != "" {
				return q, e
			}
			q.Milliseconds = n
		case "folders.change":
			q.Panel = p.Value
		case "actions.macro":
			q.Macro = p.Value
		default:
			q.Variable = p.Value
		}
	}
	return q, nil
}
func conditionMatches(q Condition, vars map[string]string) (bool, error) {
	test := func(o CompareOperand) (bool, error) {
		v, ok := vars[o.Variable]
		if !ok {
			return false, fmt.Errorf("змінну %s не знайдено", o.Variable)
		}
		expected := o.Value
		if o.ValueVariable != "" {
			var ok bool
			expected, ok = vars[o.ValueVariable]
			if !ok {
				return false, fmt.Errorf("змінну %s не знайдено", o.ValueVariable)
			}
		}
		return v == expected, nil
	}
	a, e := test(q.First)
	if e != nil {
		return false, e
	}
	switch q.Operator {
	case "if":
		return a, nil
	case "else":
		return !a, nil
	case "and", "not", "or":
		b, e := test(q.Second)
		if e != nil {
			return false, e
		}
		switch q.Operator {
		case "and":
			return a && b, nil
		case "not":
			return !a && !b, nil
		default:
			return a || b, nil
		}
	}
	return false, fmt.Errorf("невідома умова")
}
func (a *App) executeAppAction(ctx context.Context, p Puzzle, scope RunScope) (bool, error) {
	q, e := parseAppAction(p)
	if e != nil {
		return false, e
	}
	switch p.Type {
	case "folders.change", "folders.parent", "folders.root":
		return true, a.navigate(scope, p.Type, q.Panel)
	case "actions.delay":
		if q.Milliseconds < 0 || q.Milliseconds > 366*24*60*60*1000-1 {
			return false, fmt.Errorf("затримка: 0–365 днів 23:59:59.999")
		}
		select {
		case <-time.After(time.Duration(q.Milliseconds) * time.Millisecond):
			return true, nil
		case <-ctx.Done():
			return false, ctx.Err()
		}
	case "actions.condition":
		return conditionMatches(q.Condition, a.variablesSnapshot())
	case "actions.wait":
		ticker := time.NewTicker(100 * time.Millisecond)
		defer ticker.Stop()
		for {
			ok, e := conditionMatches(q.Condition, a.variablesSnapshot())
			if e != nil || ok {
				return ok, e
			}
			select {
			case <-ctx.Done():
				return false, ctx.Err()
			case <-ticker.C:
			}
		}
	case "actions.stop":
		a.stopMacros(q.Macros, q.All)
		return !q.All && !containsString(q.Macros, lastChain(scope)), nil
	case "actions.macro":
		return true, a.launchChild(q.Macro, scope)
	case "actions.set":
		var value string
		switch q.Source {
		case "static":
			value = q.Value
		case "selection":
			textActionMu.Lock()
			value, e = readSelectedTextContext(ctx)
			textActionMu.Unlock()
		case "clipboard":
			textActionMu.Lock()
			value, e = readClipboardText()
			textActionMu.Unlock()
		case "file":
			var b []byte
			file, openErr := os.Open(q.Path)
			if openErr != nil {
				return false, openErr
			}
			b, e = io.ReadAll(io.LimitReader(file, (1<<20)+1))
			file.Close()
			if len(b) > 1<<20 {
				return false, fmt.Errorf("файл змінної перевищує 1 MiB")
			}
			value, e = decodeVariableText(b)
		default:
			return false, fmt.Errorf("джерело змінної не вибрано")
		}
		if e != nil {
			return false, e
		}
		if ctx.Err() != nil {
			return false, ctx.Err()
		}
		a.mu.Lock()
		defer a.mu.Unlock()
		if readonlyVariable(q.Variable) {
			return false, fmt.Errorf("стандартна змінна або змінна доповнення доступна лише для читання")
		}
		if _, ok := a.cfg.Variables[q.Variable]; !ok {
			return false, fmt.Errorf("змінну не знайдено")
		}
		a.cfg.Variables[q.Variable] = value
		return true, nil
	case "actions.get":
		value, ok := a.variablesSnapshot()[q.Variable]
		if !ok {
			return false, fmt.Errorf("змінну не знайдено")
		}
		switch q.Destination {
		case "field":
			textActionMu.Lock()
			e = executeTextPuzzleContext(ctx, Puzzle{Type: "text.insert", Value: value})
			textActionMu.Unlock()
		case "clipboard":
			textActionMu.Lock()
			e = executeTextPuzzleContext(ctx, Puzzle{Type: "text.clipboard", Value: value})
			textActionMu.Unlock()
		case "file":
			e = os.WriteFile(q.Path, []byte(value), 0600)
		default:
			e = fmt.Errorf("призначення змінної не вибрано")
		}
		return e == nil, e
	}
	return true, fmt.Errorf("невідома дія %s", p.Type)
}
func (a *App) launchChild(id string, scope RunScope) error {
	for _, m := range a.runtimeConfig().Macros {
		if m.ID != id {
			continue
		}
		event := InputEvent{Kind: "button", Value: "click"}
		if root := rootInput(m); root != nil {
			event.Kind = root.Mode
			if root.Mode == "toggle" {
				event.Value = "true"
			}
			if root.Mode == "slider" {
				event.Value = a.variablesSnapshot()[root.Variable]
			}
		}
		return a.runScoped(id, nil, event, scope)
	}
	return fmt.Errorf("макрос не знайдено")
}
func navKey(s RunScope) string { return s.Session + "|" + s.Mode + "|" + s.Host }
func modePanels(c Config, mode string) []Panel {
	switch mode {
	case "widgets":
		return c.Widgets
	case "floating":
		return c.Floating
	default:
		return c.Smart
	}
}
func (a *App) activePanel(scope RunScope) string {
	c := a.runtimeConfig()
	panels := modePanels(c, scope.Mode)
	a.mu.RLock()
	state := a.navigation[navKey(scope)]
	a.mu.RUnlock()
	for _, p := range panels {
		if p.ID == state.Current && p.Enabled {
			return p.ID
		}
	}
	if scope.Panel != "" {
		for _, p := range panels {
			if p.ID == scope.Panel {
				return p.ID
			}
		}
	}
	for _, p := range panels {
		if p.Enabled && p.Parent == "" {
			return p.ID
		}
	}
	for _, p := range panels {
		if p.Enabled {
			return p.ID
		}
	}
	return ""
}
func (a *App) navigate(scope RunScope, action, target string) error {
	if scope.Mode == "" {
		scope = RunScope{Session: "admin", Mode: "smart"}
	}
	panels := modePanels(a.runtimeConfig(), scope.Mode)
	current := a.activePanel(scope)
	root := ""
	for _, p := range panels {
		if p.Enabled && p.Parent == "" {
			root = p.ID
			break
		}
	}
	if root == "" {
		return fmt.Errorf("головну панель не знайдено")
	}
	a.mu.Lock()
	if a.navigation == nil {
		a.navigation = map[string]Navigation{}
	}
	state := a.navigation[navKey(scope)]
	state.Current = current
	switch action {
	case "folders.root":
		target = root
		state.History = nil
	case "folders.parent":
		if len(state.History) > 0 {
			target = state.History[len(state.History)-1]
			state.History = state.History[:len(state.History)-1]
		} else {
			target = root
			for _, p := range panels {
				if p.ID == current && p.Parent != "" {
					target = p.Parent
				}
			}
		}
	case "folders.change":
		if target != current {
			state.History = append(state.History, current)
			if len(state.History) > 100 {
				state.History = state.History[1:]
			}
		}
	}
	found := false
	for _, p := range panels {
		if p.ID == target && p.Enabled {
			found = true
			break
		}
	}
	if !found {
		a.mu.Unlock()
		return fmt.Errorf("панель відсутня, вимкнена або належить іншому режиму")
	}
	state.Current = target
	a.navigation[navKey(scope)] = state
	a.mu.Unlock()
	if scope.Mode != "smart" {
		platformNavigationRefresh()
	}
	return nil
}

func decodeVariableText(b []byte) (string, error) {
	if len(b) >= 2 && (b[0] == 0xff && b[1] == 0xfe || b[0] == 0xfe && b[1] == 0xff) {
		little := b[0] == 0xff
		b = b[2:]
		if len(b)%2 != 0 {
			return "", fmt.Errorf("пошкоджений UTF-16 файл")
		}
		units := make([]uint16, len(b)/2)
		for i := range units {
			if little {
				units[i] = binary.LittleEndian.Uint16(b[i*2:])
			} else {
				units[i] = binary.BigEndian.Uint16(b[i*2:])
			}
		}
		return string(utf16.Decode(units)), nil
	}
	b = []byte(strings.TrimPrefix(string(b), "\ufeff"))
	if !utf8.Valid(b) {
		return "", fmt.Errorf("потрібен UTF-8 або UTF-16 документ із BOM")
	}
	return string(b), nil
}

func (a *App) variablesSnapshot() map[string]string {
	a.mu.RLock()
	defer a.mu.RUnlock()
	values := make(map[string]string, len(a.cfg.Variables))
	for k, v := range a.cfg.Variables {
		values[k] = v
	}
	for k, v := range a.systemVars {
		values[k] = v
	}
	for k, v := range a.moduleVars {
		values[k] = v
	}
	if a.systemVars != nil {
		now := time.Now()
		values["system__time"] = now.Format("15:04:05")
		values["system__date"] = now.Format("2006-01-02")
	}
	return values
}

func containsString(list []string, id string) bool {
	for _, v := range list {
		if v == id {
			return true
		}
	}
	return false
}
func lastChain(s RunScope) string {
	if len(s.Chain) == 0 {
		return ""
	}
	return s.Chain[len(s.Chain)-1]
}
func (a *App) stopMacros(ids []string, all bool) {
	a.mu.RLock()
	list := []context.CancelFunc{}
	for id, cancel := range a.cancels {
		matched := all || containsString(ids, id)
		for _, macroID := range ids {
			if strings.HasPrefix(id, macroID+"/") {
				matched = true
			}
		}
		if matched {
			list = append(list, cancel)
		}
	}
	a.mu.RUnlock()
	for _, cancel := range list {
		cancel()
	}
}

func (a *App) loopsAllowed() bool {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.cfg.Settings.AllowLoops
}
