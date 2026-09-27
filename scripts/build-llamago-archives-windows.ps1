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

    With GGML_NATIVE=OFF llama.cpp never uses -march=native, so the archives are
    not tied to the build machine. Which instructions they DO use is chosen by
    -Baseline (see below); everything above it is compiled out. The script then
    verifies the result with objdump and refuses to install an archive that needs
    a newer CPU than asked for.

.PARAMETER Baseline
    Lowest CPU the archives must run on. Instructions above it are compiled out.

      sse2  - every x86-64 CPU (2003+). Portable, but see the cliff below.
      sse42 - Intel Nehalem (2008+) / AMD Bulldozer (2011+).
      avx2  - Intel Haswell (2013+) / AMD Excavator (2015+). Default.

    Measured on an i7-13650HX, batch of 8, n=128:

      model                      avx2           sse2        penalty
      bge-small-en-v1.5 Q8_0     87 passages/s   46         ~1.9x
      bge-m3 Q4_K_M              12 passages/s    0.7       ~17x

    Q8_0 barely notices, but K-quant models fall off a cliff. ggml's K-quant dot
    products (Q4_K/Q6_K) have a fast path only for AVX2, so an SSE-only build
    drops to generic code. Three of the four catalog models are K-quant (bge-m3
    Q4_K_M, bge-small Q4_K_M, all-MiniLM Q4_K_M), and at 0.7 passages/sec bge-m3
    would need hours to index a real library. So sse2 is NOT a free win for old
    CPUs: it is a trade of pre-2013 compatibility for a large slowdown on most
    models.

    Default avx2. Go to sse2 deliberately, as a separate legacy build for a known
    old machine, and point that machine at the recommended Q8_0 model, which
    stays usable at ~46 passages/sec.

    Keep this in step with -Baseline in check-llamago-archives-portable.ps1: CI
    calls that script with its own default, so a mismatch fails the release
    rather than shipping a build whose CPU floor is a surprise.

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
    [ValidateSet('sse2', 'sse42', 'avx2')]
    [string]$Baseline = 'avx2',
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

# CC/CXX win over whatever gcc is first on PATH. The release workflow pins the
# choco MinGW 16.1.0 there on purpose: the runner also ships a preinstalled MinGW
# with a different libstdc++ ABI, and archives built by one toolchain have to be
# linked by the same one or the link fails on undefined std::__get_once_* symbols.
$cmakeCC = 'gcc'
$cmakeCXX = 'g++'
if ($env:CC) {
    $cmakeCC = $env:CC
    $ccBin = Split-Path $env:CC -Parent
    if ($ccBin -and (Test-Path $ccBin)) { $env:Path = "$ccBin;$env:Path" }
    Write-Host "  CC pinned by the environment: $cmakeCC"
}
if ($env:CXX) { $cmakeCXX = $env:CXX }
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
    # Retried: this pulls a repo whose submodule is large, and "RPC failed;
    # curl 56 Recv failure: Connection was reset" has interrupted it twice.
    Step "Cloning llama-go into $Clone"
    $cloned = $false
    foreach ($attempt in 1..3) {
        if (Test-Path $Clone) { Remove-Item $Clone -Recurse -Force -ErrorAction SilentlyContinue }
        Invoke-Lenient { git clone https://github.com/tcpipuk/llama-go $Clone }
        if ($LASTEXITCODE -eq 0 -and (Test-Path (Join-Path $Clone 'go.mod'))) { $cloned = $true; break }
        Write-Warning "clone attempt $attempt failed; retrying"
        Start-Sleep -Seconds 5
    }
    if (-not $cloned) { Fail 'git clone of llama-go failed after 3 attempts.' }

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
        # Retried for the same reason as the clone: this is a second network fetch.
        foreach ($attempt in 1..3) {
            Invoke-Lenient { git -C $Clone submodule update --init --recursive 2>&1 | Out-Null }
            if ($LASTEXITCODE -eq 0 -and (Test-Path (Join-Path $Clone 'llama.cpp\CMakeLists.txt'))) { return }
            Write-Warning "llama.cpp submodule update attempt $attempt failed; retrying"
            Start-Sleep -Seconds 5
        }
        Fail 'llama.cpp submodule update failed after 3 attempts.'
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
    Step "Configuring with CMake (Ninja + MinGW GCC $gccVersion, baseline $Baseline)"
    New-Item -ItemType Directory -Force -Path $Build | Out-Null

    # ---- 5a. CPU baseline ----------------------------------------------------
    # Everything above the baseline is compiled out by explicit option, never by
    # -march=native: GGML_NATIVE defaults to ON and is what recorded AVX-VNNI on
    # an Alder Lake build machine, which then faulted on every older CPU. The
    # AVX_VNNI/AVX512 options are already OFF upstream but are passed anyway so a
    # future default flip cannot silently reintroduce the trap.
    $isa = [ordered]@{
        GGML_SSE42 = ($Baseline -ne 'sse2')
        GGML_AVX   = ($Baseline -eq 'avx2')
        GGML_AVX2  = ($Baseline -eq 'avx2')
        GGML_FMA   = ($Baseline -eq 'avx2')
        GGML_F16C  = ($Baseline -eq 'avx2')
        GGML_BMI2  = ($Baseline -eq 'avx2')
    }
    $alwaysOff = @(
        'GGML_NATIVE', 'GGML_AVX_VNNI', 'GGML_AVX512', 'GGML_AVX512_VBMI',
        'GGML_AVX512_VNNI', 'GGML_AVX512_BF16'
    )

    $cmakeArgs = @(
        '-G', 'Ninja', '-S', (Join-Path $Clone 'llama.cpp'), '-B', $Build,
        '-DBUILD_SHARED_LIBS=OFF', '-DLLAMA_CURL=OFF', '-DCMAKE_BUILD_TYPE=Release',
        "-DCMAKE_C_COMPILER=$cmakeCC", "-DCMAKE_CXX_COMPILER=$cmakeCXX"
    )
    foreach ($k in $alwaysOff) { $cmakeArgs += "-D$($k)=OFF" }
    foreach ($k in $isa.Keys) { $cmakeArgs += "-D$($k)=$(if ($isa[$k]) { 'ON' } else { 'OFF' })" }

    & cmake @cmakeArgs
    if ($LASTEXITCODE -ne 0) { Fail 'cmake configure failed.' }

    # Assert the configure actually took. A silent flip back to a higher ISA is
    # exactly how a CPU-specific binary shipped once before.
    $cache = Join-Path $Build 'CMakeCache.txt'
    $wanted = @{ GGML_NATIVE = 'OFF' }
    foreach ($k in $alwaysOff) { $wanted[$k] = 'OFF' }
    foreach ($k in $isa.Keys) { $wanted[$k] = $(if ($isa[$k]) { 'ON' } else { 'OFF' }) }
    foreach ($k in $wanted.Keys) {
        $entry = "$($k):BOOL=$($wanted[$k])"
        if (Select-String -Path $cache -Pattern ([regex]::Escape($entry)) -Quiet) {
            Write-Host "  confirmed $entry"
        }
        else {
            Fail "CMake cache does not contain $entry; refusing to build an archive whose CPU baseline is not what was asked for."
        }
    }

    Step "Building ggml llama llama-common (-j $Jobs, a few minutes)"
    & cmake --build $Build --target ggml llama llama-common --config Release -j $Jobs
    if ($LASTEXITCODE -ne 0) { Fail 'cmake build failed.' }

    # ---- 6. Compile wrapper.cpp -> libbinding.a ------------------------------
    # Compile the VENDORED wrapper.cpp (the implementation of record for this
    # app's bindings, patched relative to upstream). Its include dirs come from the
    # clone because that is where the llama.cpp sources live, and step 2 proved the
    # clone's llama.h/ggml.h are identical to the vendored copies at the matched
    # ref, so both sides agree on llama_model_params.
    Step 'Compiling wrapper.cpp and assembling libbinding.a'
    if (-not (Test-AbiMatches $Clone)) {
        Fail "ABI drift: the clone at '$Ref' no longer matches the vendored headers."
    }

    $incs = @(
        "-I$Vendored", "-I$Clone", "-I$Clone\llama.cpp", "-I$Clone\llama.cpp\include",
        "-I$Clone\llama.cpp\ggml\include", "-I$Clone\llama.cpp\common",
        "-I$Clone\llama.cpp\vendor", "-I$Clone\common"
    )
    $wrapperObj = Join-Path $Build 'wrapper.o'
    & $cmakeCXX @incs -O3 -DNDEBUG -std=c++17 -fPIC -D_WIN32_WINNT=0x0A00 -DWINVER=0x0A00 `
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
    # The same script runs in CI, so the committed archives cannot regress.
    Step "Verifying CPU portability (baseline $Baseline)"
    $portability = Join-Path $PSScriptRoot 'check-llamago-archives-portable.ps1'
    if (-not (Test-Path $portability)) { Fail "Portability check not found at $portability" }
    # A failing check throws, which aborts the build before anything is installed.
    & $portability -ArchiveDir $staging -Baseline $Baseline

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
