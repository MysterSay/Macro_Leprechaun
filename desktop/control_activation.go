package main

import (
	"fmt"
	"strconv"
)

type ControlInputError struct{ message string }

func (e *ControlInputError) Error() string { return e.message }
func controlInputError(format string, args ...any) error {
	return &ControlInputError{fmt.Sprintf(format, args...)}
}
func sliderSpec(c Config, b Control) (float64, string) {
	maximum, variable := b.Appearance.SliderMax, b.Variable
	if maximum <= 0 {
		maximum = 100
	}
	for _, m := range c.Macros {
		if m.ID == b.Macro {
			if root := rootInputFor(m, b.Input); root != nil && root.Mode == "slider" {
				maximum = root.Max
				variable = root.Variable
			}
		}
	}
	return maximum, variable
}
func controlStateKey(mode, panel, control string) string {
	if mode == "" || mode == "smart" {
		return panel + "/" + control
	}
	return mode + "/" + panel + "/" + control
}
func (a *App) activatePanelControl(p Panel, b Control, value string, scope RunScope) error {
	c := a.runtimeConfig()
	event := InputEvent{Kind: b.Kind, Value: value, Input: b.Input}
	if event.Kind == "input" {
		event.Kind = "button"
	}
	id := b.Macro
	vars := map[string]string{}
	number := 0.0
	switch event.Kind {
	case "button":
		event.Value = "click"
	case "toggle":
		if !oneOf(value, "true", "false") {
			return controlInputError("toggle: true/false")
		}
		graph := false
		for _, m := range c.Macros {
			if m.ID == id && m.GraphVersion == 1 {
				graph = true
			}
		}
		if !graph && value == "false" && b.OffMacro != "" {
			id = b.OffMacro
		}
	case "slider":
		maximum, variable := sliderSpec(c, b)
		var err error
		number, err = strconv.ParseFloat(value, 64)
		if err != nil || !(number >= 0 && number <= maximum) {
			return controlInputError("slider value: 0..%g", maximum)
		}
		if variable != "" {
			if readonlyVariable(variable) {
				return controlInputError("slider variable is read-only")
			}
			vars[variable] = strconv.FormatFloat(number, 'f', -1, 64)
		}
	default:
		return controlInputError("unknown control kind")
	}
	if err := a.runScoped(id, vars, event, scope); err != nil {
		return err
	}
	// Unassigned sliders may still update their selected variable.
	a.mu.Lock()
	defer a.mu.Unlock()
	if id == "" {
		for key, v := range vars {
			a.cfg.Variables[key] = v
		}
	}
	if a.controlStates == nil {
		a.controlStates = map[string]ControlState{}
	}
	a.controlStates[controlStateKey(scope.Mode, p.ID, b.ID)] = ControlState{Active: event.Kind == "toggle" && value == "true", Value: number}
	return nil
}

func initialControlState(c Config, b Control) ControlState {
	state := ControlState{}
	if b.Kind == "slider" {
		maximum, variable := sliderSpec(c, b)
		if value, err := strconv.ParseFloat(c.Variables[variable], 64); err == nil && value >= 0 && value <= maximum {
			state.Value = value
		}
	}
	return state
}
