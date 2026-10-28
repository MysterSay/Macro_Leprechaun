package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"net"
	"strings"
	"time"
)

// Answers hostname A queries, including those from legacy mDNS clients.
// Unlike service advertisement alone, this makes macroleprechaun.local resolvable.
func startLocalMDNS(logf func(string)) (func(), error) {
	group := net.IPv4(224, 0, 0, 251)
	type listener struct {
		conn  *net.UDPConn
		iface net.Interface
	}
	var listeners []listener
	ifaces, err := net.Interfaces()
	if err != nil {
		return nil, err
	}
	for _, iface := range ifaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagMulticast == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		valid := false
		for _, a := range mustAddrs(&iface) {
			if ipnet, ok := a.(*net.IPNet); ok && ipnet.IP.To4() != nil && !ipnet.IP.IsLoopback() && !ipnet.IP.IsLinkLocalUnicast() {
				valid = true
			}
		}
		if !valid {
			continue
		}
		c, e := net.ListenMulticastUDP("udp4", &iface, &net.UDPAddr{IP: group, Port: 5353})
		if e != nil {
			logf("mDNS: " + iface.Name + ": " + e.Error())
			continue
		}
		_ = c.SetReadBuffer(65536)
		listeners = append(listeners, listener{c, iface})
	}
	if len(listeners) == 0 {
		return nil, fmt.Errorf("не вдалося приєднатися до UDP multicast 224.0.0.251:5353")
	}
	stop := make(chan struct{})
	for _, l := range listeners {
		go func(l listener) {
			buf := make([]byte, 4096)
			for {
				_ = l.conn.SetReadDeadline(time.Now().Add(time.Second))
				n, remote, e := l.conn.ReadFromUDP(buf)
				if e != nil {
					select {
					case <-stop:
						return
					default:
						continue
					}
				}
				if n < 12 || binary.BigEndian.Uint16(buf[2:4])&0x8000 != 0 {
					continue
				}
				answers := mdnsResponse(buf[:n], l.iface)
				if len(answers) == 0 {
					continue
				}
				// Unicast reply for legacy requests and QU questions; multicast for ordinary mDNS.
				dest := &net.UDPAddr{IP: group, Port: 5353}
				if remote.Port != 5353 || wantsUnicast(buf[:n]) {
					dest = remote
				}
				_, _ = l.conn.WriteToUDP(answers, dest)
			}
		}(l)
	}
	logf(fmt.Sprintf("mDNS: macroleprechaun.local активний на %d мережевих адаптерах", len(listeners)))
	return func() {
		close(stop)
		for _, l := range listeners {
			_ = l.conn.Close()
		}
	}, nil
}
func mustAddrs(i *net.Interface) []net.Addr { a, _ := i.Addrs(); return a }
func readName(packet []byte, p int) (string, int, bool) {
	var labels []string
	next := -1
	for k := 0; k < 30; k++ {
		if p >= len(packet) {
			return "", 0, false
		}
		b := int(packet[p])
		if b&0xc0 == 0xc0 {
			if p+1 >= len(packet) {
				return "", 0, false
			}
			if next < 0 {
				next = p + 2
			}
			p = ((b & 0x3f) << 8) | int(packet[p+1])
			continue
		}
		p++
		if b == 0 {
			if next < 0 {
				next = p
			}
			return strings.ToLower(strings.Join(labels, ".")) + ".", next, true
		}
		if b > 63 || p+b > len(packet) {
			return "", 0, false
		}
		labels = append(labels, string(packet[p:p+b]))
		p += b
	}
	return "", 0, false
}
func encodeName(s string) []byte {
	var b []byte
	for _, p := range strings.Split(strings.TrimSuffix(s, "."), ".") {
		if len(p) > 63 {
			return nil
		}
		b = append(b, byte(len(p)))
		b = append(b, p...)
	}
	return append(b, 0)
}
func addRR(b *bytes.Buffer, name string, typ uint16, data []byte) {
	b.Write(encodeName(name))
	_ = binary.Write(b, binary.BigEndian, typ)
	_ = binary.Write(b, binary.BigEndian, uint16(1))
	_ = binary.Write(b, binary.BigEndian, uint32(20))
	_ = binary.Write(b, binary.BigEndian, uint16(len(data)))
	b.Write(data)
}
func wantsUnicast(p []byte) bool {
	q := int(binary.BigEndian.Uint16(p[4:6]))
	off := 12
	for j := 0; j < q && j < 30; j++ {
		_, n, ok := readName(p, off)
		if !ok || n+4 > len(p) {
			return false
		}
		if binary.BigEndian.Uint16(p[n+2:n+4])&0x8000 != 0 {
			return true
		}
		off = n + 4
	}
	return false
}
func mdnsResponse(packet []byte, iface net.Interface) []byte {
	qcount := int(binary.BigEndian.Uint16(packet[4:6]))
	if qcount <= 0 || qcount > 40 {
		return nil
	}
	ips := []net.IP{}
	for _, a := range mustAddrs(&iface) {
		if i, ok := a.(*net.IPNet); ok {
			ip := i.IP.To4()
			if ip != nil && !ip.IsLoopback() && !ip.IsLinkLocalUnicast() {
				ips = append(ips, ip)
			}
		}
	}
	if len(ips) == 0 {
		return nil
	}
	off := 12
	var queries []struct {
		name string
		typ  uint16
	}
	for j := 0; j < qcount; j++ {
		name, n, ok := readName(packet, off)
		if !ok || n+4 > len(packet) {
			return nil
		}
		typ := binary.BigEndian.Uint16(packet[n : n+2])
		queries = append(queries, struct {
			name string
			typ  uint16
		}{name, typ})
		off = n + 4
	}
	var body bytes.Buffer
	count := 0
	for _, q := range queries {
		switch {
		case q.name == "macroleprechaun.local." && (q.typ == 1 || q.typ == 255):
			for _, ip := range ips {
				addRR(&body, "macroleprechaun.local.", 1, ip)
				count++
			}
		case (q.name == "_http._tcp.local." || q.name == "_macroleprechaun._tcp.local.") && (q.typ == 12 || q.typ == 255):
			service := "MacroLeprechaun." + q.name
			addRR(&body, q.name, 12, encodeName(service))
			count++
		case strings.HasPrefix(q.name, "macroleprechaun.") && (q.typ == 33 || q.typ == 255):
			var data bytes.Buffer
			_ = binary.Write(&data, binary.BigEndian, uint16(0))
			_ = binary.Write(&data, binary.BigEndian, uint16(0))
			_ = binary.Write(&data, binary.BigEndian, uint16(app.config().Settings.Port))
			data.Write(encodeName("macroleprechaun.local."))
			addRR(&body, q.name, 33, data.Bytes())
			count++
			for _, ip := range ips {
				addRR(&body, "macroleprechaun.local.", 1, ip)
				count++
			}
		case strings.HasPrefix(q.name, "macroleprechaun.") && q.typ == 16:
			t := "path=/remote.html"
			addRR(&body, q.name, 16, append([]byte{byte(len(t))}, []byte(t)...))
			count++
		}
	}
	if count == 0 {
		return nil
	}
	var out bytes.Buffer
	_ = binary.Write(&out, binary.BigEndian, binary.BigEndian.Uint16(packet[:2]))
	_ = binary.Write(&out, binary.BigEndian, uint16(0x8400))
	_ = binary.Write(&out, binary.BigEndian, uint16(0))
	_ = binary.Write(&out, binary.BigEndian, uint16(count))
	_ = binary.Write(&out, binary.BigEndian, uint32(0))
	out.Write(body.Bytes())
	return out.Bytes()
}
