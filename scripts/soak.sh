#!/usr/bin/env bash
set -euo pipefail

ORDERS_API=${ORDERS_API:-http://localhost:8082}
WORKER_METRICS=${WORKER_METRICS:-http://localhost:9100/metrics}
JOBS=${JOBS:-200}
TIMEOUT_SEC=${TIMEOUT_SEC:-180}

metric() { curl -s "$WORKER_METRICS" | awk -v k="$1" '$1 ~ "^"k {print $2; exit}'; }
completed0=$(metric 'worker_jobs_completed_total{queue="orders-create"}'); completed0=${completed0:-0}
failed0=$(metric 'worker_jobs_failed_total{queue="orders-create"}'); failed0=${failed0:-0}

echo "Enqueueing $JOBS jobs via workers queue..."
pnpm --filter workers run enqueue-orders -- --total "$JOBS" || echo "enqueue command finished"

echo "Waiting for completion (timeout ${TIMEOUT_SEC}s)..."
start=$(date +%s)
while :; do
  completed=$(metric 'worker_jobs_completed_total{queue="orders-create"}'); completed=${completed:-0}
  failed=$(metric 'worker_jobs_failed_total{queue="orders-create"}'); failed=${failed:-0}
  done_count=$((completed - completed0))
  fail_count=$((failed - failed0))
  echo "done=$done_count/$JOBS fail=$fail_count"
  if [ "$done_count" -ge "$JOBS" ]; then break; fi
  now=$(date +%s); if [ $((now - start)) -gt "$TIMEOUT_SEC" ]; then echo "Timeout waiting for jobs"; exit 1; fi
  sleep 2
done

echo "Verifying pagination and totals..."
curl -sf "$ORDERS_API/v1/orders?limit=1&offset=0" | jq -e '.total' >/dev/null

for id in $(curl -s "$ORDERS_API/v1/orders?limit=5&offset=0" | jq -r '.data[].id'); do
  json=$(curl -s "$ORDERS_API/v1/orders/$id")
  order_total=$(echo "$json" | jq -r '.total_price')
  sum_items=$(echo "$json" | jq -r '[.items[].total_price|tonumber]|add|tostring')
  if [ "$order_total" != "$sum_items" ]; then echo "Mismatch $id $order_total != $sum_items"; exit 1; fi
done

echo "Soak OK"