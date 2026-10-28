package main

import (
	"database/sql"
	"embed"
	"encoding/json"
	"flag"
	"fmt"
	"io/fs"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

//go:embed web
var assets embed.FS
var files http.Handler

const appVersion = "0.17.1"

var app *App

func main() {
	dir := flag.String("data", "", "data directory (default: executable/data)")
	uiParent := flag.Int("ui-parent", 0, "editor parent process")
	floatingURL := flag.String("floating-ui", "", "floating renderer process")
	settingsURL := flag.String("settings-ui", "", "private WebView2 editor process")
	noUI := flag.Bool("headless", false, "do not open settings")
	flag.Parse()
	exe, _ := os.Executable()
	if *dir == "" {
		*dir = filepath.Join(filepath.Dir(exe), "data")
	}
	if e := os.MkdirAll(*dir, 0700); e != nil {
		log.Fatal(e)
	}
	lf, e := os.OpenFile(filepath.Join(*dir, "application.log"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	if e != nil {
		log.Fatal(e)
	}
	defer lf.Close()
	log.SetOutput(lf)
	if *floatingURL != "" {
		if err := runFloatingWindow(*floatingURL, *dir, *uiParent); err != nil {
			log.Print(err)
			reportFloatingFailure(err)
		}
		return
	}
	if *settingsURL != "" {
		if err := runSettingsWindow(*settingsURL, *dir, *uiParent); err != nil {
			log.Print(err)
		}
		return
	}
	log.Printf("MacroLeprechaun %s starting; pid=%d; exe=%s; data=%s", appVersion, os.Getpid(), exe, *dir)
	app = &App{root: *dir, admin: token(), cfg: defaults(), devices: map[string]string{}, attempts: map[string]int{}, running: map[string]bool{}}
	if c, e := loadStorage(*dir); e == nil {
		app.cfg = c
	} else if e == sql.ErrNoRows {
		if b, e := os.ReadFile(filepath.Join(*dir, "config.json")); e == nil {
			if e = json.Unmarshal(b, &app.cfg); e != nil {
				log.Fatal(e)
			}
		} else if !os.IsNotExist(e) {
			log.Fatal(e)
		}
		if e = app.save(app.cfg); e != nil {
			log.Fatal(e)
		}
	} else {
		log.Fatal(e)
	}
	for _, kind := range []string{"icon", "image"} {
		p := filepath.Join(*dir, kind, "logo.png")
		if _, e := os.Stat(p); os.IsNotExist(e) {
			b, _ := assets.ReadFile("web/logo.png")
			_ = os.WriteFile(p, b, 0600)
		}
	}
	// Migrate previous data paths once, preserving existing target files.
	oldDevice := filepath.Join(*dir, "devices.json")
	newDevice := filepath.Join(*dir, "setting", "devices.json")
	if _, e := os.Stat(newDevice); os.IsNotExist(e) {
		if _, e = os.Stat(oldDevice); e == nil {
			if e = os.Rename(oldDevice, newDevice); e != nil {
				log.Fatal(e)
			}
		}
	}
	legacyModules, _ := os.ReadDir(filepath.Join(*dir, "modules"))
	for _, entry := range legacyModules {
		dst := filepath.Join(*dir, "plagin", entry.Name())
		if _, e := os.Stat(dst); os.IsNotExist(e) {
			if e = os.Rename(filepath.Join(*dir, "modules", entry.Name()), dst); e != nil {
				log.Fatal(e)
			}
		}
	}
	if b, e := os.ReadFile(filepath.Join(*dir, "setting", "devices.json")); e == nil {
		if e = json.Unmarshal(b, &app.devices); e != nil {
			log.Fatal(e)
		}
	}
	// External JSON dictionaries override embedded ones and make language additions independent of recompilation.
	_ = os.MkdirAll(filepath.Join(*dir, "locales"), 0700)
	for _, name := range []string{"uk", "en"} {
		p := filepath.Join(*dir, "locales", name+".json")
		if _, e := os.Stat(p); os.IsNotExist(e) {
			b, _ := assets.ReadFile("web/locales/" + name + ".json")
			_ = os.WriteFile(p, b, 0600)
		}
	}
	sub, _ := fs.Sub(assets, "web")
	static := http.FileServer(http.FS(sub))
	files = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if len(r.URL.Path) > 9 && r.URL.Path[:9] == "/locales/" {
			name := filepath.Base(r.URL.Path)
			if idPattern.MatchString(name[:len(name)-len(filepath.Ext(name))]) && filepath.Ext(name) == ".json" {
				if r.URL.Query().Get("builtin") == "1" {
					b, e := assets.ReadFile("web/locales/" + name)
					if e != nil {
						http.NotFound(w, r)
						return
					}
					w.Header().Set("Content-Type", "application/json; charset=utf-8")
					_, _ = w.Write(b)
					return
				}
				p := filepath.Join(*dir, "locales", name)
				if _, e := os.Stat(p); e == nil {
					http.ServeFile(w, r, p)
					return
				}
			}
		}
		if strings.HasPrefix(r.URL.Path, "/floating-effects/") {
			app.floatingAsset(w, r)
			return
		}
		if strings.HasPrefix(r.URL.Path, "/media/") {
			parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/media/"), "/")
			if len(parts) != 2 || (parts[0] != "icon" && parts[0] != "image") || filepath.Base(parts[1]) != parts[1] || !mediaName(parts[1]) {
				http.NotFound(w, r)
				return
			}
			w.Header().Set("Content-Security-Policy", "sandbox; default-src 'none'; style-src 'unsafe-inline'")
			http.ServeFile(w, r, filepath.Join(*dir, parts[0], parts[1]))
			return
		}
		static.ServeHTTP(w, r)
	})
	app.scanModules()
	if err := installFloatingEffects(*dir); err != nil {
		log.Print("Floating effects: ", err)
	}
	app.startProviders()
	port := app.cfg.Settings.Port
	listener, e := net.Listen("tcp4", fmt.Sprintf("127.0.0.1:%d", port))
	if e != nil {
		log.Fatal(e)
	}
	srv := &http.Server{Handler: app.routes(), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 10 * time.Second, WriteTimeout: 70 * time.Second, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 16384}
	go func() { log.Print(srv.Serve(listener)) }()
	go app.servePhoneLAN(port)
	go discovery(port)
	url := fmt.Sprintf("http://127.0.0.1:%d/?admin=%s", port, app.admin)
	if *noUI {
		fmt.Println(url)
	}
	if !*noUI {
		openSettings(url)
	}
	platformLoop(app, url)
}
func discovery(port int) {
	c, e := net.ListenUDP("udp4", &net.UDPAddr{Port: 47641})
	if e != nil {
		app.log("UDP discovery: " + e.Error())
		return
	}
	defer c.Close()
	b := make([]byte, 256)
	for {
		n, addr, e := c.ReadFromUDP(b)
		if e != nil {
			return
		}
		if !app.runtimeConfig().Settings.DisabledSmart && string(b[:n]) == "MACROLEPRECHAUN_DISCOVER_V1" {
			body, _ := json.Marshal(map[string]any{"name": "MacroLeprechaun", "port": port})
			_, _ = c.WriteToUDP(body, addr)
		}
	}
}
