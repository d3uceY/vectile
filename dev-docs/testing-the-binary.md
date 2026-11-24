# Testing the real binary (for agents)

How to build vectile, run the real desktop window, click through it, and read what it shows, without touching the developer's own app or library. Use this when a bug only shows up in the built app (timing between backend events and the UI, WebView2 behaviour, the real model) and the browser dev-stub cannot reproduce it.

This was worked out on Windows while fixing the cancel flow. It is the only path that exercises the real Go backend, the real model and the real window together.

## Which tool for which question

| Question | Use |
| --- | --- |
| Does a backend rule hold? | `go test ./backend/...`. Real-model tests skip when the model file is missing. |
| Does a UI layout or style look right? | The dev-stub (`node scripts/dev-stub.mjs`, port 9255) in a browser page. |
| Do backend events, timing and UI agree? | The real binary, as described here. |

The dev-stub answers every backend call from `scripts/vectile-stub.mjs` and fakes index runs. It cannot show a race between a real event and a real call, which is exactly the kind of bug the binary catches. Example: with an instant cancel, `indexing:cancelled` arrived before `CancelIndexing` returned, and the UI stuck on "Cancelling…". The stub did not show it. The binary did.

## Do not use the developer's instance

`wails3 task run` (the dev watcher) usually has `bin/vectile.exe` running. It holds the single-instance mutex and the real library in `%APPDATA%\vectile`. Check first:

```powershell
Get-CimInstance Win32_Process -Filter "Name='vectile.exe'" | Select ProcessId, CommandLine, ParentProcessId
```

A parent of `wails3 task run` means it is theirs. Do not kill it, do not drive it, and never click Re-index or Delete in it. Build a separate copy with its own single-instance id and its own data folder.

## 1. Prepare an isolated data folder

Everything the app stores lives under `<UserConfigDir>/vectile`. On Windows that is `%APPDATA%\vectile`, so pointing `APPDATA` at a temp folder for one process gives it a fresh library.

Build this layout under `%TEMP%\vectile-e2e`:

```
docs/                      test files to index (see below)
appdata/vectile/config.json
appdata/vectile/models/bge-m3-Q4_K_M.gguf   copied from the real %APPDATA%\vectile\models
appdata/vectile/db/
```

`config.json` can be partial, it loads on top of the defaults:

```json
{
  "projects": { "e2e": ["C:\\Users\\<you>\\AppData\\Local\\Temp\\vectile-e2e\\docs"] },
  "chunk_size_tokens": 200,
  "chunk_overlap_tokens": 20,
  "active_model": "C:\\Users\\<you>\\AppData\\Local\\Temp\\vectile-e2e\\appdata\\vectile\\models\\bge-m3-Q4_K_M.gguf",
  "mcp": { "enabled": false, "port": 31999, "transport": "streamable-http" },
  "gui": { "auto_reindex": false }
}
```

- `active_model` must be the full path, or Index stays disabled behind "Choose a model".
- `mcp.enabled: false` keeps it off port 31123, which the developer's app owns.
- Write JSON without a BOM (`[IO.File]::WriteAllText($p, $t, (New-Object System.Text.UTF8Encoding $false))`). A BOM makes the app fall back to defaults silently.
- Make the test files big enough that a run lasts tens of seconds. 16 markdown files of 30 to 40 sections of about 12 paragraphs each took roughly 8 seconds per full pass with bge-m3 on CPU, which leaves a comfortable window to cancel in. Small files finish before a script can react.

Use a different `chunk_size_tokens` from the default only if you need more chunks per file.

## 2. Build a test copy with its own single-instance id

`main.go` hard-codes `UniqueID: "com.d3ucey.vectile"`. A second copy with that id hands off to the running one and exits silently, which looks like a startup crash. Do not edit `main.go` (the dev watcher would rebuild and restart the developer's app). Use a Go build overlay, which swaps the file for one build only.

```powershell
# from the repo root
$main = [IO.File]::ReadAllText((Resolve-Path main.go).Path).Replace('"com.d3ucey.vectile"', '"com.d3ucey.vectile.e2e"')
[IO.File]::WriteAllText("$env:TEMP\vectile-e2e\main_e2e.go", $main)
$overlay = @{ Replace = @{ (Resolve-Path main.go).Path = "$env:TEMP\vectile-e2e\main_e2e.go" } } | ConvertTo-Json
[IO.File]::WriteAllText("$env:TEMP\vectile-e2e\overlay.json", $overlay)

Push-Location frontend; npm run build; Pop-Location        # dist is embedded into the exe

$env:CGO_ENABLED = '1'
$r = (Get-Location).Path
$env:LIBRARY_PATH   = "$r/third_party/llama-go/windows/amd64"
$env:C_INCLUDE_PATH = "$r/third_party/llama-go"
go build -overlay "$env:TEMP\vectile-e2e\overlay.json" -tags production -buildvcs=false -o "$env:TEMP\vectile-e2e\vectile-e2e.exe" .
```

Rebuild the frontend (`npm run build`) after every UI change, or the exe runs the old bundle. `npm run build` can leave a line-ending-only change in `frontend/bindings/events/index.ts`; `git checkout -- frontend/bindings/events/index.ts` removes it.

The exe name matters: the helper script below finds the window by process name `vectile-e2e`.

## 3. Launch it with the isolated data

Set `APPDATA` only around `Start-Process`, then put it back, so the rest of the shell keeps the real one:

```powershell
$env:APPDATA = "$env:TEMP\vectile-e2e\appdata"
$p = Start-Process -FilePath "$env:TEMP\vectile-e2e\vectile-e2e.exe" -PassThru
$env:APPDATA = [Environment]::GetFolderPath('ApplicationData')
Start-Sleep 8
```

Its log is `%TEMP%\vectile-e2e\appdata\vectile\vectile.log`. The first line names the data dir, which proves it is not using the real one. Read it for `level=ERROR` and `level=WARN` after a scenario.

A fresh data folder shows first-run overlays: an update dialog, an OCR dialog, the model dialog and the setup tour. Click `Not now` about four times, then `Skip tour`. The overlays block every other click until they are gone.

## 4. Drive it with UI Automation

WebView2 remote debugging does not work here. Wails v3 beta.6 ignores `WEBVIEW2_ADDITIONAL_BROWSER_ARGUMENTS`, and the per-user policy key under `HKCU\Software\Policies\Microsoft\Edge\WebView2` is access-denied. Do not try to elevate. Instead use .NET UI Automation, which WebView2 exposes: every button, switch and heading in the page appears with its accessible name, and `Invoke` clicks it.

Save this as `%TEMP%\vectile-e2e-ui.ps1` (keep it ASCII-only for Windows PowerShell 5.1) and dot-source it:

```powershell
Add-Type -AssemblyName UIAutomationClient
Add-Type -AssemblyName UIAutomationTypes
Add-Type -AssemblyName System.Drawing

function Get-Win {
    $p = Get-Process -Name vectile-e2e -ErrorAction SilentlyContinue |
        Where-Object { $_.MainWindowHandle -ne 0 } | Select-Object -First 1
    if (-not $p) { return $null }
    [System.Windows.Automation.AutomationElement]::FromHandle($p.MainWindowHandle)
}

function Find-All([string]$name, $type = $null) {
    $w = Get-Win; if (-not $w) { return @() }
    $cond = New-Object System.Windows.Automation.PropertyCondition(
        [System.Windows.Automation.AutomationElement]::NameProperty, $name)
    $out = @()
    foreach ($e in $w.FindAll([System.Windows.Automation.TreeScope]::Descendants, $cond)) {
        if ($type -and $e.Current.ControlType.ProgrammaticName -ne $type) { continue }
        $out += $e
    }
    $out
}

function Click-Name([string]$name, [int]$which = 0) {
    $els = @(Find-All $name 'ControlType.Button')
    if ($els.Count -le $which) { return $false }
    $pat = $null
    if ($els[$which].TryGetCurrentPattern([System.Windows.Automation.InvokePattern]::Pattern, [ref]$pat)) {
        $pat.Invoke(); return $true
    }
    $false
}

function Wait-Name([string]$name, [int]$timeoutSec = 30) {
    $end = (Get-Date).AddSeconds($timeoutSec)
    while ((Get-Date) -lt $end) {
        if (@(Find-All $name 'ControlType.Button').Count -gt 0) { return $true }
        Start-Sleep -Milliseconds 150
    }
    $false
}

function Button-Names {
    $w = Get-Win; if (-not $w) { return @() }
    $cond = New-Object System.Windows.Automation.PropertyCondition(
        [System.Windows.Automation.AutomationElement]::ControlTypeProperty,
        [System.Windows.Automation.ControlType]::Button)
    foreach ($e in $w.FindAll([System.Windows.Automation.TreeScope]::Descendants, $cond)) {
        if ($e.Current.Name) { "{0}{1}" -f $e.Current.Name, $(if ($e.Current.IsEnabled) { "" } else { " [disabled]" }) }
    }
}

function Shot([string]$path) {
    $p = Get-Process -Name vectile-e2e | Where-Object { $_.MainWindowHandle -ne 0 } | Select-Object -First 1
    Add-Type -TypeDefinition 'using System;using System.Runtime.InteropServices;public class W{[DllImport("user32.dll")]public static extern bool GetWindowRect(IntPtr h,out RECT r);[DllImport("user32.dll")]public static extern bool PrintWindow(IntPtr h,IntPtr dc,uint f);[StructLayout(LayoutKind.Sequential)]public struct RECT{public int L,T,R,B;}}' -ErrorAction SilentlyContinue
    $r = New-Object W+RECT
    [W]::GetWindowRect($p.MainWindowHandle, [ref]$r) | Out-Null
    $bmp = New-Object System.Drawing.Bitmap ($r.R - $r.L), ($r.B - $r.T)
    $g = [System.Drawing.Graphics]::FromImage($bmp)
    $dc = $g.GetHdc(); [W]::PrintWindow($p.MainWindowHandle, $dc, 2) | Out-Null; $g.ReleaseHdc($dc)
    $bmp.Save($path, [System.Drawing.Imaging.ImageFormat]::Png)
    $g.Dispose(); $bmp.Dispose()
}
```

How to use it:

- `Button-Names` is the cheapest way to read state. It lists every button with `[disabled]` where it applies, so "is Index disabled", "does the bar say Cancel or Cancelling…" are one call.
- `Click-Name 'Index'` clicks the first button with that name. Several controls share names (the header and each card both have "Re-index all"), so pass an index: `Click-Name 'Re-index all' 1` is the first card's. The header one is 0.
- `Wait-Name` only checks that a button exists, not that it is enabled. To wait for a run to finish, poll until `Cancel` disappears from `Button-Names`.
- `Shot` writes a PNG of the window, then open it with an image viewer. Use it to confirm what the accessibility tree says, not instead of it.
- Only elements the app exposes are visible. A button inside an element with `role="progressbar"` is hidden from the tree (ARIA treats progressbar children as presentational), which is why `ProgressBar` puts the role on the bar only. If a control you can see is missing from `Button-Names`, suspect its ARIA role before suspecting the script.

Example, the cancel flow:

```powershell
. "$env:TEMP\vectile-e2e-ui.ps1"
Click-Name 'Index' | Out-Null; Start-Sleep 1.5
Click-Name 'Re-index all' 1 | Out-Null       # the card's button
Wait-Name 'Cancel' 30 | Out-Null; Start-Sleep 3
Click-Name 'Cancel' | Out-Null; Start-Sleep -Milliseconds 400
Click-Name 'Cancel run' | Out-Null; Start-Sleep 2   # confirm dialog
Click-Name 'Re-index all' 1 | Out-Null; Start-Sleep 2.5
Button-Names | Where-Object { $_ -match 'Cancel|Index|Re-index' }
```

## 5. Reproduce first, then fix, then verify

A fix verified only on the fixed build proves little. When chasing a UI bug:

1. Build and run the binary as above and reproduce the bug.
2. Make the fix, rebuild the frontend and the exe, and run the same script again.
3. If you are unsure the fix is what changed the result, disable it temporarily, rebuild, and confirm the bug returns. Restore it and rebuild once more so the binary you finish with matches the source.

Stop the process before rebuilding (`Get-Process vectile-e2e | Stop-Process -Force`); a running exe cannot be overwritten.

## 6. Clean up

```powershell
Get-Process vectile-e2e -ErrorAction SilentlyContinue | Stop-Process -Force
Remove-Item "$env:TEMP\vectile-e2e" -Recurse -Force      # includes the 437 MB model copy
```

Only ever stop `vectile-e2e`. The developer's `vectile` process is not yours.

## Gotchas

- **Silent exit at launch.** A second copy with the shared `UniqueID` exits with code 0 and no window. Use the overlay build. A stale `vectile.exe` from an older portable build holding port 31123 can also answer MCP probes; check `Get-Process vectile`.
- **Index stays disabled.** No active model. Set `active_model` in `config.json` before launch. The model dialog in the first-run overlays does not activate the copied file.
- **Everything is clicked but nothing happens.** A first-run overlay is still up. Dismiss it, then look at `Button-Names`.
- **The run is over before the script reacts.** The test corpus is too small. Add files.
- **Stale UI.** The exe embeds `frontend/dist`. Run `npm run build` in `frontend/` before `go build`.
- **PowerShell traps.** Do not pipe `go test`; use ASCII-only `.ps1` files; write JSON without a BOM; `Set-Content` on a CRLF file converts it to LF; read Go and other UTF-8 sources with `[IO.File]::ReadAllText`, because `Get-Content` in Windows PowerShell 5.1 decodes them as ANSI and mangles non-ASCII characters.
- **Real-model tests as an alternative.** For backend behaviour you rarely need the window at all. `backend/services/index_cancel_test.go` drives `IndexService` with the real model, a real database and a real Wails event bus (`application.New(...)` works in a test with no window), and finishes in seconds. Prefer that when the question is about events and state, and keep the binary for what only the window can show.
