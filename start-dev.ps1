# Run Agency backends and/or frontends with per-agency config.
#
# Usage:
#   .\start-dev.ps1 [--clean-run] [--env-file=PATH] <agency> [target]
#
#   <agency>  One of: npqs, fcau, ird, cda, all
#             'all' fans out and starts every agency in parallel.
#   [target]  One of: all (default), backend, frontend
#
# Flags:
#   --clean-run       Wipe agency database(s) before starting.
#                     SQLite: deletes {agency}_applications.db files.
#                     Postgres: drops and recreates the database.
#   --env-file=PATH   Load additional env vars (non-clobbering) before
#   --env-file PATH   per-agency defaults. Useful for sharing a root .env.
#
# Each agency maps to its own:
#   - backend HTTP port and SQLite DB file
#   - frontend dev server port
#   - frontend branding config (public/configs/<agency>.branding.json)
#   - IdP client id
#
# Env-var precedence (highest to lowest):
#   parent shell env > --env-file > backend/.env (for backend vars) > script defaults
# i.e. $env:PORT=9000; .\start-dev.ps1 npqs honours the override; .env can fill in
# anything the parent didn't set; the per-agency defaults below are the floor.
#
# Examples:
#   .\start-dev.ps1 npqs              # NPQS backend + frontend
#   .\start-dev.ps1 fcau backend      # FCAU backend only
#   .\start-dev.ps1 ird frontend      # IRD frontend only
#   .\start-dev.ps1 all               # every backend + frontend, in parallel
#   .\start-dev.ps1 all backend       # every backend, no frontends
#   .\start-dev.ps1 all --clean-run   # wipe all agency DBs, then start
#
# Ctrl-C terminates every child process.

Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'

$IDP_BASE_URL = 'https://localhost:8090'

# Single source of truth for per-agency config: BE_PORT, FE_PORT, IDP_CLIENT_ID, NSW_CLIENT_ID, APP_NAME, OU_HANDLE.
# Adding an agency means one entry here - nothing else.
$AGENCY_CONFIGS = [ordered]@{
    npqs = @{ BE_PORT = 8081; FE_PORT = 5174; IDP_CLIENT_ID = 'NSW_AGENCY_PORTAL_APP_NPQS'; NSW_CLIENT_ID = 'NPQS_TO_NSW'; APP_NAME = 'National Plant Quarantine Service (NPQS)'; OU_HANDLE = 'npqs' }
    fcau = @{ BE_PORT = 8082; FE_PORT = 5175; IDP_CLIENT_ID = 'NSW_AGENCY_PORTAL_APP_FCAU'; NSW_CLIENT_ID = 'FCAU_TO_NSW'; APP_NAME = 'Food Control Administration Unit (FCAU)';  OU_HANDLE = 'fcau' }
    ird  = @{ BE_PORT = 8083; FE_PORT = 5176; IDP_CLIENT_ID = 'NSW_AGENCY_PORTAL_APP_IRD';  NSW_CLIENT_ID = 'IRD_TO_NSW';  APP_NAME = 'Inland Revenue Department (IRD)';          OU_HANDLE = 'ird'  }
    cda  = @{ BE_PORT = 8084; FE_PORT = 5177; IDP_CLIENT_ID = 'NSW_AGENCY_PORTAL_APP_CDA';  NSW_CLIENT_ID = 'CDA_TO_NSW';  APP_NAME = 'Coconut Development Authority (CDA)';       OU_HANDLE = 'cda'  }
}

$ALL_AGENCIES = $AGENCY_CONFIGS.Keys | Sort-Object

function Show-Usage {
    $agencies = ($ALL_AGENCIES -join ', ') + ', all'
    Write-Host @"
Usage: .\start-dev.ps1 [--clean-run] [--env-file=PATH] <agency> [target]

  <agency>  One of: $agencies
  [target]  One of: all (default), backend, frontend

Flags:
  --clean-run       Wipe agency DB(s) before starting
  --env-file=PATH   Load a root-level env file (non-clobbering);
  --env-file PATH   both forms are supported

Examples:
  .\start-dev.ps1 npqs              # NPQS backend + frontend
  .\start-dev.ps1 fcau backend      # FCAU backend only
  .\start-dev.ps1 all               # every agency, backends + frontends
  .\start-dev.ps1 all frontend      # every agency, frontends only
  .\start-dev.ps1 all --clean-run   # wipe all agency DBs, then start
"@ -ForegroundColor Yellow
    exit 1
}

# Parse flags and positional args manually to support kebab-case flags
# (PowerShell param() does not allow hyphens in parameter names).
$CLEAN_RUN  = $false
$ENV_FILE   = ''
$POSITIONAL = @()

$i = 0
while ($i -lt $args.Count) {
    $arg = $args[$i]
    if ($arg -eq '--clean-run') {
        $CLEAN_RUN = $true
    } elseif ($arg -like '--env-file=*') {
        $ENV_FILE = $arg.Substring('--env-file='.Length)
    } elseif ($arg -eq '--env-file') {
        $i++
        if ($i -ge $args.Count -or $args[$i] -like '--*') {
            Write-Host "[start-dev] Error: --env-file requires a path value." -ForegroundColor Red
            exit 1
        }
        $ENV_FILE = $args[$i]
    } else {
        $POSITIONAL += $arg
    }
    $i++
}

$Agency = if ($POSITIONAL.Count -gt 0) { $POSITIONAL[0] } else { '' }
$Target = if ($POSITIONAL.Count -gt 1) { $POSITIONAL[1] } else { 'all' }

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

$jobs = [System.Collections.Generic.List[System.Diagnostics.Process]]::new()

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

# Source a .env file without clobbering vars already set in the env block.
# Preserves parent-shell overrides (e.g. $env:PORT=9000; .\start-dev.ps1 npqs).
function Merge-EnvFile {
    param([string]$Path, $Block)
    if (-not (Test-Path $Path)) { return }
    foreach ($line in Get-Content $Path) {
        if ($line -match '^\s*$' -or $line -match '^\s*#') { continue }
        if ($line -notmatch '=') { continue }
        $line = $line -replace '^export\s+', ''
        $idx  = $line.IndexOf('=')
        $key  = $line.Substring(0, $idx).Trim()
        $val  = $line.Substring($idx + 1).Trim() -replace '^"(.*)"$', '$1' -replace "^'(.*)'$", '$1'
        $val  = ($val -split '\s+#')[0].Trim()
        if ($key -notmatch '^[A-Za-z_][A-Za-z0-9_]*$') { continue }
        if (-not $Block.Contains($key)) { $Block[$key] = $val }
    }
}

function Clean-Databases {
    param([string[]]$Agencies)
    # Build env: parent shell > --env-file > backend/.env (highest to lowest).
    $dbEnv = [System.Environment]::GetEnvironmentVariables('Process').Clone()
    if ($ENV_FILE -ne '') { Merge-EnvFile -Path $ENV_FILE -Block $dbEnv }
    # Capture explicit DB_NAME from parent + --env-file before .env fills in defaults.
    # backend/.env's DB_NAME is excluded so per-agency names apply by default.
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
        if (-not (Get-Command 'psql' -ErrorAction SilentlyContinue)) {
            Write-Host "[start-dev] Error: psql required for Postgres DB cleaning but not found in PATH." -ForegroundColor Red
            exit 1
        }
        $dbHost     = if ($dbEnv.Contains('DB_HOST'))     { $dbEnv['DB_HOST'] }     else { 'localhost' }
        $dbPort     = if ($dbEnv.Contains('DB_PORT'))     { $dbEnv['DB_PORT'] }     else { '5432' }
        $dbUser     = if ($dbEnv.Contains('DB_USER'))     { $dbEnv['DB_USER'] }     else { 'postgres' }
        $dbPassword = if ($dbEnv.Contains('DB_PASSWORD')) { $dbEnv['DB_PASSWORD'] } else { 'changeme' }
        # Postgres uses per-agency databases to avoid concurrent migration conflicts.
        # If DB_NAME is set in parent env or --env-file, treat it as a shared-DB override.
        $env:PGPASSWORD = $dbPassword
        foreach ($agency in $Agencies) {
            $dbName = if ($explicitDbName) { $explicitDbName } else { "${agency}_nsw_agency_db" }
            Write-Host "[start-dev]   Dropping and recreating Postgres database: $dbName"
            psql -h $dbHost -p $dbPort -U $dbUser -d postgres -c "SELECT pg_terminate_backend(pid) FROM pg_stat_activity WHERE datname = '$dbName' AND pid <> pg_backend_pid();" | Out-Null
            if ($LASTEXITCODE -ne 0) {
                Write-Host "[start-dev] Error: Failed to terminate connections to $dbName" -ForegroundColor Red
                Remove-Item Env:PGPASSWORD -ErrorAction SilentlyContinue; exit $LASTEXITCODE
            }
            psql -h $dbHost -p $dbPort -U $dbUser -d postgres -c "DROP DATABASE IF EXISTS `"$dbName`";"
            if ($LASTEXITCODE -ne 0) {
                Write-Host "[start-dev] Error: Failed to drop Postgres database $dbName" -ForegroundColor Red
                Remove-Item Env:PGPASSWORD -ErrorAction SilentlyContinue; exit $LASTEXITCODE
            }
            psql -h $dbHost -p $dbPort -U $dbUser -d postgres -c "CREATE DATABASE `"$dbName`";"
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
    if (Test-Path $FilePath) { return }
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

function Ensure-NodeModules {
    $ModulesMeta = Join-Path $FRONTEND_DIR 'node_modules/.modules.yaml'
    if (Test-Path $ModulesMeta) { return }
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

function Start-Backend {
    param([string]$AgencyName)
    $cfg         = $AGENCY_CONFIGS[$AgencyName]
    $bePort      = $cfg.BE_PORT
    $nswClientId = $cfg.NSW_CLIENT_ID

    Write-Host "[start-dev] Starting $AgencyName backend  -> http://localhost:$bePort"

    $envBlock = [System.Environment]::GetEnvironmentVariables('Process').Clone()
    if ($ENV_FILE -ne '') { Merge-EnvFile -Path $ENV_FILE -Block $envBlock }

    # Per-agency values set before .env so .env cannot override them.
    # Final precedence: parent env > --env-file > per-agency defaults > .env > script fallback.
    if (-not $envBlock.Contains('PORT'))          { $envBlock['PORT']          = "$bePort"                         }
    if (-not $envBlock.Contains('DB_PATH'))       { $envBlock['DB_PATH']       = "./${AgencyName}_applications.db" }
    if (-not $envBlock.Contains('DB_NAME'))       { $envBlock['DB_NAME']       = "${AgencyName}_nsw_agency_db"     }
    if (-not $envBlock.Contains('NSW_CLIENT_ID')) { $envBlock['NSW_CLIENT_ID'] = $nswClientId                      }

    $dotEnv = Join-Path $BACKEND_DIR '.env'
    if (Test-Path $dotEnv) {
        Merge-EnvFile -Path $dotEnv -Block $envBlock
    } else {
        Write-Host "[start-dev] WARNING: backend/.env not found - backend will fail if NSW_* vars are unset." -ForegroundColor Yellow
    }

    # DB_DRIVER falls back to sqlite only if not set by parent env, --env-file, or .env.
    if (-not $envBlock.Contains('DB_DRIVER')) { $envBlock['DB_DRIVER'] = 'sqlite' }

    $psi = [System.Diagnostics.ProcessStartInfo]::new('cmd.exe', '/c go run ./cmd/server')
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
    if ($ENV_FILE -ne '') { Merge-EnvFile -Path $ENV_FILE -Block $envBlock }

    if (-not $envBlock.Contains('VITE_PORT'))                   { $envBlock['VITE_PORT']                  = "$fePort"                  }
    if (-not $envBlock.Contains('VITE_BRANDING_NAME'))          { $envBlock['VITE_BRANDING_NAME']          = $AgencyName               }
    if (-not $envBlock.Contains('VITE_API_BASE_URL'))           { $envBlock['VITE_API_BASE_URL']           = "http://localhost:$bePort" }
    if (-not $envBlock.Contains('VITE_IDP_BASE_URL'))           { $envBlock['VITE_IDP_BASE_URL']           = $IDP_BASE_URL             }
    if (-not $envBlock.Contains('VITE_IDP_CLIENT_ID'))          { $envBlock['VITE_IDP_CLIENT_ID']          = $idpClient                }
    if (-not $envBlock.Contains('VITE_IDP_SCOPES'))             { $envBlock['VITE_IDP_SCOPES']             = 'openid,profile,email,ou' }
    if (-not $envBlock.Contains('VITE_IDP_EXPECTED_OU_HANDLE')) { $envBlock['VITE_IDP_EXPECTED_OU_HANDLE'] = $ouHandle                 }
    if (-not $envBlock.Contains('VITE_APP_URL'))                { $envBlock['VITE_APP_URL']                = "http://localhost:$fePort" }

    $psi = [System.Diagnostics.ProcessStartInfo]::new('cmd.exe', '/c pnpm run dev')
    $psi.WorkingDirectory = $FRONTEND_DIR
    $psi.UseShellExecute  = $false
    foreach ($k in $envBlock.Keys) { $psi.Environment[$k] = [string]$envBlock[$k] }
    $jobs.Add([System.Diagnostics.Process]::Start($psi))
}

# Load optional root-level env file before per-agency defaults.
if ($ENV_FILE -ne '') {
    if (-not (Test-Path $ENV_FILE)) {
        Write-Host "[start-dev] Error: --env-file not found: $ENV_FILE" -ForegroundColor Red
        exit 1
    }
}

# Resolve the agency list to launch.
if ($Agency -eq 'all') {
    $agencyList = $ALL_AGENCIES
} else {
    $agencyList = @($Agency)
}

if ($CLEAN_RUN) {
    Clean-Databases -Agencies $agencyList
}

try {
    if ($Target -eq 'all' -or $Target -eq 'frontend') { Ensure-NodeModules }

    foreach ($a in $agencyList) {
        if ($Target -eq 'all' -or $Target -eq 'backend')  { Start-Backend  $a }
        if ($Target -eq 'all' -or $Target -eq 'frontend') { Start-Frontend $a }
    }

    Write-Host "[start-dev] $($jobs.Count) process(es) running. Logs from all processes will interleave below. Press Ctrl-C to stop."

    # Mirror bash 'wait': keep running until Ctrl-C even if individual processes crash.
    # Log exits as they happen but do not kill the remaining processes.
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
