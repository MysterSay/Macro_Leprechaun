package main

import (
	"fmt"
	"net"
	"net/http"
	"time"
)

// Administration remains on loopback. Closing these independent servers also
// closes existing phone keep-alive connections when smart mode is disabled.
func (a *App) servePhoneLAN(port int) {
	servers := map[string]*http.Server{}
	var stopMDNS func()
	for {
		wanted := map[string]bool{}
		if !a.runtimeConfig().Settings.DisabledSmart {
			addresses, _ := net.InterfaceAddrs()
			for _, address := range addresses {
				ip, _, err := net.ParseCIDR(address.String())
				if err == nil && ip.To4() != nil && !ip.IsLoopback() {
					wanted[ip.String()] = true
				}
			}
			if stopMDNS == nil {
				stop, err := startLocalMDNS(a.log)
				if err == nil {
					stopMDNS = stop
				}
			}
		} else if stopMDNS != nil {
			stopMDNS()
			stopMDNS = nil
		}
		a.reconcilePhoneServers(servers, wanted, port)
		time.Sleep(250 * time.Millisecond)
	}
}

func (a *App) reconcilePhoneServers(servers map[string]*http.Server, wanted map[string]bool, port int) {
	for ip, srv := range servers {
		if !wanted[ip] {
			srv.Close()
			delete(servers, ip)
		}
	}
	for ip := range wanted {
		if servers[ip] != nil {
			continue
		}
		listener, err := net.Listen("tcp4", net.JoinHostPort(ip, fmt.Sprint(port)))
		if err != nil {
			continue
		}
		srv := &http.Server{Handler: a.routes(), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 10 * time.Second, WriteTimeout: 70 * time.Second, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 16384}
		servers[ip] = srv
		go srv.Serve(listener)
	}
}
