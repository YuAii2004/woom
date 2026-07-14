#!/usr/bin/env bash

set -euo pipefail

readonly live777_version="v0.7.3"
readonly live777_url="https://github.com/binbat/live777/releases/download/${live777_version}/live777-v0.7.3-aarch64-apple-darwin.tar.gz"
readonly work_dir="${RUNNER_TEMP}/woom-native-deps"

mkdir -p "${work_dir}"

brew install redis
redis_server="$(brew --prefix redis)/bin/redis-server"
redis_cli="$(brew --prefix redis)/bin/redis-cli"
"${redis_server}" --port 6379 --bind 127.0.0.1 >"${work_dir}/redis.log" 2>&1 &
echo "$!" >"${work_dir}/redis.pid"

archive="${work_dir}/live777.tar.gz"
curl --fail --location --retry 3 --output "${archive}" "${live777_url}"
tar -xzf "${archive}" --directory "${work_dir}"
live777_dir="$(find "${work_dir}" -type f -name live777 -print -quit | xargs dirname)"
live777_binary="${live777_dir}/live777"
(
  cd "${live777_dir}"
  nohup "${live777_binary}" >"${work_dir}/live777.log" 2>&1 &
  echo "$!" >"${work_dir}/live777.pid"
)

deadline=$((SECONDS + 120))
until "${redis_cli}" -h 127.0.0.1 -p 6379 ping 2>/dev/null | grep -q PONG; do
  if (( SECONDS >= deadline )); then
    echo "Redis did not become ready in time" >&2
    exit 1
  fi
  sleep 2
done

until curl --silent --output /dev/null --max-time 2 http://127.0.0.1:7777; do
  if (( SECONDS >= deadline )); then
    echo "Live777 did not become ready in time" >&2
    exit 1
  fi
  sleep 2
done
