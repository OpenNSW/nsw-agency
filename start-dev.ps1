# Run NSW Agency backends and/or frontends with per-agency config.
#
# Usage:
#   .\start-dev.ps1 <agency> [target]
#
#   <agency>  One of: npqs, fcau, ird, cda, default, all
#             'all' fans out and starts every agency in parallel.
#   [target]  One of: all (default), backend, frontend
#
# Each agency maps to its own:
#   - backend HTTP port and SQLite DB file
#   - frontend dev server port
#   - frontend branding config (public/configs/<agency>.branding.json)
#   - IdP client id
#
# Env-var precedence (highest to lowest):
#   parent shell env > backend/.env (for backend vars) > script defaults
# i.e. $env:PORT=9000; .\start-dev.ps1 npqs honours the override; .env
# can fill in anything the parent didn't set; per-agency defaults are the floor.
#
# Examples:
#   .\start-dev.ps1 npqs              # NPQS backend + frontend
#   .\start-dev.ps1 fcau backend      # FCAU backend only
#   .\start-dev.ps1 ird frontend      # IRD frontend only
#   .\start-dev.ps1 all               # every backend + frontend, in parallel
#   .\start-dev.ps1 all backend       # every backend, no frontends
#
# Ctrl-C terminates every child process.

param(
    [Parameter(Mandatory = $true, Position = 0)]
    [string]$Agency,

    [Parameter(Position = 1)]
    [ValidateSet('all', 'backend', 'frontend')]
    [string]$Target = 'all'
)

Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'

# Single source of truth for per-agency config: BE_PORT, FE_PORT, IDP_CLIENT_ID, NSW_CLIENT_ID.
# Adding an agency means one entry here - nothing else.
$NSW_AGENCY_CONFIGS = [ordered]@{
    npqs    = @{ BE_PORT = 8081; FE_PORT = 5174; IDP_CLIENT_ID = 'NSW_AGENCY_PORTAL_APP_NPQS'; NSW_CLIENT_ID = 'NPQS_TO_NSW' }
    fcau    = @{ BE_PORT = 8082; FE_PORT = 5175; IDP_CLIENT_ID = 'NSW_AGENCY_PORTAL_APP_FCAU'; NSW_CLIENT_ID = 'FCAU_TO_NSW' }
    ird     = @{ BE_PORT = 8083; FE_PORT = 5176; IDP_CLIENT_ID = 'NSW_AGENCY_PORTAL_APP_IRD';  NSW_CLIENT_ID = 'IRD_TO_NSW'  }
    cda     = @{ BE_PORT = 8084; FE_PORT = 5177; IDP_CLIENT_ID = 'NSW_AGENCY_PORTAL_APP_CDA';  NSW_CLIENT_ID = 'CDA_TO_NSW'  }
    default = @{ BE_PORT = 8081; FE_PORT = 5174; IDP_CLIENT_ID = 'NSW_AGENCY_TO_NSW';   NSW_CLIENT_ID = 'NSW_AGENCY_TO_NSW' }
}

$ALL_AGENCIES = $NSW_AGENCY_CONFIGS.Keys | Where-Object { $_ -ne 'default' } | Sort-Object

function Show-Usage {
    $agencies = ($ALL_AGENCIES -join ', ') + ', default, all'
    Write-Host @"
Usage: .\start-dev.ps1 <agency> [target]

  <agency>  One of: $agencies
  [target]  One of: all (default), backend, frontend

Examples:
  .\start-dev.ps1 npqs                  # NPQS backend + frontend
  .\start-dev.ps1 fcau backend          # FCAU backend only
  .\start-dev.ps1 all                   # every agency, backends + frontends
  .\start-dev.ps1 all frontend          # every agency, frontends only
"@ -ForegroundColor Yellow
    exit 1
}

if (-not $NSW_AGENCY_CONFIGS.Contains($Agency) -and $Agency -ne 'all') {
    Write-Host "[start-dev] Unknown agency '$Agency'." -ForegroundColor Red
    Show-Usage
}

$ROOT_DIR   = $PSScriptRoot
$BACKEND_DIR  = Join-Path $ROOT_DIR 'backend'
$FRONTEND_DIR = Join-Path $ROOT_DIR 'frontend'

$jobs = [System.Collections.Generic.List[System.Diagnostics.Process]]::new()

function Ensure-NodeModules {
    $NodeModulesDir = Join-Path $FRONTEND_DIR 'node_modules'
    $ModulesMeta    = Join-Path $NodeModulesDir '.modules.yaml'
    if (-not (Test-Path $ModulesMeta)) {
        Write-Host "[start-dev] node_modules not found - running pnpm install..."
        $psi = [System.Diagnostics.ProcessStartInfo]::new('cmd.exe', '/c pnpm install')
        $psi.WorkingDirectory = $FRONTEND_DIR
        $psi.UseShellExecute  = $false
        $proc = [System.Diagnostics.Process]::Start($psi)
        $proc.WaitForExit()
        if ($proc.ExitCode -ne 0) {
            Write-Host "[start-dev] pnpm install failed with exit code $($proc.ExitCode)." -ForegroundColor Red
            exit $proc.ExitCode
        }
        Write-Host "[start-dev] pnpm install completed."
    }
}

function Ensure-BrandingFile {
    param([string]$BrandingName, [string]$AppName)
    $ConfigDir = Join-Path $ROOT_DIR "frontend/public/configs"
    $FilePath  = Join-Path $ConfigDir "${BrandingName}.branding.json"
    if (-not (Test-Path $FilePath)) {
        New-Item -ItemType Directory -Path $ConfigDir -Force | Out-Null
        $Content = @"
{
  "branding": {
    "systemName": "NSW",
    "appName": "$AppName",
    "logoUrl": "",
    "systemLogoUrl": "",
    "favicon": "",
    "portalName": "",
    "description": "",
    "heroImageUrl": "",
    "partnerLogos": [
      {
        "url": "",
        "alt": ""
      }
    ]
  }
}
"@
        $Utf8NoBom = New-Object System.Text.UTF8Encoding $false
        [System.IO.File]::WriteAllText($FilePath, $Content, $Utf8NoBom)
        Write-Host "[start-dev] Created branding file: $FilePath"
    }
}

function Start-Backend {
    param([string]$AgencyName)
    $cfg = $NSW_AGENCY_CONFIGS[$AgencyName]
    $bePort = $cfg.BE_PORT
    $nswClientId = $cfg.NSW_CLIENT_ID

    Write-Host "[start-dev] Starting $AgencyName backend  -> http://localhost:$bePort (db: ${AgencyName}_applications.db)"

    # Build env block: per-agency defaults < .env < parent env (non-clobber for .env).
    $envBlock = [System.Environment]::GetEnvironmentVariables('Process').Clone()

    if (-not $envBlock.Contains('PORT'))          { $envBlock['PORT']          = "$bePort" }
    if (-not $envBlock.Contains('DB_DRIVER'))     { $envBlock['DB_DRIVER']     = 'sqlite' }
    if (-not $envBlock.Contains('DB_PATH'))       { $envBlock['DB_PATH']       = "./${AgencyName}_applications.db" }
    if (-not $envBlock.Contains('NSW_CLIENT_ID')) { $envBlock['NSW_CLIENT_ID'] = $nswClientId }

    $envFile = Join-Path $BACKEND_DIR '.env'
    if (Test-Path $envFile) {
        foreach ($line in Get-Content $envFile) {
            if ($line -match '^\s*$' -or $line -match '^\s*#') { continue }
            if ($line -notmatch '=') { continue }
            $line = $line -replace '^export\s+', ''
            $idx  = $line.IndexOf('=')
            $key  = $line.Substring(0, $idx).Trim()
            $val  = $line.Substring($idx + 1).Trim() -replace '^"(.*)"$', '$1' -replace "^'(.*)'$", '$1'
            if ($key -notmatch '^[A-Za-z_][A-Za-z0-9_]*$') { continue }
            if (-not $envBlock.Contains($key)) { $envBlock[$key] = $val }
        }
    } else {
        Write-Host "[start-dev] WARNING: backend/.env not found - backend will fail if NSW_* vars are unset." -ForegroundColor Yellow
    }

    $psi = [System.Diagnostics.ProcessStartInfo]::new('cmd.exe', '/c go run ./cmd/server')
    $psi.WorkingDirectory = $BACKEND_DIR
    $psi.UseShellExecute  = $false
    foreach ($k in $envBlock.Keys) { $psi.Environment[$k] = $envBlock[$k] }

    $proc = [System.Diagnostics.Process]::Start($psi)
    $jobs.Add($proc)
}

function Start-Frontend {
    param([string]$AgencyName)
    $cfg = $NSW_AGENCY_CONFIGS[$AgencyName]
    $fePort    = $cfg.FE_PORT
    $bePort    = $cfg.BE_PORT
    $idpClient = $cfg.IDP_CLIENT_ID

    Ensure-NodeModules
    Ensure-BrandingFile -BrandingName $AgencyName -AppName $AgencyName.ToUpper()

    Write-Host "[start-dev] Starting $AgencyName frontend -> http://localhost:$fePort (branding: $AgencyName, idp: $idpClient)"

    $envBlock = [System.Environment]::GetEnvironmentVariables('Process').Clone()

    if (-not $envBlock.Contains('VITE_PORT'))         { $envBlock['VITE_PORT']         = "$fePort" }
    if (-not $envBlock.Contains('VITE_BRANDING_NAME')){ $envBlock['VITE_BRANDING_NAME'] = $AgencyName }
    if (-not $envBlock.Contains('VITE_API_BASE_URL')) { $envBlock['VITE_API_BASE_URL']  = "http://localhost:$bePort" }
    if (-not $envBlock.Contains('VITE_IDP_BASE_URL')) { $envBlock['VITE_IDP_BASE_URL']  = 'https://localhost:8090' }
    if (-not $envBlock.Contains('VITE_IDP_CLIENT_ID')){ $envBlock['VITE_IDP_CLIENT_ID'] = $idpClient }
    if (-not $envBlock.Contains('VITE_APP_URL'))      { $envBlock['VITE_APP_URL']        = "http://localhost:$fePort" }

    $psi = [System.Diagnostics.ProcessStartInfo]::new('cmd.exe', '/c pnpm run dev')
    $psi.WorkingDirectory = $FRONTEND_DIR
    $psi.UseShellExecute  = $false
    foreach ($k in $envBlock.Keys) { $psi.Environment[$k] = $envBlock[$k] }

    $proc = [System.Diagnostics.Process]::Start($psi)
    $jobs.Add($proc)
}

function Stop-AllJobs {
    if ($jobs.Count -eq 0) { return }
    Write-Host ""
    Write-Host "[start-dev] Stopping $($jobs.Count) process(es)..."
    foreach ($p in $jobs) {
        if (-not $p.HasExited) {
            try { taskkill.exe /F /T /PID $p.Id | Out-Null } catch { }
        }
    }
    foreach ($p in $jobs) {
        try { $p.WaitForExit(3000) | Out-Null } catch { }
    }
}

# Resolve agency list.
if ($Agency -eq 'all') {
    $agencyList = $ALL_AGENCIES
} else {
    $agencyList = @($Agency)
}

try {
    foreach ($a in $agencyList) {
        if ($Target -eq 'all' -or $Target -eq 'backend')  { Start-Backend  $a }
        if ($Target -eq 'all' -or $Target -eq 'frontend') { Start-Frontend $a }
    }

    Write-Host "[start-dev] $($jobs.Count) process(es) running. Logs from all processes will interleave below. Press Ctrl-C to stop."

    # Wait for all processes; exit as soon as any one exits unexpectedly.
    $exitCode = 0
    $done = $false
    while (-not $done) {
        foreach ($p in $jobs) {
            if ($p.HasExited) {
                Write-Host "[start-dev] Process $($p.Id) exited with code $($p.ExitCode)." -ForegroundColor Red
                $exitCode = $p.ExitCode
                $done = $true
                break
            }
        }
        if (-not $done) { Start-Sleep -Milliseconds 500 }
    }
} finally {
    Stop-AllJobs
}
exit $exitCode
