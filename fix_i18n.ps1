$ErrorActionPreference = "Stop"
$cultures = @("de", "fr", "it", "cs", "pt-BR", "ko", "uk")

foreach ($culture in $cultures) {
    $ourPath = "F:\qui\web\src\i18n\locales\$culture\torrents.json"
    
    # Get upstream version
    $upstreamContent = git show upstream/develop:web/src/i18n/locales/$culture/torrents.json 2>$null
    if (-not $upstreamContent) {
        Write-Host "No upstream for $culture"
        continue
    }
    
    # Parse both as JSON
    $ourJson = Get-Content $ourPath -Raw | ConvertFrom-Json
    $upstreamJson = $upstreamContent | ConvertFrom-Json
    
    # Get manualCrossSeed from upstream
    $manualCrossSeed = $upstreamJson.torrents.manualCrossSeed
    if (-not $manualCrossSeed) {
        Write-Host "No manualCrossSeed in upstream for $culture"
        continue
    }
    
    # Insert manualCrossSeed into our JSON after reportDialog
    $torrents = $ourJson.torrents
    $keys = $torrents.PSObject.Properties.Name
    
    # Find position of reportDialog
    $reportDialogIndex = -1
    for ($j = 0; $j -lt $keys.Count; $j++) {
        if ($keys[$j] -eq "reportDialog") {
            $reportDialogIndex = $j
            break
        }
    }
    if ($reportDialogIndex -eq -1) {
        Write-Host "No reportDialog in $culture"
        continue
    }
    
    # Build new ordered object
    $newTorrents = [ordered]@{}
    for ($j = 0; $j -lt $keys.Count; $j++) {
        $key = $keys[$j]
        $newTorrents[$key] = $torrents.$key
        if ($j -eq $reportDialogIndex) {
            # Insert manualCrossSeed after reportDialog
            $newTorrents["manualCrossSeed"] = $manualCrossSeed
        }
    }
    
    $ourJson.torrents = $newTorrents
    
    # Write back with proper formatting
    $output = $ourJson | ConvertTo-Json -Depth 100
    Set-Content $ourPath -Value $output -Encoding UTF8
    Write-Host "Fixed: $culture"
}