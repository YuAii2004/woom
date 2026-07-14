$ErrorActionPreference = 'SilentlyContinue'

$workDir = Join-Path $env:RUNNER_TEMP 'woom-native-deps'

foreach ($name in @('live777.pid', 'redis.pid')) {
  $pidPath = Join-Path $workDir $name
  if (Test-Path $pidPath) {
    Stop-Process -Id (Get-Content $pidPath) -Force
  }
}
