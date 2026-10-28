package main

import (
	"encoding/binary"
	"fmt"
)

// INPUT is 40 bytes on Windows amd64; the keyboard union begins at offset 8.
// Keep construction independent from syscall/lifetime handling so the ABI is testable.
func keyboardPackets(units []uint16) [][40]byte {
	packets := make([][40]byte, 0, len(units)*2)
	add := func(vk, scan uint16, flags uint32) {
		var event [40]byte
		binary.LittleEndian.PutUint32(event[:4], 1)
		binary.LittleEndian.PutUint16(event[8:10], vk)
		binary.LittleEndian.PutUint16(event[10:12], scan)
		binary.LittleEndian.PutUint32(event[12:16], flags)
		packets = append(packets, event)
	}
	for _, u := range units {
		if u == 10 {
			add(0x10, 0, 0)
			add(13, 0, 0)
			add(13, 0, 2)
			add(0x10, 0, 2)
		} else {
			add(0, u, 4)
			add(0, u, 6)
		}
	}
	return packets
}

func mousePackets(name string) ([][40]byte, error) {
	var down, up, data uint32
	switch name {
	case "left":
		down, up = 2, 4
	case "right":
		down, up = 8, 16
	case "middle":
		down, up = 32, 64
	case "x1":
		down, up, data = 128, 256, 1
	case "x2":
		down, up, data = 128, 256, 2
	default:
		return nil, fmt.Errorf("unknown mouse button %q", name)
	}
	events := make([][40]byte, 2)
	for i, flag := range []uint32{down, up} {
		binary.LittleEndian.PutUint32(events[i][16:20], data)
		binary.LittleEndian.PutUint32(events[i][20:24], flag)
	}
	return events, nil
}
