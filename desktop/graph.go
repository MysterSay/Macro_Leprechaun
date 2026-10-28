package main

import (
	"context"
	"fmt"
	"strconv"
	"time"
)

type InputEvent struct{ Kind, Value, Input string }
type ControlState struct {
	Active bool    `json:"active"`
	Value  float64 `json:"value"`
}

func rootInput(m Macro) *Puzzle {
	if m.Lazy && m.Summary != nil {
		m.Puzzles = m.Summary.Inputs
	}
	for i := range m.Puzzles {
		if m.Puzzles[i].Type == "input" {
			return &m.Puzzles[i]
		}
	}
	return nil
}
func validateGraph(m Macro) error {
	nodes := map[string]Puzzle{}
	roots := map[string]bool{}
	for _, p := range m.Puzzles {
		if _, exists := nodes[p.ID]; exists {
			return fmt.Errorf("duplicate puzzle ID")
		}
		nodes[p.ID] = p
		if p.Type == "input" {
			roots[p.ID] = true
			if !oneOf(p.Mode, "button", "toggle", "slider") {
				return fmt.Errorf("input mode")
			}
			if p.Mode == "slider" && (p.Max <= 0 || p.Max > 1e9 || (p.Variable != "" && (readonlyVariable(p.Variable) || !idPattern.MatchString(p.Variable)))) {
				return fmt.Errorf("invalid slider input")
			}
		}
	}
	if len(roots) == 0 || len(roots) > 8 {
		return fmt.Errorf("one to eight inputs required")
	}
	parents := map[string]bool{}
	for _, p := range m.Puzzles {
		if p.Type != "actions.loop" && p.Body != "" {
			return fmt.Errorf("only loop has a body")
		}
		if p.Type != "input" && (p.Mode != "" || p.TrueNext != "" || p.FalseNext != "" || p.Variable != "" || p.Max != 0) {
			return fmt.Errorf("only input has branches/settings")
		}
		if p.Type == "input" {
			if p.Mode == "toggle" && p.Next != "" {
				return fmt.Errorf("toggle input uses true/false outputs")
			}
			if p.Mode != "toggle" && (p.TrueNext != "" || p.FalseNext != "") {
				return fmt.Errorf("single-output input")
			}
		}
		for _, target := range []string{p.Next, p.TrueNext, p.FalseNext, p.Body} {
			if target == "" {
				continue
			}
			if _, ok := nodes[target]; !ok || roots[target] || parents[target] {
				return fmt.Errorf("invalid connection or multiple parents")
			}
			parents[target] = true
		}
	}
	visited := map[string]int{}
	var visit func(string) error
	visit = func(id string) error {
		if visited[id] == 1 {
			return fmt.Errorf("puzzle graph contains a cycle")
		}
		if visited[id] == 2 {
			return nil
		}
		visited[id] = 1
		p := nodes[id]
		for _, n := range []string{p.Next, p.TrueNext, p.FalseNext, p.Body} {
			if n != "" {
				if e := visit(n); e != nil {
					return e
				}
			}
		}
		visited[id] = 2
		return nil
	}
	for id := range nodes {
		if e := visit(id); e != nil {
			return e
		}
	}
	return nil
}
func graphPath(m Macro, event InputEvent) ([]Puzzle, map[string]string, error) {
	if e := validateGraph(m); e != nil {
		return nil, nil, e
	}
	root := rootInputFor(m, event.Input)
	if root == nil {
		return nil, nil, fmt.Errorf("input not found")
	}
	if event.Kind != root.Mode {
		return nil, nil, fmt.Errorf("button type differs from input; reopen the macro editor")
	}
	next := root.Next
	vars := map[string]string{}
	switch root.Mode {
	case "toggle":
		if event.Value != "true" && event.Value != "false" {
			return nil, nil, fmt.Errorf("toggle value must be true/false")
		}
		if event.Value == "true" {
			next = root.TrueNext
		} else {
			next = root.FalseNext
		}
	case "slider":
		value, e := strconv.ParseFloat(event.Value, 64)
		if e != nil || !(value >= 0 && value <= root.Max) {
			return nil, nil, fmt.Errorf("slider value: 0..%g", root.Max)
		}
		if root.Variable != "" {
			vars[root.Variable] = strconv.FormatFloat(value, 'f', -1, 64)
		}
	}
	nodes := map[string]Puzzle{}
	for _, p := range m.Puzzles {
		nodes[p.ID] = p
	}
	path := []Puzzle{}
	for next != "" {
		p := nodes[next]
		path = append(path, p)
		next = p.Next
	}
	return path, vars, nil
}
func (a *App) inputMetadata(c Config) (map[string]any, map[string]ControlState) {
	inputs := map[string]any{}
	for _, panel := range c.Smart {
		for _, b := range controlLeaves(panel.Controls) {
			max, _ := sliderSpec(c, b)
			inputs[panel.ID+"/"+b.ID] = map[string]any{"max": max}
		}
	}
	a.mu.RLock()
	states := map[string]ControlState{}
	for k, v := range a.controlStates {
		states[k] = v
	}
	a.mu.RUnlock()
	for _, panel := range c.Smart {
		for _, b := range controlLeaves(panel.Controls) {
			key := panel.ID + "/" + b.ID
			if _, ok := states[key]; !ok {
				states[key] = initialControlState(c, b)
			}
		}
	}
	return inputs, states
}
func (a *App) executeGraph(ctx context.Context, name string, path []Puzzle, scope RunScope) {
	for _, p := range path {
		if ctx.Err() != nil {
			return
		}
		var e error
		if p.Type == "actions.loop" {
			if !a.loopsAllowed() {
				a.log(name + " · Цикл вимкнено в налаштуваннях; гілку завершено")
				return
			}
			q, e := parseAppAction(p)
			if e != nil {
				a.log(e.Error())
				return
			}
			interval := q.Interval
			if interval < 10 {
				interval = 10
			}
			for {
				if ctx.Err() != nil || !a.loopsAllowed() {
					return
				}
				ok, e := conditionMatches(q.Condition, a.variablesSnapshot())
				if e != nil {
					a.log(name + ": " + e.Error())
					return
				}
				if ok {
					break
				}
				body := []Puzzle{}
				id := p.Body
				for id != "" {
					found := false
					for _, n := range scope.Nodes {
						if n.ID == id {
							body = append(body, n)
							id = n.Next
							found = true
							break
						}
					}
					if !found {
						break
					}
				}
				a.executeGraph(ctx, name+" / цикл", body, scope)
				select {
				case <-ctx.Done():
					return
				case <-time.After(time.Duration(interval) * time.Millisecond):
				}
			}
			continue
		}
		if len(p.Type) > 7 && p.Type[:7] == "plugin:" {
			e = a.executePlugin(ctx, p)
			if e != nil {
				a.log(name + ": " + e.Error())
				return
			}
			continue
		}
		if len(p.Type) > 4 && p.Type[:4] == "win." {
			textActionMu.Lock()
			e = a.executeWinPuzzleContext(ctx, p)
			textActionMu.Unlock()
		} else if len(p.Type) > 5 && p.Type[:5] == "text." {
			textActionMu.Lock()
			e = executeTextPuzzleContext(ctx, p)
			textActionMu.Unlock()
		} else if len(p.Type) > 8 && p.Type[:8] == "actions." || len(p.Type) > 8 && p.Type[:8] == "folders." {
			var proceed bool
			proceed, e = a.executeAppAction(ctx, p, scope)
			if e == nil && !proceed {
				a.log(name + " · умова хибна; гілку зупинено")
				return
			}
		} else {
			a.log("↳ " + name + " · " + p.Type + " (дія ще не підключена)")
			continue
		}
		if e != nil {
			a.log(name + " · " + p.Type + ": " + e.Error())
			return
		}
		a.log("↳ " + name + " · " + p.Type + " ✓")
	}
	a.log(time.Now().Format("15:04:05") + " ✓ " + name)
}

func rootInputFor(m Macro, input string) *Puzzle {
	if m.Lazy && m.Summary != nil {
		m.Puzzles = m.Summary.Inputs
	}
	if input == "" {
		return rootInput(m)
	}
	for i := range m.Puzzles {
		if m.Puzzles[i].Type == "input" && m.Puzzles[i].ID == input {
			return &m.Puzzles[i]
		}
	}
	return nil
}
