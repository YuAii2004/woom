$ErrorActionPreference = 'Stop'

$live777Version = 'v0.7.3'
$live777Url = "https://github.com/binbat/live777/releases/download/$live777Version/live777-v0.7.3-x86_64-pc-windows-msvc.zip"
$redisUrl = 'https://github.com/tporadowski/redis/releases/download/v5.0.14.1/Redis-x64-5.0.14.1.zip'
$workDir = Join-Path $env:RUNNER_TEMP 'woom-native-deps'

New-Item -ItemType Directory -Force -Path $workDir | Out-Null

$redisArchive = Join-Path $workDir 'redis.zip'
$live777Archive = Join-Path $workDir 'live777.zip'
$redisExtract = Join-Path $workDir 'redis'
$live777Extract = Join-Path $workDir 'live777'

Invoke-WebRequest -Uri $redisUrl -OutFile $redisArchive
Expand-Archive -Path $redisArchive -DestinationPath $redisExtract -Force

Invoke-WebRequest -Uri $live777Url -OutFile $live777Archive
Expand-Archive -Path $live777Archive -DestinationPath $live777Extract -Force

$redisServer = Get-ChildItem -Path $redisExtract -Filter 'redis-server.exe' -Recurse | Select-Object -First 1
$redisCli = Get-ChildItem -Path $redisExtract -Filter 'redis-cli.exe' -Recurse | Select-Object -First 1
$live777Binary = Get-ChildItem -Path $live777Extract -Filter 'live777.exe' -Recurse | Select-Object -First 1

$redisProcess = Start-Process -FilePath $redisServer.FullName -ArgumentList '--port 6379 --bind 127.0.0.1' -WorkingDirectory $redisServer.DirectoryName -RedirectStandardOutput (Join-Path $workDir 'redis.log') -RedirectStandardError (Join-Path $workDir 'redis-error.log') -PassThru
$live777Process = Start-Process -FilePath $live777Binary.FullName -WorkingDirectory $live777Binary.DirectoryName -RedirectStandardOutput (Join-Path $workDir 'live777.log') -RedirectStandardError (Join-Path $workDir 'live777-error.log') -PassThru

$redisProcess.Id | Set-Content (Join-Path $workDir 'redis.pid')
$live777Process.Id | Set-Content (Join-Path $workDir 'live777.pid')

$deadline = (Get-Date).AddMinutes(2)
do {
  $redisReady = (& $redisCli.FullName -h 127.0.0.1 -p 6379 ping 2>$null) -contains 'PONG'
  if (-not $redisReady) { Start-Sleep -Seconds 2 }
} while (-not $redisReady -and (Get-Date) -lt $deadline)
if (-not $redisReady) { throw 'Redis did not become ready in time' }

$live777Ready = $false
do {
  try {
    Invoke-WebRequest -Uri 'http://127.0.0.1:7777' -TimeoutSec 2 -UseBasicParsing | Out-Null
    $live777Ready = $true
  } catch {
    Start-Sleep -Seconds 2
  }
} while (-not $live777Ready -and (Get-Date) -lt $deadline)
if (-not $live777Ready) { throw 'Live777 did not become ready in time' }
