<#
.SYNOPSIS
    Verifies the llama.cpp static archives do not need a CPU newer than the
    project's declared CPU baseline.

.DESCRIPTION
    The Windows archives are committed to the repo and linked into every shipped
    binary. If they are built on a modern machine with GGML_NATIVE left at its
    default ON, that adds -march=native, which records AVX-VNNI (vpdpbusd) kernels
    on Intel Alder Lake and newer. Those instructions fault with
    STATUS_ILLEGAL_INSTRUCTION (0xc000001d) on any older CPU, which killed the app
    the instant an index run started embedding on other people's PCs.

    -Baseline is the lowest CPU the archives are allowed to need; anything above
    it fails the check. Detection is structural, not symbol-based: an "avx512"
    symbol in an archive is only the ggml_cpu_has_avx512* probe and says nothing
    about the code, so the check reads the disassembly instead.

    Keep -Baseline in step with -Baseline in
    scripts/build-llamago-archives-windows.ps1.

    Run with no arguments to check the committed Windows archives, or pass
    -ArchiveDir to check a staging directory before it is installed.

.EXAMPLE
    powershell -NoProfile -ExecutionPolicy Bypass -File scripts/check-llamago-archives-portable.ps1
#>
[CmdletBinding()]
param(
    # Directory containing lib*.a. Defaults to the committed Windows archives.
    [string]$ArchiveDir = '',

    # Lowest CPU the archives may require: sse2, sse42 or avx2.
    # avx2 is the shipped default (see build-llamago-archives-windows.ps1).
    # There is no "avx" level: AVX already has 256-bit %ymm for float ops, so
    # objdump cannot separate AVX from AVX2 here, and an unverifiable level is
    # worse than none.
    [ValidateSet('sse2', 'sse42', 'avx2')]
    [string]$Baseline = 'avx2',

    # Path to objdump. Auto-detected from PATH and the WinLibs MinGW install.
    [string]$Objdump = ''
)

$ErrorActionPreference = 'Stop'

if (-not $ArchiveDir) {
    $ArchiveDir = Join-Path $PSScriptRoot '..\third_party\llama-go\windows\amd64'
}
$ArchiveDir = [System.IO.Path]::GetFullPath($ArchiveDir)
if (-not (Test-Path $ArchiveDir)) { throw "Archive directory not found: $ArchiveDir" }

$archives = @(Get-ChildItem $ArchiveDir -Filter *.a -File -ErrorAction SilentlyContinue)
if ($archives.Count -eq 0) { throw "No .a archives found in $ArchiveDir" }

if (-not $Objdump) {
    $cmd = Get-Command objdump.exe -ErrorAction SilentlyContinue
    if ($cmd) { $Objdump = $cmd.Source }
}
if (-not $Objdump) {
    $roots = @(
        (Join-Path $env:ProgramData 'mingw64'),
        (Join-Path $env:LOCALAPPDATA 'Microsoft\WinGet\Packages')
    ) | Where-Object { Test-Path $_ }
    foreach ($root in $roots) {
        $hit = Get-ChildItem $root -Recurse -Filter objdump.exe -File -ErrorAction SilentlyContinue |
            Select-Object -First 1
        if ($hit) { $Objdump = $hit.FullName; break }
    }
}
if (-not $Objdump) { throw 'objdump.exe not found; cannot verify archive portability.' }
Write-Host "objdump: $Objdump"

# What is above the chosen baseline. The SSE-only baselines cannot use any VEX
# (c4/c5) or EVEX (62) encoded instruction, and in 64-bit machine code those
# opcodes are never anything else. This also catches VEX-encoded BMI (andn, mulx,
# shlx), whose mnemonics do not start with "v".
#
# The trailing part of the pattern matters: objdump wraps a long instruction's
# bytes onto extra lines that start with an address and bytes but no mnemonic, so
# matching the first byte alone reports data as code (a `movabs $0x736f622d...,%rax`
# continues with "62 6f 73" on the next line). Requiring the remaining byte column
# to be followed by a mnemonic token restricts hits to real instructions.
if ($Baseline -eq 'sse2' -or $Baseline -eq 'sse42') {
    $forbidden = [ordered]@{ 'VEX/EVEX encoded instruction (needs AVX or newer)' = '^\s*[0-9a-f]+:\s+(c4|c5|62)\s(?:\s*[0-9a-f]{2})*\s+[a-zA-Z(]' }
    $anchor = '%xmm'
}
else {
    $forbidden = [ordered]@{
        'AVX-512 instruction'  = '%zmm|\{k[0-7]\}'
        'AVX-VNNI instruction' = 'vpdpbusd'
    }
    $anchor = '%ymm'
}

$offenders = @()
$cpuVerified = $false
$tmp = Join-Path ([System.IO.Path]::GetTempPath()) ("vectile-archcheck-" + [guid]::NewGuid().ToString('N'))
New-Item -ItemType Directory -Force -Path $tmp | Out-Null

try {
    foreach ($f in $archives) {
        $dis = Join-Path $tmp ($f.BaseName + '.txt')
        # objdump writes the disassembly to stdout; stderr is merged only because
        # a failure there would otherwise pass silently.
        $prev = $ErrorActionPreference
        $ErrorActionPreference = 'Continue'
        try {
            & $Objdump -d $f.FullName 2>&1 | Out-File -FilePath $dis -Encoding ascii
        }
        finally { $ErrorActionPreference = $prev }

        if ($LASTEXITCODE -ne 0) {
            throw "objdump could not read $($f.Name) (exit $LASTEXITCODE); the check cannot be trusted."
        }

        # A vacuous pass (objdump silently produced nothing) is worse than no check.
        $instr = (Select-String -Path $dis -Pattern '^\s*[0-9a-f]+:\s' | Measure-Object).Count
        if ($instr -eq 0 -and $f.Length -gt 100KB) {
            throw "objdump found no instructions in $($f.Name); the check cannot be trusted."
        }

        foreach ($label in $forbidden.Keys) {
            $bad = Select-String -Path $dis -Pattern $forbidden[$label] |
                Select-Object -First 3 -ExpandProperty Line
            if ($bad) { $offenders += "$($f.Name): $label -> $($bad -join ' | ')" }
        }

        # ggml-cpu holds the kernels. The anchor has to be present for the
        # baseline, which is what proves the scan is really reading the code and
        # not silently passing on an empty file.
        if ($f.Name -eq 'libggml-cpu.a') {
            $anchorHits = (Select-String -Path $dis -Pattern $anchor -SimpleMatch | Measure-Object).Count
            if ($anchorHits -eq 0) {
                throw "libggml-cpu.a contains no $anchor code, so the disassembly scan is not reading the kernels."
            }
            Write-Host ("  libggml-cpu.a: {0:N0} instruction lines, {1:N0} {2} uses" -f $instr, $anchorHits, $anchor)
            $cpuVerified = $true
        }
    }
}
finally {
    Remove-Item $tmp -Recurse -Force -ErrorAction SilentlyContinue
}

if (-not $cpuVerified) {
    throw 'libggml-cpu.a was not found in the archive set; nothing was verified.'
}
if ($offenders.Count -gt 0) {
    $detail = $offenders -join "`n  "
    throw "Archives need a CPU newer than the '$Baseline' baseline. Rebuild them with scripts/build-llamago-archives-windows.ps1 -Baseline $Baseline.`n  $detail"
}

Write-Host "OK: $($archives.Count) archives at the '$Baseline' baseline."
