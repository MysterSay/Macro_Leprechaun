package main

import "fmt"

func controlLeaves(controls []Control) []Control {
	result := []Control{}
	for _, b := range controls {
		if len(b.Segments) > 0 {
			result = append(result, controlLeaves(b.Segments)...)
		} else {
			result = append(result, b)
		}
	}
	return result
}
func controlSpan(b Control) (int, int) { return max(1, b.ColSpan), max(1, b.RowSpan) }
func validateControlGeometry(p Panel, mode int) error {
	occupied := map[int]bool{}
	ids := map[string]bool{}
	for i, b := range p.Controls {
		w, h := controlSpan(b)
		if b.ColSpan < 0 || b.RowSpan < 0 || w > p.Columns || h > 30 {
			return fmt.Errorf("invalid button span")
		}
		cell := i
		if b.Cell != nil {
			cell = *b.Cell
		}
		if cell < 0 || cell >= 240 || cell%p.Columns+w > p.Columns || cell/p.Columns+h > 30 {
			return fmt.Errorf("button outside grid")
		}
		if mode == 0 {
			for y := 0; y < h; y++ {
				for x := 0; x < w; x++ {
					pos := cell + y*p.Columns + x
					if occupied[pos] {
						return fmt.Errorf("buttons overlap")
					}
					occupied[pos] = true
				}
			}
		}
		if !idPattern.MatchString(b.ID) || ids[b.ID] {
			return fmt.Errorf("duplicate control ID")
		}
		ids[b.ID] = true
		if len(b.Segments) > 0 {
			if mode == 2 || len(b.Segments) < 2 || len(b.Segments) > 8 || !oneOf(b.SplitAxis, "", "horizontal", "vertical") {
				return fmt.Errorf("invalid split button")
			}
			for _, part := range b.Segments {
				if len(part.Segments) > 0 || part.Input == "" || part.Macro != b.Macro || ids[part.ID] || !idPattern.MatchString(part.ID) {
					return fmt.Errorf("invalid split segment")
				}
				ids[part.ID] = true
			}
		}
	}
	return nil
}
