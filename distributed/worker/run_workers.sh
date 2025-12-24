#!/bin/bash

./kill_workers.sh

# Array of ports to run workers on
PORTS=(8040 8050 8060 8070 8080 8090 9030 9040)

# Run each worker in the background
for PORT in "${PORTS[@]}"; do
    echo "Starting worker on port :$PORT"
    nohup go run worker.go -port=":$PORT" > "worker_$PORT.log" 2>&1 &
done

echo "All workers started."
