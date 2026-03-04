# Cleanup script for legacy Error Tracking resources.
# Intended for local/test environments after one-shot migration to trace-only observability.
#
# Usage examples:
#   ./scripts/cleanup_legacy_error_tracking.ps1 -RedisCli "redis-cli" -MySqlCli "mysql" -MySqlDB "meta_assist"
#   ./scripts/cleanup_legacy_error_tracking.ps1 -DryRun

param(
    [string]$RedisCli = "redis-cli",
    [string]$RedisHost = "",
    [int]$RedisPort = 6379,
    [string]$RedisPassword = "",
    [string]$MySqlCli = "mysql",
    [string]$MySqlHost = "127.0.0.1",
    [int]$MySqlPort = 3306,
    [string]$MySqlUser = "root",
    [string]$MySqlPassword = "",
    [string]$MySqlDB = "",
    [switch]$DryRun
)

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

function Build-RedisBaseArgs {
    $args = @()
    if ($RedisHost -ne "") {
        $args += "-h"
        $args += $RedisHost
    }
    if ($RedisPort -gt 0) {
        $args += "-p"
        $args += "$RedisPort"
    }
    if ($RedisPassword -ne "") {
        $args += "-a"
        $args += $RedisPassword
    }
    return $args
}

function Invoke-RedisCommand {
    param(
        [string[]]$Args
    )
    $baseArgs = Build-RedisBaseArgs
    $allArgs = @($baseArgs + $Args)
    if ($DryRun) {
        Write-Host "[DryRun][Redis] $RedisCli $($allArgs -join ' ')"
        return ""
    }
    return & $RedisCli @allArgs
}

function Cleanup-Redis {
    Write-Host "Cleaning Redis legacy keys..."
    Invoke-RedisCommand -Args @("DEL", "errors:stream") | Out-Null

    $scanArgs = Build-RedisBaseArgs
    $scanArgs += "--scan"
    $scanArgs += "--pattern"
    $scanArgs += "err:trace:*"
    if ($DryRun) {
        Write-Host "[DryRun][Redis] $RedisCli $($scanArgs -join ' ')"
        return
    }

    $keys = & $RedisCli @scanArgs
    foreach ($key in $keys) {
        if ([string]::IsNullOrWhiteSpace($key)) {
            continue
        }
        Invoke-RedisCommand -Args @("DEL", $key.Trim()) | Out-Null
    }
}

function Invoke-MySql {
    param(
        [string]$Sql
    )

    if ([string]::IsNullOrWhiteSpace($MySqlDB)) {
        Write-Host "MySqlDB is empty, skip MySQL cleanup."
        return
    }

    $args = @("-h", $MySqlHost, "-P", "$MySqlPort", "-u", $MySqlUser)
    if ($MySqlPassword -ne "") {
        $args += "-p$MySqlPassword"
    }
    $args += $MySqlDB
    $args += "-e"
    $args += $Sql

    if ($DryRun) {
        Write-Host "[DryRun][MySQL] $MySqlCli $($args -join ' ')"
        return
    }
    & $MySqlCli @args | Out-Null
}

function Cleanup-MySql {
    Write-Host "Cleaning MySQL legacy table..."
    Invoke-MySql -Sql "DROP TABLE IF EXISTS observability_error_events;"
}

Write-Host "Starting legacy Error Tracking cleanup..."
Cleanup-Redis
Cleanup-MySql
Write-Host "Legacy Error Tracking cleanup completed."
