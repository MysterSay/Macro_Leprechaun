package main

import (
	"context"
	"encoding/json"
	"testing"
	"time"
)

func loopMacro() Macro {
	value, _ := json.Marshal(AppAction{Interval: 10, Condition: Condition{Operator: "if", First: CompareOperand{Variable: "name", Value: "done"}}})
	return Macro{ID: "looptest", Name: "looptest", Steps: []Step{}, GraphVersion: 1, Puzzles: []Puzzle{{ID: "i", Type: "input", Mode: "button", Next: "l"}, {ID: "l", Type: "actions.loop", Value: string(value), Body: "b", Next: "n"}, {ID: "b", Type: "actions.set", Value: actionPuzzle("actions.set", AppAction{Variable: "name", Source: "static", Value: "done"}).Value}, {ID: "n", Type: "actions.set", Value: actionPuzzle("actions.set", AppAction{Variable: "volume", Source: "static", Value: "77"}).Value}}}
}
func TestLoopPermissionAndBranches(t *testing.T) {
	a := testApp(t)
	m := loopMacro()
	if e := validateGraph(m); e != nil {
		t.Fatal(e)
	}
	path, _, e := graphPath(m, InputEvent{Kind: "button"})
	if e != nil {
		t.Fatal(e)
	}
	a.executeGraph(context.Background(), m.Name, path, RunScope{Nodes: m.Puzzles})
	if a.config().Variables["name"] == "done" || a.config().Variables["volume"] == "77" {
		t.Fatal("disabled loop executed")
	}
	a.cfg.Settings.AllowLoops = true
	a.executeGraph(context.Background(), m.Name, path, RunScope{Nodes: m.Puzzles})
	if a.config().Variables["name"] != "done" || a.config().Variables["volume"] != "77" {
		t.Fatal("body or continuation did not run")
	}
	m.Puzzles[3].Next = "b"
	if e = validateGraph(m); e == nil {
		t.Fatal("shared branch accepted")
	}
}
func TestStopAllCancelsWaitAndDelay(t *testing.T) {
	a := testApp(t)
	for i, kind := range []string{"actions.wait", "actions.delay"} {
		q := AppAction{Milliseconds: 60000, Condition: Condition{Operator: "if", First: CompareOperand{Variable: "name", Value: "never"}}}
		m := Macro{ID: []string{"waittest", "delaytest"}[i], Name: kind, Steps: []Step{}, GraphVersion: 1, Puzzles: []Puzzle{{ID: "i", Type: "input", Mode: "button", Next: "a"}, {ID: "a", Type: kind, Value: actionPuzzle(kind, q).Value}}}
		a.cfg.Macros = append(a.cfg.Macros, m)
		if e := a.run(m.ID, nil); e != nil {
			t.Fatal(e)
		}
	}
	a.stopMacros(nil, true)
	until := time.Now().Add(time.Second)
	for time.Now().Before(until) {
		a.mu.RLock()
		n := len(a.running)
		a.mu.RUnlock()
		if n == 0 {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("stop did not cancel active macros")
}
func TestReservedVariablesReadOnly(t *testing.T) {
	a := testApp(t)
	a.systemVars = map[string]string{"system__time": "12:00:00"}
	if len(a.variablesSnapshot()["system__time"]) != 8 {
		t.Fatal("system vars missing")
	}
	if _, e := a.executeAppAction(context.Background(), actionPuzzle("actions.set", AppAction{Variable: "system__time", Source: "static", Value: "wrong"}), RunScope{}); e == nil {
		t.Fatal("system variable writable")
	}
}
