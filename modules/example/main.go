package main

import (
	"encoding/json"
	"fmt"
	"os"
	"time"
)

func main() {
	var req struct {
		API       int               `json:"api"`
		Action    string            `json:"action"`
		Variables map[string]string `json:"variables"`
	}
	if e := json.NewDecoder(os.Stdin).Decode(&req); e != nil {
		fmt.Println(`{"ok":false,"message":"Invalid JSON"}`)
		return
	}
	reply := map[string]any{"ok": false, "message": "Unsupported action or API"}
	if req.API == 1 && req.Action == "clock" {
		reply = map[string]any{"ok": true, "message": "Clock updated", "variables": map[string]string{"clock": time.Now().Format("15:04:05")}}
	}
	json.NewEncoder(os.Stdout).Encode(reply)
}
