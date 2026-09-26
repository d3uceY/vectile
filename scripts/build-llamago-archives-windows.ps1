<#
.SYNOPSIS
    Builds the llama.cpp static archives for Windows/amd64 (x86-64) and installs
    them into third_party/llama-go/windows/amd64/.

.DESCRIPTION
    Windows twin of scripts/build-llamago-archives.sh. The vendored llama-go
    ships only the resulting .a files plus headers, so the llama.cpp SOURCE has
    to be fetched and compiled here.

    CPU PORTABILITY IS THE WHOLE POINT OF THIS SCRIPT.
    llama.cpp's GGML_NATIVE defaults to ON, which adds -march=native. On a build
    machine that supports AVX-VNNI (Intel Alder Lake / 12th gen and newer) that
    compiles VPDPBUSD into the quantized dot-product kernels, which then fault
    with STATUS_ILLEGAL_INSTRUCTION on every older CPU. That shipped once and
    made the app crash the moment an index run started on other people's PCs.
    Do NOT remove -DGGML_NATIVE=OFF.

    With GGML_NATIVE=OFF llama.cpp explicitly enables SSE4.2/AVX/AVX2/FMA/F16C/
    BMI2 (a Haswell-2013 floor) and never AVX-VNNI or AVX-512. The script
    verifies the result with objdump and refuses to install a non-portable
    archive. This matches the Linux/macOS archives built by
    scripts/build-llamago-archives.sh.

    MinGW GCC must be 16.1.0 (WinLibs UCRT/posix/seh): the libstdc++ ABI has to
    match the GCC 16.1.0 toolchain the release workflow pins with
    `choco install mingw --version=16.1.0`.

.PARAMETER LlamaGoRef
    llama-go commit/tag to build from. Omit to auto-detect: the script walks HEAD,
    then tags, then every commit that moved the llama.cpp submodule, and picks the
    first ref whose wrapper.h, wrapper.cpp, llama.cpp/include/llama.h and
    llama.cpp/ggml/include/ggml.h are all identical to the vendored copies. That
    set is the ABI contract with the Go bindings, so a ref that only matches
    wrapper.h is not good enough.

.PARAMETER WorkDir
    Where to clone/build. Defaults to a directory under $env:TEMP. Left in place
    with -Keep so a failed build can be inspected.

.PARAMETER Keep
    Do not delete the work directory when the script finishes.

.EXAMPLE
    powershell -NoProfile -ExecutionPolicy Bypass -File scripts/build-llamago-archives-windows.ps1
#>
[CmdletBinding()]
param(
    [string]$LlamaGoRef = '',
    [string]$WorkDir = '',
    [int]$Jobs = 0,
    [string]$InstallDir = '',
    [switch]$Keep
)

$ErrorActionPreference = 'Stop'
function Step([string]$msg) { Write-Host "`n==> $msg" -ForegroundColor Cyan }
function Fail([string]$msg) { throw $msg }

# Windows PowerShell 5.1 turns a native command's stderr into a terminating
# NativeCommandError whenever it is merged with 2>&1 and $ErrorActionPreference
# is 'Stop'. git writes to stderr even on success ("HEAD is now at ..."), so
# every 2>&1 call has to run with the preference relaxed.
function Invoke-Lenient([scriptblock]$Block) {
    $prev = $ErrorActionPreference
    $ErrorActionPreference = 'Continue'
    try { & $Block } finally { $ErrorActionPreference = $prev }
}

# SHA-256 of a file's bytes with CRLF normalised to LF, so a checkout with
# different line endings still compares equal. Null when the file is missing.
function Get-NormalizedHash([string]$Path) {
    if (-not (Test-Path $Path)) { return $null }
    $bytes = [System.IO.File]::ReadAllBytes($Path)
    $ms = New-Object System.IO.MemoryStream
    for ($i = 0; $i -lt $bytes.Length; $i++) {
        if ($bytes[$i] -eq 13 -and ($i + 1) -lt $bytes.Length -and $bytes[$i + 1] -eq 10) { continue }
        $ms.WriteByte($bytes[$i])
    }
    $sha = [System.Security.Cryptography.SHA256]::Create()
    try { $hash = $sha.ComputeHash($ms.ToArray()) } finally { $sha.Dispose(); $ms.Dispose() }
    return ([BitConverter]::ToString($hash) -replace '-', '')
}

if (-not $InstallDir) { $InstallDir = Join-Path $PSScriptRoot '..\third_party\llama-go\windows\amd64' }
$InstallDir = [System.IO.Path]::GetFullPath($InstallDir)

$RepoRoot = [System.IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..'))
$Vendored = Join-Path $RepoRoot 'third_party\llama-go'
$VendoredWrapperH = Join-Path $Vendored 'wrapper.h'
if (-not (Test-Path $VendoredWrapperH)) { Fail "Vendored wrapper.h not found at $VendoredWrapperH" }

# ---- 0. Toolchain ----------------------------------------------------------
# Prefer whatever is on PATH, then fall back to the WinLibs winget install
# (WinLibs UCRT/posix/seh is what the archives and CI are built with).
function Resolve-MinGWBin {
    $gcc = Get-Command gcc.exe -ErrorAction SilentlyContinue
    if ($gcc) { return (Split-Path $gcc.Source -Parent) }
    $guess = Join-Path $env:LOCALAPPDATA 'Microsoft\WinGet\Packages'
    if (Test-Path $guess) {
        $hit = Get-ChildItem $guess -Recurse -Filter gcc.exe -ErrorAction SilentlyContinue |
            Where-Object { $_.FullName -match 'WinLibs' } | Select-Object -First 1
        if ($hit) { return $hit.DirectoryName }
    }
    Fail "gcc.exe not found. Install MinGW-w64 WinLibs:`n  winget install --id BrechtSanders.WinLibs.POSIX.UCRT -e --accept-source-agreements --accept-package-agreements --disable-interactivity"
}

$MinGWBin = Resolve-MinGWBin
$env:Path = "$MinGWBin;$env:Path"

foreach ($tool in 'gcc', 'g++', 'ar', 'cmake', 'git') {
    if (-not (Get-Command $tool -ErrorAction SilentlyContinue)) { Fail "'$tool' is required but was not found on PATH." }
}

$gccVersion = (& gcc --version | Select-Object -First 1)
if ($gccVersion -notmatch '16\.1\.0') {
    Write-Warning "MinGW GCC is '$gccVersion'. The archives were built with GCC 16.1.0 and CI pins that version; a different GCC can break the libstdc++ ABI against libllama-common.a."
}

if ($Jobs -le 0) { $Jobs = [Environment]::ProcessorCount }

if (-not $WorkDir) { $WorkDir = Join-Path $env:TEMP 'vectile-llamago-windows' }
$WorkDir = [System.IO.Path]::GetFullPath($WorkDir)
if (Test-Path $WorkDir) { Remove-Item $WorkDir -Recurse -Force }
New-Item -ItemType Directory -Force -Path $WorkDir | Out-Null

$Clone = Join-Path $WorkDir 'llama-go'
$Build = Join-Path $WorkDir 'build'

try {
    # ---- 1. Clone llama-go (sources only; submodule comes after the ref is known)
    Step "Cloning llama-go into $Clone"
    git clone https://github.com/tcpipuk/llama-go $Clone
    if ($LASTEXITCODE -ne 0) { Fail 'git clone failed.' }

    # ---- 2. Pick the llama-go ref that matches the vendored bindings ---------
    # cgo compiles the Go bindings against the VENDORED llama.cpp headers on
    # every build of the app, so the archives have to come from the same
    # llama.cpp revision as those headers.
    #
    # Match the files that define the ABI: wrapper.h is the Go<->C contract, and
    # llama.h/ggml.h are what the Go bindings and the prebuilt archives were
    # written against. llama-go has bumped its llama.cpp submodule since this tree
    # was vendored, and a newer llama.h changes llama_model_params, so a build
    # from HEAD links a llama.cpp that disagrees with the vendored bindings and
    # every model load dies with 0xc0000005 (a low-address read inside
    # llama_wrapper_model_load).
    #
    # wrapper.cpp is deliberately NOT part of the match set: the vendored copy is
    # patched relative to upstream and is the implementation that gets compiled
    # (see step 6), so it cannot drift.
    $abiFiles = @('wrapper.h', 'llama.cpp\include\llama.h', 'llama.cpp\ggml\include\ggml.h')
    $wantHash = @{}
    foreach ($rel in $abiFiles) {
        $h = Get-NormalizedHash (Join-Path $Vendored $rel)
        if (-not $h) { Fail "vendored third_party/llama-go/$rel is missing; cannot ABI-guard the build." }
        $wantHash[$rel] = $h
    }

    function Test-AbiMatches([string]$Repo) {
        foreach ($rel in $abiFiles) {
            $h = Get-NormalizedHash (Join-Path $Repo $rel)
            if (-not $h -or $h -ne $wantHash[$rel]) { return $false }
        }
        return $true
    }

    function Sync-Submodule {
        Invoke-Lenient { git -C $Clone submodule update --init --recursive 2>&1 | Out-Null }
        if ($LASTEXITCODE -ne 0) { Fail 'llama.cpp submodule update failed.' }
        if (-not (Test-Path (Join-Path $Clone 'llama.cpp\CMakeLists.txt'))) {
            Fail 'llama.cpp sources are missing after the submodule update.'
        }
    }

    # Candidates: HEAD, then every llama-go commit that moved the llama.cpp
    # submodule (newest first), then tags. Deduplicated by the pinned llama.cpp
    # SHA so refs pinning the same revision are only fetched once. Commits come
    # before tags because the submodule bumps are the discriminating set and
    # trying them first keeps the search to a couple of fetches.
    $candidates = New-Object System.Collections.Generic.List[string]
    $seen = New-Object System.Collections.Generic.HashSet[string]
    function Add-AbiCandidate([string]$Ref) {
        $gl = (git -C $Clone ls-tree $Ref llama.cpp) -split '\s+' |
            Where-Object { $_ -match '^[0-9a-f]{40}$' } | Select-Object -First 1
        if (-not $gl) { return }
        if ($seen.Add($gl)) { $candidates.Add($Ref) }
    }
    foreach ($ref in @('HEAD')) { Add-AbiCandidate $ref }
    foreach ($ref in @(git -C $Clone log --format='%H' -- llama.cpp)) { Add-AbiCandidate $ref }
    foreach ($ref in @(git -C $Clone tag --sort=-version:refname)) { Add-AbiCandidate $ref }

    Step 'Finding the llama-go ref whose llama.cpp matches the vendored headers'
    if ($LlamaGoRef) {
        Write-Host "  pinned to $LlamaGoRef by -LlamaGoRef"
        $candidates.Clear()
        $candidates.Add($LlamaGoRef)
    }

    $Ref = $null
    foreach ($candidate in $candidates) {
        Invoke-Lenient { git -C $Clone checkout --force $candidate 2>&1 | Out-Null }
        if ($LASTEXITCODE -ne 0) { continue }
        Sync-Submodule
        if (Test-AbiMatches $Clone) { $Ref = $candidate; break }
        Write-Host "  $candidate does not match the vendored bindings"
    }
    if (-not $Ref) {
        Fail 'No llama-go ref matches the vendored bindings. Pass -LlamaGoRef <ref> for a llama-go commit whose llama.cpp matches third_party/llama-go/llama.cpp.'
    }
    Write-Host "  matched ref: $Ref"

    # ---- 4. MinGW patch for cpp-httplib (idempotent) -------------------------
    # MinGW does not define _WIN32_WINNT/WINVER, which cpp-httplib needs.
    $marker = '_WIN32_WINNT=0x0A00 WINVER=0x0A00'
    foreach ($rel in @('llama.cpp\common\CMakeLists.txt', 'llama.cpp\vendor\cpp-httplib\CMakeLists.txt')) {
        $path = Join-Path $Clone $rel
        if (-not (Test-Path $path)) { continue }
        if ((Get-Content $path -Raw) -match [regex]::Escape($marker)) { continue }
        Add-Content -Path $path -Value @"

# vectile build-llamago-archives-windows.ps1: MinGW does not define
# _WIN32_WINNT/WINVER by default; cpp-httplib requires the Windows 10 API level.
if (MINGW)
    target_compile_definitions(`${TARGET} PRIVATE _WIN32_WINNT=0x0A00 WINVER=0x0A00)
endif()
"@
        Write-Host "  patched: $rel"
    }

    # ---- 5. Configure + build ------------------------------------------------
    Step "Configuring with CMake (Ninja + MinGW GCC $gccVersion)"
    New-Item -ItemType Directory -Force -Path $Build | Out-Null
    # -DGGML_NATIVE=OFF is the portability switch (no -march=native, no
    # AVX-VNNI). The AVX512*/AVX_VNNI options are already OFF by default
    # upstream; they are passed explicitly so a future default flip cannot
    # silently reintroduce the trap.
    & cmake -G Ninja -S (Join-Path $Clone 'llama.cpp') -B $Build `
        -DBUILD_SHARED_LIBS=OFF -DLLAMA_CURL=OFF -DCMAKE_BUILD_TYPE=Release `
        -DCMAKE_C_COMPILER=gcc -DCMAKE_CXX_COMPILER=g++ `
        -DGGML_NATIVE=OFF `
        -DGGML_AVX_VNNI=OFF -DGGML_AVX512=OFF -DGGML_AVX512_VBMI=OFF `
        -DGGML_AVX512_VNNI=OFF -DGGML_AVX512_BF16=OFF
    if ($LASTEXITCODE -ne 0) { Fail 'cmake configure failed.' }

    # Belt and braces: assert the configure actually took.
    $cache = Join-Path $Build 'CMakeCache.txt'
    if (-not (Select-String -Path $cache -Pattern 'GGML_NATIVE:BOOL=OFF' -Quiet)) {
        Fail 'GGML_NATIVE is not OFF in the CMake cache; -march=native would make these archives CPU-specific.'
    }
    foreach ($key in 'GGML_AVX_VNNI:BOOL=OFF', 'GGML_AVX512:BOOL=OFF', 'GGML_AVX512_VNNI:BOOL=OFF') {
        if ((Select-String -Path $cache -Pattern ([regex]::Escape($key)) -Quiet)) { Write-Host "  confirmed $key" }
    }

    Step "Building ggml llama llama-common (-j $Jobs, a few minutes)"
    & cmake --build $Build --target ggml llama llama-common --config Release -j $Jobs
    if ($LASTEXITCODE -ne 0) { Fail 'cmake build failed.' }

    # ---- 6. Compile wrapper.cpp -> libbinding.a ------------------------------
    # CRITICAL: libbinding.a has to be compiled against the SAME llama.cpp
    # headers as the libraries CMake just built. wrapper.cpp fills in
    # llama_model_params and hands it to llama_model_load_from_file, so if the
    # two sides disagree on that struct's layout llama.cpp reads garbage and the
    # process dies inside model load (0xc0000005). The vendored
    # third_party/llama-go/llama.cpp tree is a headers-only snapshot and may be
    # older than the clone, so compile the clone's wrapper.cpp out of the clone's
    # OWN tree. That is safe for the Go side because the Go bindings only ever
    # touch the llama_wrapper_* API in wrapper.h, which contains no llama.cpp
    # types and is ABI-guarded against the vendored copy in step 2.
    Step 'Compiling wrapper.cpp and assembling libbinding.a'
    # Compile the VENDORED wrapper.cpp. It is the implementation of record for
    # this app's Go bindings (and is patched relative to upstream); at the matched
    # ref its wrapper.h and the clone's are identical, so it satisfies the same
    # contract. The include dirs come from the clone because that is where the
    # llama.cpp sources live, and at the matched ref llama.h/ggml.h are identical
    # to the vendored copies anyway.
    if (-not (Test-AbiMatches $Clone)) {
        Fail "ABI drift: the clone at '$Ref' no longer matches the vendored headers."
    }

    $incs = @(
        "-I$Vendored", "-I$Clone", "-I$Clone\llama.cpp", "-I$Clone\llama.cpp\include",
        "-I$Clone\llama.cpp\ggml\include", "-I$Clone\llama.cpp\common",
        "-I$Clone\llama.cpp\vendor", "-I$Clone\common"
    )
    $wrapperObj = Join-Path $Build 'wrapper.o'
    & g++ @incs -O3 -DNDEBUG -std=c++17 -fPIC -D_WIN32_WINNT=0x0A00 -DWINVER=0x0A00 `
        -c (Join-Path $Clone 'wrapper.cpp') -o $wrapperObj
    if ($LASTEXITCODE -ne 0) { Fail 'wrapper.cpp compile failed.' }
    $binding = Join-Path $Build 'libbinding.a'
    Remove-Item $binding -Force -ErrorAction SilentlyContinue
    & ar crs $binding $wrapperObj
    if ($LASTEXITCODE -ne 0) { Fail 'ar crs libbinding.a failed.' }

    # ---- 7. Collect the archives --------------------------------------------
    # MinGW drops the "lib" prefix on the ggml/llama targets, so look for both
    # spellings instead of assuming one.
    function Find-Archive([string]$Base) {
        foreach ($name in @("lib$Base.a", "$Base.a")) {
            $hit = Get-ChildItem $Build -Recurse -File -ErrorAction SilentlyContinue |
                Where-Object { $_.Name -eq $name } | Select-Object -First 1
            if ($hit) { return $hit.FullName }
        }
        return $null
    }

    $staging = Join-Path $WorkDir 'staging'
    New-Item -ItemType Directory -Force -Path $staging | Out-Null
    Copy-Item $binding (Join-Path $staging 'libbinding.a') -Force

    $wanted = [ordered]@{
        'llama-common'      = 'libllama-common.a'
        'llama-common-base' = 'libllama-common-base.a'
        'llama'             = 'libllama.a'
        'ggml-cpu'          = 'libggml-cpu.a'
        'ggml'              = 'libggml.a'
        'ggml-base'         = 'libggml-base.a'
    }
    foreach ($base in $wanted.Keys) {
        $src = Find-Archive $base
        if (-not $src) { Fail "Could not find archive '$base' under $Build. The llama.cpp build did not produce it." }
        Copy-Item $src (Join-Path $staging $wanted[$base]) -Force
    }

    # ---- 8. Portability gate -------------------------------------------------
    # Refuse to install anything that needs a CPU newer than the AVX2 baseline.
    Step 'Verifying CPU portability (no AVX-VNNI, no AVX-512)'
    $objdump = (Get-Command objdump.exe -ErrorAction SilentlyContinue).Source
    if (-not $objdump -and (Test-Path (Join-Path $MinGWBin 'objdump.exe'))) { $objdump = Join-Path $MinGWBin 'objdump.exe' }
    if (-not $objdump) { Fail 'objdump.exe not found; cannot verify archive portability.' }

    # vpdpbusd = AVX-VNNI (Alder Lake 2021+). %zmm / {k1} = AVX-512.
    $offenders = @()
    $cpuVerified = $false
    foreach ($f in Get-ChildItem $staging -Filter *.a) {
        $dis = Join-Path $WorkDir ('disasm_' + $f.BaseName + '.txt')
        Invoke-Lenient { & $objdump -d $f.FullName 2>&1 | Out-File -FilePath $dis -Encoding ascii }
        if ($LASTEXITCODE -ne 0) {
            Fail "objdump could not read $($f.Name) (exit $LASTEXITCODE); the portability check cannot be trusted."
        }

        # A vacuous pass (objdump silently produced nothing) would be worse than
        # no check at all, so require real disassembly from anything non-trivial.
        $instr = (Select-String -Path $dis -Pattern '^\s*[0-9a-f]+:\s' | Measure-Object).Count
        if ($instr -eq 0 -and $f.Length -gt 100KB) {
            Fail "objdump found no instructions in $($f.Name); the portability check cannot be trusted."
        }

        $bad = Select-String -Path $dis -Pattern 'vpdpbusd', '%zmm', '\{k1\}' |
            Select-Object -First 3 -ExpandProperty Line
        if ($bad) { $offenders += "$($f.Name): $($bad -join ' | ')" }

        # Anchor: ggml-cpu is where the kernels live. AVX2 must be present (it is
        # the intended ceiling), which also proves the scan is reading the code.
        if ($f.Name -eq 'libggml-cpu.a') {
            $ymm = (Select-String -Path $dis -Pattern '%ymm' -SimpleMatch | Measure-Object).Count
            if ($ymm -eq 0) {
                Fail 'libggml-cpu.a contains no AVX2 code, so the disassembly scan is not reading the kernels.'
            }
            Write-Host ("  libggml-cpu.a: {0:N0} instruction lines, {1:N0} AVX2 (ymm) uses" -f $instr, $ymm)
            $cpuVerified = $true
        }
    }
    if (-not $cpuVerified) { Fail 'libggml-cpu.a was not staged; nothing was verified.' }
    if ($offenders.Count -gt 0) {
        Fail "Archive(s) contain non-portable instructions - GGML_NATIVE was not disabled:`n  $($offenders -join "`n  ")"
    }
    Write-Host '  OK: AVX2 ceiling, no AVX-VNNI, no AVX-512.'

    # ---- 9. Install ----------------------------------------------------------
    Step "Installing archives into $InstallDir"
    New-Item -ItemType Directory -Force -Path $InstallDir | Out-Null
    foreach ($f in Get-ChildItem $staging -Filter *.a) {
        Copy-Item $f.FullName (Join-Path $InstallDir $f.Name) -Force
        Write-Host ("  {0,-24} {1,12:N0} bytes" -f $f.Name, $f.Length)
    }

    Step 'Done. Next:'
    Write-Host '  go test ./backend/embeddings/   (needs the model in place)'
    Write-Host '  git diff --stat third_party/llama-go/windows/amd64'
}
finally {
    if (-not $Keep -and (Test-Path $WorkDir)) {
        Remove-Item $WorkDir -Recurse -Force -ErrorAction SilentlyContinue
    }
    elseif ($Keep) {
        Write-Host "`nWork directory kept at $WorkDir"
    }
}
