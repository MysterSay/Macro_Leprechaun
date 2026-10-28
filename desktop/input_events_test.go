package main

import (
	"encoding/binary"
	"testing"
	"unicode/utf16"
)

func TestWindowsUnicodeInputABI(t *testing.T) {
	packets := keyboardPackets(utf16.Encode([]rune("Я🙂\n")))
	if len(packets) != 10 {
		t.Fatalf("event count %d", len(packets))
	}
	want := [][3]uint16{{0, 0x42f, 4}, {0, 0x42f, 6}, {0, 0xd83d, 4}, {0, 0xd83d, 6}, {0, 0xde42, 4}, {0, 0xde42, 6}, {16, 0, 0}, {13, 0, 0}, {13, 0, 2}, {16, 0, 2}}
	for i, p := range packets {
		if binary.LittleEndian.Uint32(p[:4]) != 1 {
			t.Fatal("not INPUT_KEYBOARD")
		}
		got := [3]uint16{binary.LittleEndian.Uint16(p[8:10]), binary.LittleEndian.Uint16(p[10:12]), uint16(binary.LittleEndian.Uint32(p[12:16]))}
		if got != want[i] {
			t.Fatalf("event %d: %x != %x", i, got, want[i])
		}
		for _, b := range p[16:] {
			if b != 0 {
				t.Fatal("unexpected timestamp/extra data")
			}
		}
	}
}
