package main

import (
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"
)

func TestDesktopPanelNavigation(t *testing.T) {
	a := testApp(t)
	a.cfg.Widgets = append(a.cfg.Widgets, Panel{ID: "widgetchild", Parent: a.cfg.Widgets[0].ID, Name: "Child", Enabled: true, Columns: 1, Edge: "right", Thickness: 4, Position: 50, Controls: []Control{}})
	scope := RunScope{Session: "desktop", Mode: "widgets", Host: a.cfg.Widgets[0].ID, Panel: a.cfg.Widgets[0].ID}
	if err := a.navigate(scope, "folders.change", "widgetchild"); err != nil {
		t.Fatal(err)
	}
	if a.activePanel(scope) != "widgetchild" {
		t.Fatal("did not switch")
	}
	if err := a.navigate(scope, "folders.parent", ""); err != nil {
		t.Fatal(err)
	}
	if a.activePanel(scope) != scope.Panel {
		t.Fatal("did not go back")
	}
}
func TestDisabledPhoneRequests(t *testing.T) {
	a := testApp(t)
	a.cfg.Settings.DisabledSmart = true
	r := httptest.NewRequest("GET", "/api/discover", nil)
	r.RemoteAddr = "192.168.0.7:1234"
	w := httptest.NewRecorder()
	a.routes().ServeHTTP(w, r)
	if w.Code != 503 {
		t.Fatal(w.Code)
	}
	r = httptest.NewRequest("GET", "/api/discover", nil)
	r.RemoteAddr = "127.0.0.1:1234"
	w = httptest.NewRecorder()
	a.routes().ServeHTTP(w, r)
	if w.Code != 200 {
		t.Fatal(w.Code)
	}
}
func TestSinglePanelRoot(t *testing.T) {
	c := defaults()
	p := c.Smart[0]
	p.ID = "another"
	c.Smart = append(c.Smart, p)
	if validate(c) == nil {
		t.Fatal("multiple roots accepted")
	}
	normalizePanelRoots(&c)
	if err := validate(c); err != nil {
		t.Fatal(err)
	}
	if c.Smart[1].Parent != c.Smart[0].ID {
		t.Fatal("migration")
	}
}

func TestPhoneListenerCloseAndRestore(t *testing.T) {
	a := testApp(t)
	l, err := net.Listen("tcp4", "127.0.0.2:0")
	if err != nil {
		t.Skip(err)
	}
	port := l.Addr().(*net.TCPAddr).Port
	l.Close()
	servers := map[string]*http.Server{}
	defer func() {
		for _, s := range servers {
			s.Close()
		}
	}()
	wanted := map[string]bool{"127.0.0.2": true}
	address := net.JoinHostPort("127.0.0.2", strconv.Itoa(port))
	a.reconcilePhoneServers(servers, wanted, port)
	c, err := net.DialTimeout("tcp4", address, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	c.Close()
	a.reconcilePhoneServers(servers, map[string]bool{}, port)
	c, err = net.DialTimeout("tcp4", address, time.Second)
	if err == nil {
		c.Close()
		t.Fatal("listener still open")
	}
	a.reconcilePhoneServers(servers, wanted, port)
	c, err = net.DialTimeout("tcp4", address, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	c.Close()
}
