#!/usr/bin/env bash
# Runs the whole benchmark on one Android device: the operation suite, then a
# fill + cold-open pair of app launches for every store. Each cold open runs in
# its own fresh process, so its open time and memory are not affected by the
# other stores.
#
# Usage: tool/run_all.sh <device-id> [store ...]
# Set SUITE=0 to run only the cold-start launches.
# Output: build/bench/<run>.log for every launch and build/bench/results.tsv.
set -euo pipefail

device=${1:?usage: tool/run_all.sh <device-id> [store ...]}
shift
stores=("$@")
if [ ${#stores[@]} -eq 0 ]; then
  stores=(rodinia hive_ce shared_preferences sqflite get_storage flutter_secure_storage)
fi

cd "$(dirname "$0")/.."
out=build/bench
mkdir -p "$out"
results=$out/results.tsv
printf 'store\tmetric\tvalue\n' > "$results"

timeout_before=$(adb -s "$device" shell settings get system screen_off_timeout | tr -d '\r')
restore() { adb -s "$device" shell settings put system screen_off_timeout "$timeout_before" || true; }
trap restore EXIT
adb -s "$device" shell settings put system screen_off_timeout 1800000

# `flutter drive` uninstalls the app when it finishes, which deletes its data,
# unless --keep-app-running is passed. Fill launches pass it so the data is
# still there for the open launch that follows.
launch() {
  local name=$1 define=$2 keep=${3:-}
  adb -s "$device" shell am force-stop dev.rodinia.rodinia_benchmarks
  adb -s "$device" shell input keyevent KEYCODE_WAKEUP
  adb -s "$device" shell wm dismiss-keyguard
  echo "== $name"
  flutter drive --profile -d "$device" \
    --dart-define=BENCH="$define" $keep \
    --driver=test_driver/integration_test.dart \
    --target=integration_test/benchmark_test.dart > "$out/$name.log" 2>&1 &
  local pid=$!
  while kill -0 "$pid" 2>/dev/null; do
    local free_kb
    free_kb=$(df -k / | awk 'NR==2 {print $4}')
    if [ "$free_kb" -lt 307200 ]; then
      echo "free disk below 300 MB, stopping $name" >&2
      pkill -P "$pid" || true
      kill "$pid" || true
      exit 1
    fi
    sleep 5
  done
  if ! wait "$pid" || ! grep -q 'All tests passed' "$out/$name.log"; then
    echo "$name failed, see $out/$name.log" >&2
    exit 1
  fi
  awk -F'\t' '/RESULT\t/ {print $2 "\t" $3 "\t" $4}' "$out/$name.log" >> "$results"
  # The Gradle daemon keeps growing across builds; on a small machine its
  # memory ends up in swap and fills the disk over a long run.
  (cd android && ./gradlew --stop > /dev/null 2>&1) || true
}

if [ "${SUITE:-1}" != 0 ]; then launch suite suite; fi
for store in "${stores[@]}"; do
  launch "fill-$store" "fill:$store" --keep-app-running
  launch "open-$store" "open:$store"
done

echo "results: $results"
