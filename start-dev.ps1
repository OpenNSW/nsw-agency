# Run Agency backends and/or frontends with per-agency config.
#
# Usage:
#   .\start-dev.ps1 [-CleanRun] [-EnvFile PATH] <agency> [target]
#
#   <agency>  One of: npqs, fcau, ird, cda, all
#             'all' fans out and starts every agency in parallel.
#   [target]  One of: all (default), backend, frontend
#
# Flags:
#   -CleanRun        Wipe agency database(s) before starting, then run migrations.
#                     SQLite: deletes {agency}_applications.db files.
#                     Postgres: drops and recreates the database.
#   -EnvFile PATH    Load additional env vars (non-clobbering) before
#                     per-agency defaults. Useful for sharing a root .env.
#
# Env-var precedence (highest to lowest):
#   parent shell env > -EnvFile > backend/.env (for backend vars) > script defaults
#
# Ctrl-C terminates every child process.

Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'

$IDP_BASE_URL = 'https://localhost:8090'

# Single source of truth for per-agency config: BE_PORT, FE_PORT, IDP_CLIENT_ID, NSW_CLIENT_ID, APP_NAME, OU_HANDLE.
$AGENCY_CONFIGS = [ordered]@{
    npqs = @{ BE_PORT = 8081; FE_PORT = 5174; IDP_CLIENT_ID = 'OGA_PORTAL_APP_NPQS'; NSW_CLIENT_ID = 'NPQS_TO_NSW'; APP_NAME = 'National Plant Quarantine Service (NPQS)'; OU_HANDLE = 'npqs' }
    fcau = @{ BE_PORT = 8082; FE_PORT = 5175; IDP_CLIENT_ID = 'OGA_PORTAL_APP_FCAU'; NSW_CLIENT_ID = 'FCAU_TO_NSW'; APP_NAME = 'Food Control Administration Unit (FCAU)';  OU_HANDLE = 'fcau' }
    ird  = @{ BE_PORT = 8083; FE_PORT = 5176; IDP_CLIENT_ID = 'OGA_PORTAL_APP_IRD';  NSW_CLIENT_ID = 'IRD_TO_NSW';  APP_NAME = 'Inland Revenue Department (IRD)';          OU_HANDLE = 'ird'  }
    cda  = @{ BE_PORT = 8084; FE_PORT = 5177; IDP_CLIENT_ID = 'OGA_PORTAL_APP_CDA';  NSW_CLIENT_ID = 'CDA_TO_NSW';  APP_NAME = 'Coconut Development Authority (CDA)';       OU_HANDLE = 'cda'  }
}

$ALL_AGENCIES = $AGENCY_CONFIGS.Keys | Sort-Object

[CmdletBinding()]
param(
    [Parameter(Position = 0)]
    [string]$Agency = '',

    [Parameter(Position = 1)]
    [string]$Target = 'all',

    [Parameter()]
    [switch]$CleanRun,

    [Parameter()]
    [Alias('env-file')]
    [string]$EnvFile = ''
)

function Show-Usage {
    $agencies = ($ALL_AGENCIES -join ', ') + ', all'
    [Console]::Error.WriteLine(@"
Usage: .\start-dev.ps1 [-CleanRun] [-EnvFile PATH] <agency> [target]

  <agency>  One of: $agencies
  [target]  One of: all (default), backend, frontend

Flags:
  -CleanRun       Wipe agency DB(s) then run migrations before starting
  -EnvFile PATH   Load a root-level env file (non-clobbering)

Examples:
  .\start-dev.ps1 npqs              # NPQS backend + frontend
  .\start-dev.ps1 fcau backend      # FCAU backend only
  .\start-dev.ps1 all               # every agency, backends + frontends
  .\start-dev.ps1 all -CleanRun     # wipe all agency DBs, then start
"@)
    exit 1
}

if ($Agency -eq '') { Show-Usage }

if ($Target -notin @('all', 'backend', 'frontend')) {
    Write-Host "[start-dev] Unknown target '$Target'." -ForegroundColor Red
    Show-Usage
}

if (-not $AGENCY_CONFIGS.Contains($Agency) -and $Agency -ne 'all') {
    Write-Host "[start-dev] Unknown agency '$Agency'. Expected: $($ALL_AGENCIES -join ', '), all." -ForegroundColor Red
    exit 1
}

$ROOT_DIR     = $PSScriptRoot
$BACKEND_DIR  = Join-Path $ROOT_DIR 'backend'
$FRONTEND_DIR = Join-Path $ROOT_DIR 'frontend'

# Cross-platform helper variables for process execution
$isWindows = ($env:OS -like "*Windows*") -or ($PSVersionTable.Platform -eq 'Win32NT') -or ($IsWindows)
$shellCmd  = if ($isWindows) { 'cmd.exe' } else { '/bin/sh' }
$shellArg  = if ($isWindows) { '/c' } else { '-c' }

$jobs = [System.Collections.Generic.List[System.Diagnostics.Process]]::new()

function Stop-AllJobs {
    if ($jobs.Count -eq 0) { return }
    Write-Host ""
    Write-Host "[start-dev] Stopping $($jobs.Count) process(es)..."
    foreach ($p in $jobs) {
        if (-not $p.HasExited) {
            try {
                if ($isWindows) {
                    Start-Process taskkill.exe -ArgumentList "/F", "/T", "/PID", $p.Id -NoNewWindow -Wait | Out-Null
                } else {
                    if ($PSVersionTable.PSVersion.Major -ge 7) {
                        $p.Kill($true)
                    } else {
                        Start-Process kill -ArgumentList "-TERM", "-$($p.Id)" -NoNewWindow -Wait | Out-Null
                    }
                }
            } catch {
                try { $p.Kill() } catch { }
            }
        }
    }
    foreach ($p in $jobs) {
        try { $p.WaitForExit(3000) | Out-Null } catch { }
    }
}

# Source a .env file without clobbering vars already set in the env block.
function Merge-EnvFile {
    param([string]$Path, $Block)
    if (-not (Test-Path $Path)) { return }
    foreach ($line in Get-Content $Path) {
        if ($line -match '^\s*$' -or $line -match '^\s*#') { continue }
        if ($line -notmatch '=') { continue }
        
        $line = $line -replace '^export\s+', ''
        $idx  = $line.IndexOf('=')
        $key  = $line.Substring(0, $idx).Trim()
        $val  = $line.Substring($idx + 1).Trim()
        
        if ($key -notmatch '^[A-Za-z_][A-Za-z0-9_]*$') { continue }
        
        # Parse quoted values and strip comments outside of quotes
        if ($val -match '^"(.*)"\s*(#.*)?$') {
            $val = $Matches[1]
        } elseif ($val -match "^'(.*)'\s*(#.*)?$") {
            $val = $Matches[1]
        } else {
            $val = ($val -split '\s+#')[0].Trim()
        }
        
        if (-not $Block.Contains($key)) { $Block[$key] = $val }
    }
}

function Clean-Databases {
    param([string[]]$Agencies)
    $dbEnv = [System.Environment]::GetEnvironmentVariables('Process').Clone()
    if ($EnvFile -ne '') { Merge-EnvFile -Path $EnvFile -Block $dbEnv }
    $explicitDbName = if ($dbEnv.Contains('DB_NAME')) { $dbEnv['DB_NAME'] } else { $null }
    Merge-EnvFile -Path (Join-Path $BACKEND_DIR '.env') -Block $dbEnv

    $dbDriver = if ($dbEnv.Contains('DB_DRIVER')) { $dbEnv['DB_DRIVER'] } else { 'sqlite' }
    Write-Host "[start-dev] Cleaning agency databases (driver: $dbDriver)..."

    if ($dbDriver -eq 'sqlite') {
        foreach ($agency in $Agencies) {
            $dbPath = Join-Path $BACKEND_DIR "${agency}_applications.db"
            if (Test-Path $dbPath) {
                Write-Host "[start-dev]   Deleting SQLite DB for ${agency}: $dbPath"
                Remove-Item $dbPath -Force
            } else {
                Write-Host "[start-dev]   SQLite DB for ${agency} not found (nothing to delete): $dbPath"
            }
        }
    } elseif ($dbDriver -eq 'postgres') {
        # psql can run on both Windows and macOS/Linux
        $psqlCmd = if ($isWindows) { 'psql.exe' } else { 'psql' }
        if (-not (Get-Command $psqlCmd -ErrorAction SilentlyContinue)) {
            Write-Host "[start-dev] Error: psql required for Postgres DB cleaning but not found in PATH." -ForegroundColor Red
            exit 1
        }
        $dbHost     = if ($dbEnv.Contains('DB_HOST'))     { $dbEnv['DB_HOST'] }     else { 'localhost' }
        $dbPort     = if ($dbEnv.Contains('DB_PORT'))     { $dbEnv['DB_PORT'] }     else { '5432' }
        $dbUser     = if ($dbEnv.Contains('DB_USER'))     { $dbEnv['DB_USER'] }     else { 'postgres' }
        $dbPassword = if ($dbEnv.Contains('DB_PASSWORD')) { $dbEnv['DB_PASSWORD'] } else { 'changeme' }
        
        $env:PGPASSWORD = $dbPassword
        foreach ($agency in $Agencies) {
            $dbName = if ($explicitDbName) { $explicitDbName } else { "${agency}_nsw_agency_db" }
            Write-Host "[start-dev]   Dropping and recreating Postgres database: $dbName"
            & $psqlCmd -h $dbHost -p $dbPort -U $dbUser -d postgres -c "SELECT pg_terminate_backend(pid) FROM pg_stat_activity WHERE datname = '$dbName' AND pid <> pg_backend_pid();" | Out-Null
            if ($LASTEXITCODE -ne 0) {
                Write-Host "[start-dev] Error: Failed to terminate connections to $dbName" -ForegroundColor Red
                Remove-Item Env:PGPASSWORD -ErrorAction SilentlyContinue; exit $LASTEXITCODE
            }
            & $psqlCmd -h $dbHost -p $dbPort -U $dbUser -d postgres -c "DROP DATABASE IF EXISTS `"$dbName`";"
            if ($LASTEXITCODE -ne 0) {
                Write-Host "[start-dev] Error: Failed to drop Postgres database $dbName" -ForegroundColor Red
                Remove-Item Env:PGPASSWORD -ErrorAction SilentlyContinue; exit $LASTEXITCODE
            }
            & $psqlCmd -h $dbHost -p $dbPort -U $dbUser -d postgres -c "CREATE DATABASE `"$dbName`";"
            if ($LASTEXITCODE -ne 0) {
                Write-Host "[start-dev] Error: Failed to create Postgres database $dbName" -ForegroundColor Red
                Remove-Item Env:PGPASSWORD -ErrorAction SilentlyContinue; exit $LASTEXITCODE
            }
        }
        Remove-Item Env:PGPASSWORD -ErrorAction SilentlyContinue
    } else {
        Write-Host "[start-dev] Unknown DB_DRIVER '$dbDriver'; skipping database clean." -ForegroundColor Yellow
    }
}

function Ensure-BrandingFile {
    param([string]$AgencyName, [string]$AppName)
    $ConfigDir = Join-Path $FRONTEND_DIR 'public/configs'
    $FilePath  = Join-Path $ConfigDir "${AgencyName}.branding.json"
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
    "partnerLogos": [{"url": "", "alt": ""}]
  }
}
"@
    $Utf8NoBom = New-Object System.Text.UTF8Encoding $false
    [System.IO.File]::WriteAllText($FilePath, $Content, $Utf8NoBom)
    Write-Host "[start-dev] Wrote branding file: $FilePath"
}

function Run-Migrations {
    param([string[]]$Agencies)
    $migrEnv = [System.Environment]::GetEnvironmentVariables('Process').Clone()
    if ($EnvFile -ne '') { Merge-EnvFile -Path $EnvFile -Block $migrEnv }
    $explicitDbName = if ($migrEnv.Contains('DB_NAME')) { $migrEnv['DB_NAME'] } else { $null }
    Merge-EnvFile -Path (Join-Path $BACKEND_DIR '.env') -Block $migrEnv

    $dbDriver = if ($migrEnv.Contains('DB_DRIVER')) { $migrEnv['DB_DRIVER'] } else { 'sqlite' }
    Write-Host "[start-dev] Running migrations (driver: $dbDriver)..."

    foreach ($agency in $Agencies) {
        $agencyEnv = $migrEnv.Clone()
        if ($dbDriver -eq 'sqlite') {
            Write-Host "[start-dev]   migrate up -> ${agency}_applications.db"
            $agencyEnv['DB_DRIVER'] = 'sqlite'
            $agencyEnv['DB_PATH']   = "./${agency}_applications.db"
        } else {
            $dbName = if ($explicitDbName) { $explicitDbName } else { "${agency}_nsw_agency_db" }
            Write-Host "[start-dev]   migrate up -> $dbName"
            $agencyEnv['DB_NAME'] = $dbName
        }
        
        $psi = [System.Diagnostics.ProcessStartInfo]::new($shellCmd, "$shellArg `"go run ./cmd/migrate up`"")
        $psi.WorkingDirectory = $BACKEND_DIR
        $psi.UseShellExecute  = $false
        foreach ($k in $agencyEnv.Keys) { $psi.Environment[$k] = [string]$agencyEnv[$k] }
        $proc = [System.Diagnostics.Process]::Start($psi)
        $proc.WaitForExit()
        if ($proc.ExitCode -ne 0) {
            Write-Host "[start-dev] Error: Migration failed for $agency (exit code $($proc.ExitCode))." -ForegroundColor Red
            exit $proc.ExitCode
        }
    }
}

function Start-Backend {
    param([string]$AgencyName)
    $cfg         = $AGENCY_CONFIGS[$AgencyName]
    $bePort      = $cfg.BE_PORT
    $nswClientId = $cfg.NSW_CLIENT_ID

    Write-Host "[start-dev] Starting $AgencyName backend  -> http://localhost:$bePort"

    $envBlock = [System.Environment]::GetEnvironmentVariables('Process').Clone()
    if ($EnvFile -ne '') { Merge-EnvFile -Path $EnvFile -Block $envBlock }

    if (-not $envBlock.Contains('PORT'))            { $envBlock['PORT']            = "$bePort"                         }
    if (-not $envBlock.Contains('DB_PATH'))         { $envBlock['DB_PATH']         = "./${AgencyName}_applications.db" }
    if (-not $envBlock.Contains('DB_NAME'))         { $envBlock['DB_NAME']         = "${AgencyName}_nsw_agency_db"     }
    if (-not $envBlock.Contains('NSW_CLIENT_ID'))   { $envBlock['NSW_CLIENT_ID']   = $nswClientId                      }
    if (-not $envBlock.Contains('AUTH_EXPECTED_OU')) { $envBlock['AUTH_EXPECTED_OU'] = $cfg.OU_HANDLE                  }
    if (-not $envBlock.Contains('ALLOWED_ORIGINS')) { $envBlock['ALLOWED_ORIGINS'] = "http://localhost:$($cfg.FE_PORT)" }

    $dotEnv = Join-Path $BACKEND_DIR '.env'
    if (Test-Path $dotEnv) {
        Merge-EnvFile -Path $dotEnv -Block $envBlock
    } else {
        Write-Host "[start-dev] WARNING: backend/.env not found - backend will fail if NSW_* vars are unset." -ForegroundColor Yellow
    }

    if (-not $envBlock.Contains('DB_DRIVER')) { $envBlock['DB_DRIVER'] = 'sqlite' }

    $psi = [System.Diagnostics.ProcessStartInfo]::new($shellCmd, "$shellArg `"go run ./cmd/server`"")
    $psi.WorkingDirectory = $BACKEND_DIR
    $psi.UseShellExecute  = $false
    foreach ($k in $envBlock.Keys) { $psi.Environment[$k] = [string]$envBlock[$k] }
    $jobs.Add([System.Diagnostics.Process]::Start($psi))
}

function Start-Frontend {
    param([string]$AgencyName)
    $cfg       = $AGENCY_CONFIGS[$AgencyName]
    $fePort    = $cfg.FE_PORT
    $bePort    = $cfg.BE_PORT
    $idpClient = $cfg.IDP_CLIENT_ID
    $appName   = $cfg.APP_NAME
    $ouHandle  = $cfg.OU_HANDLE

    Ensure-BrandingFile -AgencyName $AgencyName -AppName $appName

    Write-Host "[start-dev] Starting $AgencyName frontend -> http://localhost:$fePort (branding: $AgencyName, idp: $idpClient)"

    $envBlock = [System.Environment]::GetEnvironmentVariables('Process').Clone()
    if ($EnvFile -ne '') { Merge-EnvFile -Path $EnvFile -Block $envBlock }

    if (-not $envBlock.Contains('VITE_PORT'))                   { $envBlock['VITE_PORT']                  = "$fePort"                  }
    if (-not $envBlock.Contains('VITE_BRANDING_NAME'))          { $envBlock['VITE_BRANDING_NAME']          = $AgencyName               }
    if (-not $envBlock.Contains('VITE_API_BASE_URL'))           { $envBlock['VITE_API_BASE_URL']           = "http://localhost:$bePort" }
    if (-not $envBlock.Contains('VITE_IDP_BASE_URL'))           { $envBlock['VITE_IDP_BASE_URL']           = $IDP_BASE_URL             }
    if (-not $envBlock.Contains('VITE_IDP_CLIENT_ID'))          { $envBlock['VITE_IDP_CLIENT_ID']          = $idpClient                }
    if (-not $envBlock.Contains('VITE_IDP_SCOPES'))             { $envBlock['VITE_IDP_SCOPES']             = 'openid,profile,email,ou' }
    if (-not $envBlock.Contains('VITE_IDP_EXPECTED_OU_HANDLE')) { $envBlock['VITE_IDP_EXPECTED_OU_HANDLE'] = $ouHandle                 }
    if (-not $envBlock.Contains('VITE_APP_URL'))                { $envBlock['VITE_APP_URL']                = "http://localhost:$fePort" }

    $psi = [System.Diagnostics.ProcessStartInfo]::new($shellCmd, "$shellArg `"pnpm run dev`"")
    $psi.WorkingDirectory = $FRONTEND_DIR
    $psi.UseShellExecute  = $false
    foreach ($k in $envBlock.Keys) { $psi.Environment[$k] = [string]$envBlock[$k] }
    $jobs.Add([System.Diagnostics.Process]::Start($psi))
}

# Load optional root-level env file before per-agency defaults.
if ($EnvFile -ne '') {
    if (-not (Test-Path $EnvFile)) {
        Write-Host "[start-dev] Error: --env-file not found: $EnvFile" -ForegroundColor Red
        exit 1
    }
}

# Resolve the agency list to launch.
if ($Agency -eq 'all') {
    $agencyList = $ALL_AGENCIES
} else {
    $agencyList = @($Agency)
}

if ($CleanRun) {
    Clean-Databases -Agencies $agencyList
    Run-Migrations  -Agencies $agencyList
}

try {
    foreach ($a in $agencyList) {
        if ($Target -eq 'all' -or $Target -eq 'backend')  { Start-Backend  $a }
        if ($Target -eq 'all' -or $Target -eq 'frontend') { Start-Frontend $a }
    }

    Write-Host "[start-dev] $($jobs.Count) process(es) running. Logs from all processes will interleave below. Press Ctrl-C to stop."

    $reported = @{}
    while ($true) {
        foreach ($p in $jobs) {
            if ($p.HasExited -and -not $reported[$p.Id]) {
                Write-Host "[start-dev] Process $($p.Id) exited with code $($p.ExitCode)." -ForegroundColor Yellow
                $reported[$p.Id] = $true
            }
        }
        Start-Sleep -Milliseconds 500
    }
} finally {
    Stop-AllJobs
}
