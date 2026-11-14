# Bounded side-cut product decoder fuzzing

`FuzzReadSideCutProducts` exercises the actual strict typed equal, between-center
unequal and external-radical general STEP readers. It does not substitute parser-
only validation or expand geometry acceptance. There are 38 initial seeds:
blind/through output from each of the three registered writers, wrong-category
inputs, truncations, invalid prefix/NUL payloads, trailing out-of-file records,
invalid terminators and small malformed inputs.

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
readback stability. Separate ordinary tests prove the six valid seeds fit these
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
