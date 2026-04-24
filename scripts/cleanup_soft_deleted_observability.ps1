# Cleanup script for already soft-deleted observability rows.
#
# Usage examples:
#   ./scripts/cleanup_soft_deleted_observability.ps1 -MySqlCli "mysql" -MySqlDB "meta_assist"
#   ./scripts/cleanup_soft_deleted_observability.ps1 -MySqlHost "127.0.0.1" -MySqlPort 3306 -MySqlUser "root" -MySqlPassword "secret" -MySqlDB "meta_assist" -BatchSize 1000
#   ./scripts/cleanup_soft_deleted_observability.ps1 -DryRun

param(
    [string]$MySqlCli = "mysql",
    [string]$MySqlHost = "127.0.0.1",
    [int]$MySqlPort = 3306,
    [string]$MySqlUser = "root",
    [string]$MySqlPassword = "",
    [string]$MySqlDB = "",
    [int]$BatchSize = 1000,
    [switch]$DryRun
)

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

function New-MySqlArgs {
    param(
        [string]$Sql,
        [switch]$MaskPassword
    )

    $args = @("-h", $MySqlHost, "-P", "$MySqlPort", "-u", $MySqlUser, "-N", "-B")
    if ($MySqlPassword -ne "") {
        if ($MaskPassword) {
            $args += "-p******"
        } else {
            $args += "-p$MySqlPassword"
        }
    }
    $args += $MySqlDB
    $args += "-e"
    $args += $Sql
    return $args
}

function Invoke-MySqlText {
    param(
        [string]$Sql
    )

    if ([string]::IsNullOrWhiteSpace($MySqlDB)) {
        throw "MySqlDB is empty."
    }

    $displayArgs = New-MySqlArgs -Sql $Sql -MaskPassword
    if ($DryRun) {
        Write-Host "[DryRun][MySQL] $MySqlCli $($displayArgs -join ' ')"
        return ""
    }

    $args = New-MySqlArgs -Sql $Sql
    return (& $MySqlCli @args | Out-String).Trim()
}

function Invoke-MySqlInt {
    param(
        [string]$Sql
    )

    $raw = Invoke-MySqlText -Sql $Sql
    if ([string]::IsNullOrWhiteSpace($raw)) {
        return 0
    }
    return [int64]::Parse(($raw -split "`r?`n")[-1].Trim())
}

function Remove-SoftDeletedRowsByTable {
    param(
        [string]$TableName
    )

    Write-Host "Checking table $TableName ..."
    $countSql = "SELECT COUNT(*) FROM $TableName WHERE deleted_at IS NOT NULL;"
    $initialTotal = Invoke-MySqlInt -Sql $countSql

    if ($DryRun) {
        $deleteSql = "DELETE FROM $TableName WHERE deleted_at IS NOT NULL ORDER BY deleted_at ASC, id ASC LIMIT $BatchSize; SELECT ROW_COUNT();"
        Invoke-MySqlText -Sql $deleteSql | Out-Null
        return
    }

    Write-Host "Table $TableName soft-deleted rows: $initialTotal"
    if ($initialTotal -le 0) {
        return
    }

    $remaining = $initialTotal
    $deletedTotal = 0
    $round = 0

    while ($true) {
        $round++
        $deleteSql = "DELETE FROM $TableName WHERE deleted_at IS NOT NULL ORDER BY deleted_at ASC, id ASC LIMIT $BatchSize; SELECT ROW_COUNT();"
        $affected = Invoke-MySqlInt -Sql $deleteSql
        if ($affected -le 0) {
            break
        }

        $deletedTotal += $affected
        $remaining = [Math]::Max(0, $remaining - $affected)
        Write-Host ("[{0}] table={1} deleted={2} deleted_total={3} remaining_estimate={4}" -f $round, $TableName, $affected, $deletedTotal, $remaining)
    }

    $finalRemaining = Invoke-MySqlInt -Sql $countSql
    Write-Host "Table $TableName cleanup completed. deleted_total=$deletedTotal remaining=$finalRemaining"
}

if ($BatchSize -le 0) {
    throw "BatchSize must be greater than 0."
}

if ([string]::IsNullOrWhiteSpace($MySqlDB)) {
    Write-Host "MySqlDB is empty, skip observability cleanup."
    exit 0
}

Write-Host "Starting soft-deleted observability cleanup..."
Remove-SoftDeletedRowsByTable -TableName "observability_trace_spans"
Remove-SoftDeletedRowsByTable -TableName "observability_metrics"
Write-Host "Soft-deleted observability cleanup completed."
