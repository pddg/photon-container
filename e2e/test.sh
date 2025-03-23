#!/bin/bash

set -e

function log() {
    echo ""
    echo "e2e: $1"
    echo ""
}

log "Starting e2e tests"

docker compose up --build -d --wait

log "Wait until the Photon is up and running"
for i in $(seq 1 20)
do
    state=$(curl -s "http://localhost:8080/migrate/status" | jq -r '.state')
    if [ "$state" == "unknown" ]; then
        break
    fi
    sleep 1
done

log "Photon is up and running"

log "Download and unarchive database of Photon"
curl -i -X POST "http://localhost:8080/migrate/download?latest=true"

log "Wait until the database is downloaded and unarchived"
for i in $(seq 1 20)
do
    state=$(curl -sS "http://localhost:8080/migrate/status" | jq -r '.state')
    if [ "$state" == "migrated" ]; then
        break
    fi
    sleep 1
done

log "Database is downloaded and unarchived. Wait until the Photon is ready for testing"
for i in $(seq 1 20)
do
    state=$(curl -sS "http://localhost:2322/status" | jq -r '.status')
    if [ "$state" == "Ok" ]; then
        break
    fi
    sleep 1
done

log "Photon is now ready for testing. Use the reverse geocoding endpoint to test it"
FEATURES=$(curl -sS -X GET 'http://localhost:2322/reverse?lat=42.508004&lon=1.529161' | jq -r '.features')
if [ $(echo "$FEATURES" | jq 'length') -eq 0 ]; then
    log "Test failed"
    exit 1
fi
log Found features:
echo "$FEATURES"
log "Test passed"
