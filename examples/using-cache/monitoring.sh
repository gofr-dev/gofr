#!/bin/bash
# This script starts the monitoring stack (Prometheus and Grafana)
# for the cache example.

# Get the directory of the script
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" &> /dev/null && pwd)"

# Navigate to the monitoring directory and start docker-compose
cd "${SCRIPT_DIR}/../../pkg/cache/monitoring" && docker-compose up --build