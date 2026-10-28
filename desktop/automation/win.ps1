$ErrorActionPreference='Stop'
[Console]::InputEncoding=New-Object System.Text.UTF8Encoding($false)
[Console]::OutputEncoding=New-Object System.Text.UTF8Encoding($false)
function Terminals {
 $list=New-Object System.Collections.Generic.List[object]
 foreach($entry in @(@('powershell','PowerShell','powershell.exe'),@('pwsh','PowerShell 7','pwsh.exe'),@('cmd','CMD','cmd.exe'),@('python','Python','python.exe'),@('py','Python (py)','py.exe'))){
  $found=Get-Command $entry[2] -ErrorAction SilentlyContinue
  if($found -and $found.Source -notlike '*\Microsoft\WindowsApps\*'){$list.Add(@{id=$entry[0];name=$entry[1];path=$found.Source})}
 }
 $wsl=Get-Command wsl.exe -ErrorAction SilentlyContinue
 if($wsl){$distros=(& $wsl.Source --list --quiet 2>$null)-replace "`0",'';if($LASTEXITCODE -eq 0){foreach($d in $distros){$name=$d.Trim();if($name){$list.Add(@{id='wsl:'+ $name;name='WSL - '+$name;path=$wsl.Source})}}}}
 return $list.ToArray()
}
function Programs {
 $list=New-Object System.Collections.Generic.List[object]
 $seen=@{}
 foreach($root in @('HKLM:\SOFTWARE\Microsoft\Windows\CurrentVersion\App Paths','HKLM:\SOFTWARE\WOW6432Node\Microsoft\Windows\CurrentVersion\App Paths','HKCU:\SOFTWARE\Microsoft\Windows\CurrentVersion\App Paths')){
  foreach($key in @(Get-ChildItem $root -ErrorAction SilentlyContinue)){$path=[string]$key.GetValue('');$path=[Environment]::ExpandEnvironmentVariables($path.Trim('"'));if($path -and (Test-Path -LiteralPath $path) -and -not $seen.ContainsKey($path)){$seen[$path]=$true;$list.Add(@{name=[IO.Path]::GetFileNameWithoutExtension($key.PSChildName);path=$path})}}
 }
 $shell=New-Object -ComObject WScript.Shell
 foreach($root in @([Environment]::GetFolderPath('StartMenu'),[Environment]::GetFolderPath('CommonStartMenu'))){foreach($file in @(Get-ChildItem -LiteralPath $root -Filter *.lnk -Recurse -ErrorAction SilentlyContinue)){$shortcut=$shell.CreateShortcut($file.FullName);$path=$shortcut.TargetPath;if($path -and ([IO.Path]::GetExtension($path) -ieq '.exe') -and (Test-Path -LiteralPath $path) -and -not $seen.ContainsKey($path)){$seen[$path]=$true;$list.Add(@{name=$file.BaseName;path=$path})};[void][Runtime.InteropServices.Marshal]::ReleaseComObject($shortcut)}}
 [void][Runtime.InteropServices.Marshal]::ReleaseComObject($shell)
 if(Get-Command Get-StartApps -ErrorAction SilentlyContinue){foreach($a in @(Get-StartApps -ErrorAction SilentlyContinue)){if($a.AppID -like '*!*'){$list.Add(@{name=$a.Name;path='shell:AppsFolder\'+$a.AppID})}}}
 return @($list.ToArray() | Sort-Object name)
}
function LoadAudio {if(-not ('Leprechaun.Audio' -as [type])){Add-Type -TypeDefinition @'
__AUDIO_SOURCE__
'@}}
try {
 $request=[Console]::In.ReadToEnd()|ConvertFrom-Json
 $q=$request.data
 $value=$null
 switch($request.action){
 'network' {$names=@(Get-NetConnectionProfile -ErrorAction SilentlyContinue|Select-Object -ExpandProperty Name -Unique);$value=$names -join ', '}
 'inventory' {
  $devices=@();$audioError='';try{LoadAudio;$devices=@([Leprechaun.Audio]::List())}catch{$audioError=$_.Exception.Message}
  $value=@{terminals=@(Terminals);apps=@(Programs);devices=$devices;audioError=$audioError;platform='windows'}
 }
 'pick' {
  Add-Type -AssemblyName System.Windows.Forms
  $owner=New-Object Windows.Forms.Form;$owner.TopMost=$true;$owner.ShowInTaskbar=$false;$owner.StartPosition='CenterScreen'
  try{
   if($q.kind -eq 'folder'){$dialog=New-Object Windows.Forms.FolderBrowserDialog;$dialog.Description='MacroLeprechaun';if($dialog.ShowDialog($owner)-eq 'OK'){$value=$dialog.SelectedPath}else{$value=''}}
   else{if($q.kind -eq 'output'){$dialog=New-Object Windows.Forms.SaveFileDialog;$dialog.Filter='Text (*.txt)|*.txt|Log (*.log)|*.log|All files (*.*)|*.*';$dialog.OverwritePrompt=$true}else{$dialog=New-Object Windows.Forms.OpenFileDialog;$dialog.CheckFileExists=$true;$dialog.Filter=if($q.kind-eq 'program'){'Programs (*.exe)|*.exe|All files (*.*)|*.*'}else{'All files (*.*)|*.*'}};if($dialog.ShowDialog($owner)-eq 'OK'){$value=$dialog.FileName}else{$value=''}}
   $dialog.Dispose()
  }finally{$owner.Dispose()}
 }
 'command' {
  $terminal=@(Terminals | Where-Object {$_.id -ceq $q.terminal})|Select-Object -First 1
  if(-not $terminal){throw 'Selected terminal is no longer installed'}
  $info=New-Object Diagnostics.ProcessStartInfo;$info.FileName=$terminal.path;$info.UseShellExecute=$false;$info.CreateNoWindow=$true
  if($q.directory){if(-not (Test-Path -LiteralPath $q.directory -PathType Container)){throw 'Working directory does not exist'};$info.WorkingDirectory=$q.directory}
  $script="[Console]::OutputEncoding = New-Object Text.UTF8Encoding(`$false); & {`n"+[string]$q.command+"`n}"
  $bytes=[Text.Encoding]::Unicode.GetBytes($script)
  switch -Wildcard ($q.terminal){
   'powershell' {$info.Arguments='-NoProfile -NonInteractive -OutputFormat Text -EncodedCommand '+[Convert]::ToBase64String($bytes)}
   'pwsh' {$info.Arguments='-NoProfile -NonInteractive -OutputFormat Text -EncodedCommand '+[Convert]::ToBase64String($bytes)}
   'cmd' {$info.Arguments='/d /s /c "chcp 65001>nul & '+$q.command+'"'}
   {$_ -eq 'python' -or $_ -eq 'py'} {$code=[Convert]::ToBase64String([Text.Encoding]::UTF8.GetBytes([string]$q.command));$info.Arguments='-X utf8 -c "import base64;exec(compile(base64.b64decode('''+$code+'''),''<macro>'',''exec''))"'}
   'wsl:*' {$script=[Convert]::ToBase64String([Text.Encoding]::UTF8.GetBytes([string]$q.command));$distro=$q.terminal.Substring(4);if($distro -match '["\r\n]'){throw 'Invalid WSL distro'};$info.Arguments='-d "'+$distro+'" -- sh -c "echo '+$script+' | base64 -d | sh"'}
   default {throw 'Unknown terminal'}
  }
  $info.RedirectStandardOutput=$true;$info.RedirectStandardError=$true;$info.StandardOutputEncoding=New-Object Text.UTF8Encoding($false);$info.StandardErrorEncoding=New-Object Text.UTF8Encoding($false)
  $process=New-Object Diagnostics.Process;$process.StartInfo=$info
  try{[void]$process.Start();$stdout=$process.StandardOutput.ReadToEndAsync();$stderr=$process.StandardError.ReadToEndAsync();if(-not $process.WaitForExit(60000)){try{Stop-Process -Id $process.Id -Force -ErrorAction SilentlyContinue}catch{};throw 'Command timed out after 60 seconds'}
   $text=$stdout.Result;if($stderr.Result){$text+=[Environment]::NewLine+$stderr.Result}
   if($q.output){[IO.File]::WriteAllText([string]$q.output,$text,(New-Object Text.UTF8Encoding($false)))}
   if($process.ExitCode -ne 0){throw ('Command exit code '+$process.ExitCode)}
   $value=@{exitCode=$process.ExitCode}
  }finally{$process.Dispose()}
 }
 'file' {if(-not (Test-Path -LiteralPath $q.path -PathType Leaf)){throw 'File not found'};if($q.app){if($q.app -like 'shell:AppsFolder\*'){throw 'Packaged apps cannot be passed an arbitrary file; choose an executable or Windows default'};Start-Process -FilePath ([string]$q.app) -ArgumentList ('"'+$q.path+'"')|Out-Null}else{Start-Process -FilePath ([string]$q.path)|Out-Null}}
 'folder' {if(-not (Test-Path -LiteralPath $q.path -PathType Container)){throw 'Folder not found'};Start-Process -FilePath explorer.exe -ArgumentList ('"'+$q.path+'"')|Out-Null}
 'program' {if($q.path -like 'shell:AppsFolder\*'){if($q.args){throw 'Packaged apps do not accept command line arguments here'};Start-Process explorer.exe -ArgumentList ([string]$q.path)|Out-Null}else{if($q.args){Start-Process -FilePath ([string]$q.path) -ArgumentList ([string]$q.args)|Out-Null}else{Start-Process -FilePath ([string]$q.path)|Out-Null}}}
 'url' {Start-Process -FilePath ([string]$q.url)|Out-Null}
 'audio' {LoadAudio;$capture=([string]$q.action).StartsWith('mic_');$amount=if(([string]$q.action).EndsWith('set')){[double]$q.value}else{[double]$q.steps};[Leprechaun.Audio]::Apply([string]$q.device,$capture,[string]$q.action,$amount)}
 default {throw 'Unknown Windows operation'}
 }
 [Console]::WriteLine((@{ok=$true;value=$value}|ConvertTo-Json -Depth 8 -Compress))
}catch{[Console]::WriteLine((@{ok=$false;error=$_.Exception.Message}|ConvertTo-Json -Compress));exit 1}
