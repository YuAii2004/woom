#!/usr/bin/env bash

set -euo pipefail

work_dir="${RUNNER_TEMP}/woom-native-deps"

for pid_file in "${work_dir}/live777.pid" "${work_dir}/redis.pid"; do
  if [[ -f "${pid_file}" ]]; then
    kill "$(cat "${pid_file}")" 2>/dev/null || true
  fi
done
