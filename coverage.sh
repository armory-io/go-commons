#!/bin/bash
echo "Running tests with coverage..."

go test -coverpkg=./... --coverprofile=profile.cov $(go list ./... | grep -v /e2e) -json | tee report.json

exit "${PIPESTATUS[0]}"

