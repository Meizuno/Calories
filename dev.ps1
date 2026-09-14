<#
.SYNOPSIS
  Runs both dev processes with live reload, in one terminal.

.DESCRIPTION
  server/  air     — rebuilds and restarts the Go API on every .go/.sql/.env save
  client/  vite    — hot-reloads the SPA in the browser

  Output from both is merged and prefixed. Ctrl+C stops both.

  Use http://localhost:5173 — the Vite proxy puts /api on the same origin, which
  the session cookies depend on. Port 8080 serves the API alone.

.EXAMPLE
  .\dev.ps1
  .\dev.ps1 -SkipClient     # API only
#>
[CmdletBinding()]
param(
    [switch]$SkipClient,
    [switch]$SkipServer
)

$ErrorActionPreference = 'Stop'
$root = $PSScriptRoot
$procs = @()
$subs = @()

function Assert-Tool($name, $hint) {
    if (-not (Get-Command $name -ErrorAction SilentlyContinue)) {
        throw "$name not found on PATH. $hint"
    }
}

# Starts a child process with its output piped back here, tagged so the two
# streams stay tellable apart once merged.
function Start-Dev($Tag, $Color, $File, $Arguments, $WorkDir) {
    $psi = [System.Diagnostics.ProcessStartInfo]::new()
    $psi.FileName               = $File
    $psi.Arguments              = $Arguments
    $psi.WorkingDirectory       = $WorkDir
    $psi.UseShellExecute        = $false
    $psi.CreateNoWindow         = $true
    $psi.RedirectStandardOutput = $true
    $psi.RedirectStandardError  = $true
    # Vite and air both colourise only when they believe a terminal is attached;
    # redirection hides that, so ask for colour explicitly.
    $psi.EnvironmentVariables['FORCE_COLOR'] = '1'

    $proc = [System.Diagnostics.Process]::new()
    $proc.StartInfo = $psi
    $proc.EnableRaisingEvents = $true

    $onData = {
        if ($null -ne $EventArgs.Data) {
            $t = $Event.MessageData.Tag
            $c = $Event.MessageData.Color
            Write-Host "$t " -ForegroundColor $c -NoNewline
            Write-Host $EventArgs.Data
        }
    }
    $data = [pscustomobject]@{ Tag = $Tag; Color = $Color }
    $script:subs += Register-ObjectEvent -InputObject $proc -EventName OutputDataReceived -Action $onData -MessageData $data
    $script:subs += Register-ObjectEvent -InputObject $proc -EventName ErrorDataReceived  -Action $onData -MessageData $data

    [void]$proc.Start()
    $proc.BeginOutputReadLine()
    $proc.BeginErrorReadLine()
    $script:procs += $proc
    return $proc
}

try {
    if (-not $SkipServer) {
        Assert-Tool 'air' 'Install it with: go install github.com/air-verse/air@latest'
        Write-Host 'server  -> http://localhost:8080  (air: rebuilds on save)' -ForegroundColor DarkGray
        Start-Dev -Tag '[server]' -Color Cyan -File 'air' -Arguments '' -WorkDir (Join-Path $root 'server') | Out-Null
    }
    if (-not $SkipClient) {
        Assert-Tool 'pnpm' 'Install pnpm: https://pnpm.io/installation'
        Write-Host 'client  -> http://localhost:5173  (vite: hot reload)  <- open this one' -ForegroundColor DarkGray
        Start-Dev -Tag '[client]' -Color Magenta -File 'pnpm.cmd' -Arguments 'dev' -WorkDir (Join-Path $root 'client') | Out-Null
    }
    if ($procs.Count -eq 0) { throw 'Nothing to run.' }

    Write-Host 'Ctrl+C stops both.' -ForegroundColor DarkGray
    Write-Host ''

    # Idle here so the output events keep firing. Exits if either child dies, so a
    # crashed server does not leave a half-running stack behind.
    while ($true) {
        Start-Sleep -Milliseconds 250
        foreach ($p in $procs) {
            if ($p.HasExited) {
                Write-Host ''
                Write-Host "a process exited (code $($p.ExitCode)) — shutting down" -ForegroundColor Yellow
                return
            }
        }
    }
}
finally {
    foreach ($p in $procs) {
        if ($p -and -not $p.HasExited) {
            # Kill the whole tree: air spawns the compiled binary, pnpm spawns vite,
            # and killing only the parent would strand the child holding the port.
            & taskkill.exe /PID $p.Id /T /F 2>&1 | Out-Null
        }
    }
    foreach ($s in $subs) {
        if ($s) { Unregister-Event -SubscriptionId $s.Id -ErrorAction SilentlyContinue }
    }
    Write-Host 'dev servers stopped.' -ForegroundColor DarkGray
}
