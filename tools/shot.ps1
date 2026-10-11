param([string]$Out)
Add-Type -AssemblyName System.Drawing
Add-Type @"
using System;
using System.Text;
using System.Runtime.InteropServices;
public class Win {
  public delegate bool EnumProc(IntPtr h, IntPtr l);
  [DllImport("user32.dll")] public static extern bool EnumWindows(EnumProc p, IntPtr l);
  [DllImport("user32.dll")] public static extern uint GetWindowThreadProcessId(IntPtr h, out uint pid);
  [DllImport("user32.dll")] public static extern int GetWindowText(IntPtr h, StringBuilder s, int n);
  [DllImport("user32.dll")] public static extern bool SetForegroundWindow(IntPtr h);
  [DllImport("user32.dll")] public static extern bool GetWindowRect(IntPtr h, out RECT r);
  [DllImport("user32.dll")] public static extern bool ShowWindow(IntPtr h, int c);
  public struct RECT { public int Left, Top, Right, Bottom; }
}
"@

$target = (Get-Process launch -ErrorAction SilentlyContinue | Select-Object -First 1).Id
if (-not $target) { Write-Output "no process"; exit 1 }

$script:hit = [IntPtr]::Zero
$cb = [Win+EnumProc]{
  param($hw, $l)
  $p2 = 0
  [Win]::GetWindowThreadProcessId($hw, [ref]$p2) | Out-Null
  if ($p2 -eq $target) {
    $sb = New-Object System.Text.StringBuilder 256
    [Win]::GetWindowText($hw, $sb, 256) | Out-Null
    if ($sb.ToString() -eq "Flow Launcher") { $script:hit = $hw; return $false }
  }
  return $true
}
[Win]::EnumWindows($cb, [IntPtr]::Zero) | Out-Null

$h = $script:hit
if ($h -eq [IntPtr]::Zero) { Write-Output "no window"; exit 1 }

[Win]::ShowWindow($h, 5) | Out-Null
[Win]::SetForegroundWindow($h) | Out-Null
Start-Sleep -Milliseconds 900

$r = New-Object Win+RECT
[Win]::GetWindowRect($h, [ref]$r) | Out-Null
$w = $r.Right - $r.Left
$hh = $r.Bottom - $r.Top
Write-Output "rect $($r.Left),$($r.Top) ${w}x${hh}"

$bmp = New-Object System.Drawing.Bitmap $w, $hh
$g = [System.Drawing.Graphics]::FromImage($bmp)
$g.CopyFromScreen($r.Left, $r.Top, 0, 0, $bmp.Size)
$bmp.Save($Out, [System.Drawing.Imaging.ImageFormat]::Png)
Write-Output "saved $Out"
