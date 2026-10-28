//go:build windows
package main
import("os";"encoding/json";"os/exec";"syscall";_ "embed";"fmt";"path/filepath")
//go:embed notify.ps1
var script string
func main(){var q struct{Op,Block string;Value map[string]any};if e:=json.NewDecoder(os.Stdin).Decode(&q);e!=nil{return};if q.Op!="execute"{json.NewEncoder(os.Stdout).Encode(map[string]any{"ok":true});return};f,e:=os.CreateTemp("","leprechaun-notify-*.ps1");if e==nil{_,e=f.WriteString("\xef\xbb\xbf"+script);f.Close()};if e==nil{payload,_:=json.Marshal(map[string]any{"kind":q.Block,"value":q.Value,"script":f.Name()});cmd:=exec.Command("powershell.exe","-NoProfile","-NonInteractive","-STA","-ExecutionPolicy","Bypass","-File",filepath.Clean(f.Name()));cmd.SysProcAttr=&syscall.SysProcAttr{HideWindow:true};r,w,_:=os.Pipe();cmd.Stdin=r;e=cmd.Start();r.Close();if e==nil{w.Write(payload);w.Close();go cmd.Wait()}else{w.Close();os.Remove(f.Name())}};if e!=nil{json.NewEncoder(os.Stdout).Encode(map[string]any{"ok":false,"error":fmt.Sprint(e)})}else{json.NewEncoder(os.Stdout).Encode(map[string]any{"ok":true})}}
