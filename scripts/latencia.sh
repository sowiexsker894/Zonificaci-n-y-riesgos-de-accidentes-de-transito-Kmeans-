#!/usr/bin/env bash
# Mide la latencia de POST /api/predict (N peticiones) y reporta p50/p95/máx en ms.
B=${BASE:-http://localhost:8080}; N=${1:-500}
for i in $(seq 1 "$N"); do
  curl -s -o /dev/null -w "%{time_total}\n" -X POST "$B/api/predict" -d '{"lat":-12.0464,"lon":-77.0428,"hora":18}'
done | sort -n | awk -v n="$N" '{a[NR]=$1} END{printf "N=%d  p50=%.2f ms  p95=%.2f ms  p99=%.2f ms  max=%.2f ms  -> %s\n", n, a[int(NR*.5)]*1000, a[int(NR*.95)]*1000, a[int(NR*.99)]*1000, a[NR]*1000, (a[int(NR*.99)]*1000<100?"CUMPLE <100 ms":"NO CUMPLE")}'
