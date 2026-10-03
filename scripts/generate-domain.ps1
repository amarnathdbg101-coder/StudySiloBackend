param (
    [Parameter(Mandatory=$true, Position=0)]
    [string]$DomainName
)

$Pascal = $DomainName.Substring(0,1).ToUpper() + $DomainName.Substring(1)
$Lower = $DomainName.ToLower()
$Plural = $Lower + "s"

$rootDir = Split-Path -Parent $PSScriptRoot
if (!(Test-Path "$rootDir\internal")) {
    $rootDir = "C:\Go\GoProjectStructure"
}

$tplDir = "$PSScriptRoot\templates"
$utf8NoBom = New-Object System.Text.UTF8Encoding $false

Write-Host "⚡ Scaffolding domain: $Pascal..." -ForegroundColor Cyan

function Process-Template($tplName, $targetPath) {
    $content = [System.IO.File]::ReadAllText("$tplDir\$tplName")
    $content = $content.Replace("__PASCAL__", $Pascal).Replace("__LOWER__", $Lower).Replace("__PLURAL__", $Plural)
    [System.IO.File]::WriteAllText($targetPath, $content, $utf8NoBom)
}

# 1. Model
$modelFile = "$rootDir\internal\handler\model\$Lower.go"
Process-Template "model.go.tpl" $modelFile
Write-Host "  [+] Created Model: $modelFile" -ForegroundColor Green

# 2. DTO
$dtoFile = "$rootDir\internal\handler\dto\${Lower}_dto.go"
Process-Template "dto.go.tpl" $dtoFile
Write-Host "  [+] Created DTO: $dtoFile" -ForegroundColor Green

# 3. Repository
$repoFile = "$rootDir\internal\handler\repository\${Lower}_repository.go"
Process-Template "repository.go.tpl" $repoFile
Write-Host "  [+] Created Repository: $repoFile" -ForegroundColor Green

# 4. Service
$serviceFile = "$rootDir\internal\handler\services\${Lower}_service.go"
Process-Template "services.go.tpl" $serviceFile
Write-Host "  [+] Created Service: $serviceFile" -ForegroundColor Green

# 5. Controller
$controllerFile = "$rootDir\internal\handler\controller\${Lower}_controller.go"
Process-Template "controller.go.tpl" $controllerFile
Write-Host "  [+] Created Controller: $controllerFile" -ForegroundColor Green

# 6. Migrations
$timestamp = (Get-Date).ToString("yyyyMMddHHmmss")
$upMig = "$rootDir\internal\migration\${timestamp}_create_${Plural}_table.up.sql"
$downMig = "$rootDir\internal\migration\${timestamp}_create_${Plural}_table.down.sql"
Process-Template "migration_up.sql.tpl" $upMig
Process-Template "migration_down.sql.tpl" $downMig
Write-Host "  [+] Created Migrations: $upMig" -ForegroundColor Green

Write-Host ""
Write-Host "🚀 Domain $Pascal created successfully!" -ForegroundColor Green
Write-Host "👉 Wire it in internal/handler/routes/router.go in 4 lines:" -ForegroundColor Yellow
Write-Host "    ${Lower}Repo := repository.New${Pascal}Repository(db)" -ForegroundColor Cyan
Write-Host "    ${Lower}Service := services.New${Pascal}Service(${Lower}Repo)" -ForegroundColor Cyan
Write-Host "    ${Lower}Ctrl := controller.New${Pascal}Controller(${Lower}Service)" -ForegroundColor Cyan
Write-Host ""
Write-Host "    r.Route(`/api/v1/$Plural`, func(r chi.Router) {" -ForegroundColor Cyan
Write-Host "        r.Get(`/`, ${Lower}Ctrl.List)" -ForegroundColor Cyan
Write-Host "        r.Get(`/{id}`, ${Lower}Ctrl.GetByID)" -ForegroundColor Cyan
Write-Host "        r.Post(`/`, ${Lower}Ctrl.Create)" -ForegroundColor Cyan
Write-Host "        r.Delete(`/{id}`, ${Lower}Ctrl.Delete)" -ForegroundColor Cyan
Write-Host "    })" -ForegroundColor Cyan
