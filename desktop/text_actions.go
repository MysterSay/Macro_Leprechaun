package main

import (
	"fmt"
	"strings"
	"sync"
	"unicode/utf16"
)

// Input synthesis and selection changes must not interleave between macros.
var textActionMu sync.Mutex

func transformSelection(text, rules string) (string, error) {
	pairs := []string{}
	seen := map[string]bool{}
	for i, line := range strings.Split(strings.ReplaceAll(rules, "\r\n", "\n"), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		old, replacement, ok := strings.Cut(line, " - ")
		if !ok && strings.HasSuffix(line, " -") {
			old = strings.TrimSuffix(line, " -")
			replacement = ""
			ok = true
		}
		if !ok || old == "" || seen[old] {
			return "", fmt.Errorf("правило %d: очікується унікальне «текст - заміна»", i+1)
		}
		seen[old] = true
		pairs = append(pairs, old, replacement)
	}
	if len(pairs) == 0 {
		return "", fmt.Errorf("список замін порожній")
	}
	return strings.NewReplacer(pairs...).Replace(text), nil
}

// Windows selection positions count UTF-16 code units, not Unicode code points.
func selectionBounds(text []uint16, caret int, mode string) (int, int, error) {
	if caret < 0 || caret > len(text) {
		return 0, 0, fmt.Errorf("позиція курсора недоступна")
	}
	lo, hi := caret, caret
	switch mode {
	case "", "all":
		return 0, len(text), nil
	case "before":
		return 0, caret, nil
	case "after":
		return caret, len(text), nil
	case "line":
		for lo > 0 && text[lo-1] != '\n' && text[lo-1] != '\r' {
			lo--
		}
		for hi < len(text) && text[hi] != '\n' && text[hi] != '\r' {
			hi++
		}
	case "sentence":
		boundary := func(c uint16) bool { return c == '.' || c == '!' || c == '?' || c == '\n' || c == '\r' }
		for lo > 0 && !boundary(text[lo-1]) {
			lo--
		}
		for lo < len(text) && (text[lo] == ' ' || text[lo] == '\t') {
			lo++
		}
		for hi < len(text) && !boundary(text[hi]) {
			hi++
		}
		if hi < len(text) && (text[hi] == '.' || text[hi] == '!' || text[hi] == '?') {
			hi++
		}
	default:
		return 0, 0, fmt.Errorf("невідомий режим виділення")
	}
	if lo > hi {
		lo = hi
	}
	return lo, hi, nil
}
func findSelection(text []uint16, query string, from int) (int, int, error) {
	needle := utf16.Encode([]rune(query))
	if len(needle) == 0 {
		return 0, 0, fmt.Errorf("текст пошуку порожній")
	}
	if from < 0 || from > len(text) {
		from = 0
	}
	best, bestDistance := -1, len(text)+1
	for i := 0; i+len(needle) <= len(text); i++ {
		matches := true
		for j, v := range needle {
			if text[i+j] != v {
				matches = false
				break
			}
		}
		if !matches {
			continue
		}
		distance := 0
		if from < i {
			distance = i - from
		} else if from > i+len(needle) {
			distance = from - i - len(needle)
		}
		// On a tie choose the occurrence at/after the caret.
		if distance < bestDistance || (distance == bestDistance && i >= from && best < from) {
			best, bestDistance = i, distance
		}
	}
	if best < 0 {
		return 0, 0, fmt.Errorf("текст не знайдено")
	}
	return best, best + len(needle), nil
}
