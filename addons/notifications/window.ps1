# Notification UI. $Notice is passed by plugin.ps1, not interpolated as code.
param($Notice)
$ErrorActionPreference = 'Stop'
$q = $Notice
try{
 Add-Type -AssemblyName System.Windows.Forms
 Add-Type -AssemblyName System.Drawing
 $v=$q.value
 if($q.kind -eq 'windows'){
  $icon=New-Object Windows.Forms.NotifyIcon;$icon.Icon=[Drawing.SystemIcons]::Information;$icon.Text='MacroLeprechaun';$icon.Visible=$true
  $icon.ShowBalloonTip(7000,[string]$v.title,[string]$v.text,[Windows.Forms.ToolTipIcon]::Info)
  $timer=New-Object Windows.Forms.Timer;$timer.Interval=8000;$timer.Add_Tick({$icon.Visible=$false;$icon.Dispose();[Windows.Forms.Application]::ExitThread()});$timer.Start();[Windows.Forms.Application]::Run()
 }else{
  Add-Type -TypeDefinition 'public class LeprechaunNotice:System.Windows.Forms.Form{protected override bool ShowWithoutActivation{get{return true;}} protected override System.Windows.Forms.CreateParams CreateParams{get{var p=base.CreateParams;p.ExStyle|=0x08000000;return p;}}}' -ReferencedAssemblies @('System.Windows.Forms.dll','System.Drawing.dll')
  $form=New-Object LeprechaunNotice;$form.FormBorderStyle='None';$form.ShowInTaskbar=$false;$form.TopMost=$true;$form.StartPosition='Manual';$form.Size=New-Object Drawing.Size(380,150)
  $bg='#18251e';$fg='#ffffff';if([string]$v.background -match '^#[0-9a-fA-F]{6}$'){$bg=$v.background};if([string]$v.color -match '^#[0-9a-fA-F]{6}$'){$fg=$v.color};$form.BackColor=[Drawing.ColorTranslator]::FromHtml($bg);$form.ForeColor=[Drawing.ColorTranslator]::FromHtml($fg)
  $title=New-Object Windows.Forms.Label;$title.Text=[string]$v.title;$title.Font=New-Object Drawing.Font('Segoe UI',12,[Drawing.FontStyle]::Bold);$title.Location=New-Object Drawing.Point(18,15);$title.Size=New-Object Drawing.Size(330,30)
  $text=New-Object Windows.Forms.Label;$text.Text=[string]$v.text;$text.Font=New-Object Drawing.Font('Segoe UI',10);$text.Location=New-Object Drawing.Point(18,50);$text.Size=New-Object Drawing.Size(340,85);$form.Controls.Add($title);$form.Controls.Add($text)
  $area=[Windows.Forms.Screen]::FromPoint([Windows.Forms.Cursor]::Position).WorkingArea;$position=[string]$v.position;$x=$area.Right-400;$y=$area.Bottom-170;if($position -like '*left'){$x=$area.Left+20}elseif($position -like '*center'){$x=$area.Left+($area.Width-380)/2};if($position -like 'top*'){$y=$area.Top+20}elseif($position -like 'middle*'){$y=$area.Top+($area.Height-150)/2};$form.Location=New-Object Drawing.Point([int]$x,[int]$y)
  $seconds=5;if($v.timeout){$seconds=[Math]::Max(0.1,[Math]::Min(3600,[double]$v.timeout))};$timer=New-Object Windows.Forms.Timer;$timer.Interval=[int]($seconds*1000);$timer.Add_Tick({$form.Close()});$form.Add_Click({$form.Close()});$timer.Start();[Windows.Forms.Application]::Run($form)
 }
 } finally {
    if ($timer) { $timer.Stop(); $timer.Dispose() }
    if ($icon) { $icon.Visible = $false; $icon.Dispose() }
    if ($form) { $form.Dispose() }
}
