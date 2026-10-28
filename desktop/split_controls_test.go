package main

import (
	"context"
	"testing"
	"time"
)

func TestSplitInputsRouteAndCancelIndependently(t *testing.T) {
	a := testApp(t)
	m := Macro{ID: "split", Name: "Split", GraphVersion: 1, Steps: []Step{}, Puzzles: []Puzzle{{ID: "a", Type: "input", Mode: "button", Next: "wa"}, {ID: "b", Type: "input", Mode: "button", Next: "wb"}, {ID: "wa", Type: "actions.wait", Value: actionPuzzle("actions.wait", AppAction{Condition: Condition{Operator: "if", First: CompareOperand{Variable: "name", Value: "wait_forever"}}}).Value}, {ID: "wb", Type: "actions.wait", Value: actionPuzzle("actions.wait", AppAction{Condition: Condition{Operator: "if", First: CompareOperand{Variable: "name", Value: "wait_forever"}}}).Value}}}
	if err := validateGraph(m); err != nil {
		t.Fatal(err)
	}
	a.cfg.Macros = append(a.cfg.Macros, m)
	for _, input := range []string{"a", "b"} {
		path, _, err := graphPath(m, InputEvent{Kind: "button", Value: "click", Input: input})
		if err != nil || len(path) != 1 || path[0].ID != "w"+input {
			t.Fatal(path, err)
		}
		if err := a.runInput(m.ID, nil, InputEvent{Kind: "button", Value: "click", Input: input}); err != nil {
			t.Fatal(err)
		}
	}
	a.mu.RLock()
	n := len(a.running)
	a.mu.RUnlock()
	if n != 2 {
		t.Fatal(n)
	}
	a.stopMacros([]string{m.ID}, false)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	for {
		a.mu.RLock()
		n = len(a.running)
		a.mu.RUnlock()
		if n == 0 {
			break
		}
		select {
		case <-ctx.Done():
			t.Fatal("split branches not cancelled")
		default:
			time.Sleep(time.Millisecond)
		}
	}
	m.Puzzles[2].Next = "b"
	if validateGraph(m) == nil {
		t.Fatal("connection into input accepted")
	}
}
func TestButtonGeometryCannotOverwrite(t *testing.T) {
	p := Panel{Columns: 4, Rows: 3, Controls: []Control{{ID: "a", Cell: intRef(0), ColSpan: 2, RowSpan: 2}, {ID: "b", Cell: intRef(2)}}}
	if err := validateControlGeometry(p, 0); err != nil {
		t.Fatal(err)
	}
	p.Controls[1].Cell = intRef(5)
	if validateControlGeometry(p, 0) == nil {
		t.Fatal("covered cell accepted")
	}
	p.Controls[1].Cell = intRef(3)
	p.Controls[1].ColSpan = 2
	if validateControlGeometry(p, 0) == nil {
		t.Fatal("row wrapping accepted")
	}
}
func intRef(n int) *int { return &n }
func TestSplitControlStorage(t *testing.T) {
	a := testApp(t)
	m := Macro{ID: "parts", Name: "Parts", GraphVersion: 1, Steps: []Step{}, Puzzles: []Puzzle{{ID: "first", Type: "input", Mode: "button"}, {ID: "second", Type: "input", Mode: "slider", Max: 200}}}
	a.cfg.Macros = append(a.cfg.Macros, m)
	a.cfg.Smart[0].Controls = []Control{{ID: "envelope", Kind: "button", Macro: m.ID, Cell: intRef(0), ColSpan: 2, Segments: []Control{{ID: "one", Kind: "button", Macro: m.ID, Input: "first"}, {ID: "two", Kind: "slider", Macro: m.ID, Input: "second"}}}}
	if err := a.save(a.cfg); err != nil {
		t.Fatal(err)
	}
	restored, err := loadStorage(a.root)
	if err != nil {
		t.Fatal(err)
	}
	parts := controlLeaves(restored.Smart[0].Controls)
	if len(parts) != 2 || parts[1].Input != "second" {
		t.Fatal(parts)
	}
	max, _ := sliderSpec(restored, parts[1])
	if max != 200 {
		t.Fatal(max)
	}
}
