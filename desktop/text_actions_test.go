package main

import (
	"testing"
	"unicode/utf16"
)

func TestTextTransformation(t *testing.T) {
	for _, c := range []struct{ in, rules, want string }{{"а б а", "а - б\nб - f", "б f б"}, {"Привіт, світе!", "світе - друже", "Привіт, друже!"}, {"🙂a🙂", "🙂 - ⭐\na -", "⭐⭐"}, {"A a", "a - f", "A f"}} {
		got, e := transformSelection(c.in, c.rules)
		if e != nil || got != c.want {
			t.Fatalf("%q %v", got, e)
		}
	}
	for _, s := range []string{"", "a f", "a - b\na - c"} {
		if _, e := transformSelection("a", s); e == nil {
			t.Fatal("invalid rule accepted")
		}
	}
}
func TestTextSelection(t *testing.T) {
	text := utf16.Encode([]rune("Привіт! Рядок 🙂 тут.\r\nДалі"))
	for _, c := range []struct {
		mode  string
		caret int
		want  string
	}{{"all", 0, string(utf16.Decode(text))}, {"sentence", 12, "Рядок 🙂 тут."}, {"line", 12, "Привіт! Рядок 🙂 тут."}, {"before", 7, "Привіт!"}, {"after", len(text), ""}} {
		lo, hi, e := selectionBounds(text, c.caret, c.mode)
		if e != nil || string(utf16.Decode(text[lo:hi])) != c.want {
			t.Fatalf("%s: %d %d %v", c.mode, lo, hi, e)
		}
	}
	lo, hi, e := findSelection(text, "🙂", 0)
	if e != nil || hi-lo != 2 {
		t.Fatal(lo, hi, e)
	}
	lo2, _, e := findSelection(text, "🙂", hi)
	if e != nil || lo2 != lo {
		t.Fatal("wrap")
	}
	if _, _, e = findSelection(text, "немає", 0); e == nil {
		t.Fatal("missing text accepted")
	}
}

func TestTransformPreservesWholeSelection(t *testing.T) {
	cases := []struct{ in, rules, want string }{
		{"незмінний початок а текст б кінець", "а - f\nб - g", "незмінний почfток f текст g кінець"},
		{"Початок. старе слово! Далі 🙂\nБез змін.", "старе слово - нове слово", "Початок. нове слово! Далі 🙂\nБез змін."},
		{"Тут нічого не міняємо 🙂", "xyz - abc", "Тут нічого не міняємо 🙂"},
		{"до abc після abc кінець", "abc -", "до  після  кінець"},
	}
	for _, c := range cases {
		got, e := transformSelection(c.in, c.rules)
		if e != nil || got != c.want {
			t.Fatalf("got %q want %q: %v", got, c.want, e)
		}
	}
}
func TestFindClosestToCaret(t *testing.T) {
	for _, c := range []struct {
		text, query string
		caret, want int
	}{
		{"ab----ab", "ab", 3, 0},
		{"ab----ab", "ab", 5, 6},
		{"ab----ab", "ab", 4, 6},
		{"ab----ab", "ab", 1, 0},
		{"abab", "ab", 2, 2},
		{"aaaaa", "aaa", 2, 2},
		{"🙂---🙂", "🙂", 3, 0},
		{"🙂---🙂", "🙂", 4, 5},
	} {
		lo, _, e := findSelection(utf16.Encode([]rune(c.text)), c.query, c.caret)
		if e != nil || lo != c.want {
			t.Fatalf("%q caret=%d got %d want %d: %v", c.text, c.caret, lo, c.want, e)
		}
	}
}
