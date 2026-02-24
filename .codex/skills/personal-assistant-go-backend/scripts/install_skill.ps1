[CmdletBinding()]
param(
    [string]$SourceSkillDir = (Join-Path $PSScriptRoot ".."),
    [string]$CodexHome = "",
    [switch]$NoBackup
)

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

function Resolve-NormalizedPath {
    param([Parameter(Mandatory = $true)][string]$Path)
    return (Resolve-Path -Path $Path -ErrorAction Stop).Path
}

if ([string]::IsNullOrWhiteSpace($CodexHome)) {
    if (-not [string]::IsNullOrWhiteSpace($env:CODEX_HOME)) {
        $CodexHome = $env:CODEX_HOME
    }
    else {
        $CodexHome = Join-Path $HOME ".codex"
    }
}

$sourcePath = Resolve-NormalizedPath -Path $SourceSkillDir
$skillName = Split-Path -Path $sourcePath -Leaf
$skillFile = Join-Path $sourcePath "SKILL.md"

if (-not (Test-Path -Path $skillFile)) {
    throw "在源技能目录中未找到 SKILL.md: $sourcePath"
}

$targetSkillsRoot = Join-Path $CodexHome "skills"
$targetSkillPath = Join-Path $targetSkillsRoot $skillName
New-Item -ItemType Directory -Path $targetSkillsRoot -Force | Out-Null

if (Test-Path -Path $targetSkillPath) {
    if (-not $NoBackup) {
        $stamp = Get-Date -Format "yyyyMMdd_HHmmss"
        $backupPath = "$targetSkillPath.bak.$stamp"
        Copy-Item -Path $targetSkillPath -Destination $backupPath -Recurse -Force
        Write-Host ("[install-skill] 已创建备份: {0}" -f $backupPath)
    }
    Remove-Item -Path $targetSkillPath -Recurse -Force
}

Copy-Item -Path $sourcePath -Destination $targetSkillPath -Recurse -Force

Write-Host ("[install-skill] 安装完成: {0}" -f $skillName)
Write-Host ("[install-skill] 源路径: {0}" -f $sourcePath)
Write-Host ("[install-skill] 目标路径: {0}" -f $targetSkillPath)
