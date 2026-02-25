[CmdletBinding()]
param(
    [string]$RepoRoot = (Join-Path $PSScriptRoot "..\..\..\.."),
    [int]$MaxPerCheck = 20,
    [switch]$FailOnHit
)

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

function Resolve-NormalizedPath {
    param([Parameter(Mandatory = $true)][string]$Path)
    return (Resolve-Path -Path $Path -ErrorAction Stop).Path
}

function Get-RelativeRepoPath {
    param(
        [Parameter(Mandatory = $true)][string]$RepoPath,
        [Parameter(Mandatory = $true)][string]$TargetPath
    )
    $repoRoot = $RepoPath
    if (-not $repoRoot.EndsWith("\") -and -not $repoRoot.EndsWith("/")) {
        $repoRoot = "$repoRoot\"
    }
    $repoUri = New-Object System.Uri($repoRoot)
    $targetUri = New-Object System.Uri($TargetPath)
    $relativeUri = $repoUri.MakeRelativeUri($targetUri)
    return [System.Uri]::UnescapeDataString($relativeUri.ToString()).Replace("\", "/")
}

function Get-GoFiles {
    param(
        [Parameter(Mandatory = $true)][string]$BaseDirectory,
        [string[]]$ExcludeRegex = @()
    )

    if (-not (Test-Path -Path $BaseDirectory)) {
        return @()
    }

    $files = Get-ChildItem -Path $BaseDirectory -Recurse -File -Filter *.go
    if ($ExcludeRegex.Count -eq 0) {
        return $files
    }

    return $files | Where-Object {
        $full = $_.FullName
        foreach ($pattern in $ExcludeRegex) {
            if ($full -match $pattern) {
                return $false
            }
        }
        return $true
    }
}

$repoPath = Resolve-NormalizedPath -Path $RepoRoot

$checks = @(
    @{
        CheckId = "service-direct-db"
        RuleRef = "#1"
        Severity = "高"
        Title = "Service 层不应直接访问 DB"
        Directory = "internal/service"
        Pattern = "global\.DB"
        ExcludeRegex = @()
    },
    @{
        CheckId = "controller-to-repository"
        RuleRef = "#1"
        Severity = "高"
        Title = "Controller 层不应直接访问 Repository"
        Directory = "internal/controller"
        Pattern = "repository\.GroupApp|personal_assistant/internal/repository"
        ExcludeRegex = @()
    },
    @{
        CheckId = "direct-viper-usage"
        RuleRef = "#9"
        Severity = "中"
        Title = "业务代码应避免直接使用 Viper"
        Directory = "."
        Pattern = "github\.com/spf13/viper|\bviper\."
        ExcludeRegex = @(
            "\\internal\\core\\config\.go$",
            "\\internal\\model\\config\\",
            "\\.codex\\"
        )
    },
    @{
        CheckId = "raw-json-response"
        RuleRef = "#3,#10"
        Severity = "中"
        Title = "在要求统一响应时，Controller 应避免直接返回原始 JSON"
        Directory = "internal/controller"
        Pattern = "\.JSON\(|AbortWithStatusJSON\("
        ExcludeRegex = @()
    }
)

$hits = New-Object System.Collections.Generic.List[object]

foreach ($check in $checks) {
    $scanDir = Resolve-NormalizedPath -Path (Join-Path $repoPath $check.Directory)
    $goFiles = Get-GoFiles -BaseDirectory $scanDir -ExcludeRegex $check.ExcludeRegex

    foreach ($file in $goFiles) {
        $matches = Select-String -Path $file.FullName -Pattern $check.Pattern -AllMatches
        foreach ($match in $matches) {
            $snippet = ($match.Line -replace "^\s+|\s+$", "")
            $hits.Add([PSCustomObject]@{
                    CheckId  = $check.CheckId
                    RuleRef  = $check.RuleRef
                    Severity = $check.Severity
                    Title    = $check.Title
                    Path     = Get-RelativeRepoPath -RepoPath $repoPath -TargetPath $file.FullName
                    Line     = $match.LineNumber
                    Snippet  = $snippet
                })
        }
    }
}

Write-Host ("[rule-probe] 仓库根目录: {0}" -f $repoPath)

if ($hits.Count -eq 0) {
    Write-Host "[rule-probe] 未发现命中项。"
    exit 0
}

$grouped = $hits | Group-Object -Property CheckId
$total = 0
foreach ($group in $grouped) {
    $sample = $group.Group | Select-Object -First 1
    $count = $group.Count
    $total += $count
    Write-Host ""
    Write-Host ("[{0}][规则 {1}][{2}] {3} (命中: {4})" -f $sample.CheckId, $sample.RuleRef, $sample.Severity, $sample.Title, $count)

    $shown = 0
    foreach ($item in $group.Group | Select-Object -First $MaxPerCheck) {
        $shown++
        Write-Host ("  - {0}:{1}  {2}" -f $item.Path, $item.Line, $item.Snippet)
    }

    if ($count -gt $shown) {
        Write-Host ("  - ... 另有 {0} 条" -f ($count - $shown))
    }
}

Write-Host ""
Write-Host ("[rule-probe] 总命中数: {0}" -f $total)

if ($FailOnHit) {
    exit 1
}

exit 0
