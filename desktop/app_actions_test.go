package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func actionPuzzle(kind string, q AppAction) Puzzle {
	b, _ := json.Marshal(q)
	return Puzzle{Type: kind, Value: string(b)}
}
func TestConditionsTruthTable(t *testing.T) {
	for _, aa := range []bool{false, true} {
		for _, bb := range []bool{false, true} {
			vars := map[string]string{"a": "no", "b": "no", "expected": "yes"}
			if aa {
				vars["a"] = "yes"
			}
			if bb {
				vars["b"] = "yes"
			}
			for op, want := range map[string]bool{"if": aa, "else": !aa, "and": aa && bb, "not": !aa && !bb, "or": aa || bb} {
				q := Condition{Operator: op, First: CompareOperand{Variable: "a", ValueVariable: "expected"}, Second: CompareOperand{Variable: "b", Value: "yes"}}
				got, e := conditionMatches(q, vars)
				if e != nil || got != want {
					t.Fatal(op, aa, bb, got, e)
				}
			}
		}
	}
}
func TestWaitDoesNotBlockVariableWriter(t *testing.T) {
	a := testApp(t)
	a.cfg.Variables["flag"] = "no"
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	done := make(chan error, 1)
	go func() {
		_, e := a.executeAppAction(ctx, actionPuzzle("actions.wait", AppAction{Condition: Condition{Operator: "if", First: CompareOperand{Variable: "flag", Value: "yes"}}}), RunScope{})
		done <- e
	}()
	_, e := a.executeAppAction(ctx, actionPuzzle("actions.set", AppAction{Variable: "flag", Source: "static", Value: "yes"}), RunScope{})
	if e != nil {
		t.Fatal(e)
	}
	if e = <-done; e != nil {
		t.Fatal(e)
	}
	ctx2, cancel2 := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel2()
	if _, e = a.executeAppAction(ctx2, actionPuzzle("actions.wait", AppAction{Condition: Condition{Operator: "if", First: CompareOperand{Variable: "flag", Value: "never"}}}), RunScope{}); e == nil {
		t.Fatal("wait ignored cancellation")
	}
}
func TestPanelHistoryAndSessionIsolation(t *testing.T) {
	a := testApp(t)
	a.cfg.Smart = append(a.cfg.Smart, Panel{ID: "child", Name: "Child", Parent: "phone", Enabled: true, Columns: 3, Controls: []Control{}}, Panel{ID: "leaf", Name: "Leaf", Parent: "child", Enabled: true, Columns: 3, Controls: []Control{}})
	s := RunScope{Session: "phone1", Mode: "smart", Panel: "phone"}
	for _, step := range []struct{ action, target, want string }{{"folders.change", "child", "child"}, {"folders.change", "leaf", "leaf"}, {"folders.parent", "", "child"}, {"folders.root", "", "phone"}} {
		if e := a.navigate(s, step.action, step.target); e != nil {
			t.Fatal(e)
		}
		if a.activePanel(s) != step.want {
			t.Fatal(step, a.activePanel(s))
		}
	}
	if a.activePanel(RunScope{Session: "phone2", Mode: "smart"}) != "phone" {
		t.Fatal("session leak")
	}
	if e := a.navigate(s, "folders.change", "missing"); e == nil {
		t.Fatal("unknown panel accepted")
	}
	a.cfg.Smart[0].Parent = "leaf"
	if e := validate(a.cfg); e == nil {
		t.Fatal("parent cycle accepted")
	}
}
func TestVariablesAndGraphGate(t *testing.T) {
	a := testApp(t)
	file := filepath.Join(t.TempDir(), "value.txt")
	if e := os.WriteFile(file, []byte("unchanged\n🙂"), 0600); e != nil {
		t.Fatal(e)
	}
	if _, e := a.executeAppAction(context.Background(), actionPuzzle("actions.set", AppAction{Variable: "name", Source: "file", Path: file}), RunScope{}); e != nil {
		t.Fatal(e)
	}
	if a.config().Variables["name"] != "unchanged\n🙂" {
		t.Fatal("file read")
	}
	out := filepath.Join(t.TempDir(), "out.txt")
	if _, e := a.executeAppAction(context.Background(), actionPuzzle("actions.get", AppAction{Variable: "name", Destination: "file", Path: out}), RunScope{}); e != nil {
		t.Fatal(e)
	}
	b, _ := os.ReadFile(out)
	if string(b) != "unchanged\n🙂" {
		t.Fatal("file write")
	}
	a.executeGraph(context.Background(), "gate", []Puzzle{actionPuzzle("actions.condition", AppAction{Condition: Condition{Operator: "if", First: CompareOperand{Variable: "name", Value: "no"}}}), actionPuzzle("actions.set", AppAction{Variable: "name", Source: "static", Value: "must not run"})}, RunScope{})
	if a.config().Variables["name"] == "must not run" {
		t.Fatal("false gate continued")
	}
}
func TestParallelMacroCycleGuard(t *testing.T) {
	a := testApp(t)
	if e := a.runScoped("notepad", nil, InputEvent{Kind: "button"}, RunScope{Chain: []string{"notepad"}}); e == nil {
		t.Fatal("recursive macro accepted")
	}
	m := Macro{ID: "child_macro", Name: "Child", Steps: []Step{}, GraphVersion: 1, Puzzles: []Puzzle{{ID: "input", Type: "input", Mode: "button", Next: "set"}, {ID: "set", Type: "actions.set", Value: actionPuzzle("actions.set", AppAction{Variable: "name", Source: "static", Value: "child ran"}).Value}}}
	a.cfg.Macros = append(a.cfg.Macros, m)
	if e := a.launchChild("child_macro", RunScope{Chain: []string{"parent"}}); e != nil {
		t.Fatal(e)
	}
	deadline := time.Now().Add(time.Second)
	for a.config().Variables["name"] != "child ran" && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if a.config().Variables["name"] != "child ran" {
		t.Fatal("parallel macro did not run")
	}
}

func TestVariableDocumentEncoding(t *testing.T) {
	for _, c := range []struct {
		b    []byte
		want string
	}{{[]byte{0xff, 0xfe, 0x2f, 0x04}, "Я"}, {[]byte{0xfe, 0xff, 0x04, 0x2f}, "Я"}, {[]byte("\xef\xbb\xbfhello"), "hello"}} {
		got, e := decodeVariableText(c.b)
		if e != nil || got != c.want {
			t.Fatal(got, e)
		}
	}
	if _, e := decodeVariableText([]byte{0xff, 0xfe, 1}); e == nil {
		t.Fatal("broken UTF16 accepted")
	}
}
