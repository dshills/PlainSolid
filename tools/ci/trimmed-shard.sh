#!/usr/bin/env bash
# Keep the existing trimmed-solid shard command.
exec bash "$(dirname "$0")/test-shard.sh" ./brep/trimmed "$@"
