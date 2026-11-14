# Bounded side-cut product decoder fuzzing

`FuzzReadSideCutProducts` exercises the actual strict typed equal, between-center
unequal, external-radical general, exact-wall and offset STEP readers. It does not substitute parser-
only validation or expand geometry acceptance. There are 92 initial seeds:
blind/through output from each of the five registered writers, wrong-category
inputs, truncations, invalid prefix/NUL payloads, trailing out-of-file records,
invalid terminators and small malformed inputs. Every valid seed additionally contributes forged
product-class, model-circle edge-owner and shared cylinder-placement mutations.

The harness uses a fixed work schedule per input: one selected typed read, at
most one accepted-descriptor write, and one readback. It performs no retries,
tessellation, mass integration or external oracle calls. Every read uses explicit
limits: 64 KiB, 768 entities, 8192 parsed values, syntax depth 16, 64 vertices,
24 faces, one solid and 256 edge uses. Inputs larger than the byte bound are
skipped. A shared two-second context bounds cooperative native work across the
three calls; this is not a process-level hard realtime guarantee. Structural
resource limits remain active independently of wall-clock scheduling.

Assertions cover all-or-none failures, complete native shell results, required
source lineage/proof fields, accepted-descriptor re-emission and normalized
readback stability. Separate ordinary tests prove the ten valid seeds fit these
limits, reject category crossings and malformed outer payloads, and check
pre-cancelled calls. Deadline/cancellation during accepted-output checks is an
inconclusive bounded case, not a falsely reported semantic defect.

## Retained bounded run

From the isolated exchange checkout, using the standard Go fuzz engine and no
additional dependencies:

```
GOMAXPROCS=2 GOCACHE=/tmp/plainsolid-go-cache go test ./exchange/step \
  -run '^$' -fuzz '^FuzzReadSideCutProducts$' -fuzztime=30s -parallel=1
GOMAXPROCS=2 GOCACHE=/tmp/plainsolid-go-cache go test ./exchange/step \
  -run '^$' -fuzz '^FuzzReadSideCutProducts$' -fuzztime=100x -parallel=1 -timeout=90s
```

Both passed on 2026-11-13. The time-bounded run reported 39 completed executions
in 32.2 seconds; the count-bounded run completed 100 executions in 17.9 seconds.
All 38 baseline seeds completed, with no new interesting inputs or failing
artifacts reported. These are deliberately small smoke campaigns, not evidence
of exhaustive fuzz coverage or general STEP robustness. Go reports worker batch
completion, so intervals without an updated execution count are not themselves
proof of a stalled decoder.

## Five-family extension

The same fixed resource schedule now applies to all five typed readers; exact-wall
and offset success additionally require source wall IDs, an exact owned radical
join and a single native shell. Ordinary regressions exercise all cross-category
pairs and prove each semantic mutation actually rejects, including blind and
through variants. Corruption changes a genuine parsed dependency, not merely a
string with an unknown entity number. The existing test-only source mutation
helper now accepts `testing.TB` so fuzz seed construction uses the same serializer.

CI adds a count-bounded 100-execution, one-worker smoke with GOMAXPROCS=2 and a
180-second outer test timeout, retaining the per-input two-second cooperative
context. With 92 seeds this is primarily a regression/seed smoke, not a large
mutation campaign. It does not qualify general STEP acceptance.

The expanded count-bounded campaign passed 100 executions in 46.641 seconds:
all 92 baseline seeds completed, one new interesting input and no failing artifact.
Focused semantic/cross-family tests passed in 9.809 seconds; Go 1.23 passed in 8.414 seconds.
