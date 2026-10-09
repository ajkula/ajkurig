#!/bin/sh
cd "$(dirname "$0")/../.." && exec go run ./cmd/release -config examples/discs/release.json "$@"
