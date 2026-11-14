# Independent unit triage

Source: PlainCAD 50432ed; original report a1b8ced. No app, integration worktree, or kernel edits.

Two isolated one-worker real-WASM selections in `/tmp/plainsolid-unit-triage` ran 53 original tests: 49 passed, 4 failed. Reports: `/tmp/plaincad-unit-triage-results.json` (38/40), `/tmp/plaincad-unit-triage-secondary.json` (11/13). Original full run remains 1630/1689, not superseded.

## Repeatable failures examined

- featureGraph fallback join: expects `rebuildDocument` to fail because no OCCT handles exist. With the real WASM provider it succeeds. This is an explicit legacy fallback fixture, not evidence of a Boolean defect.
- featureGraph fallback edge-treatment: expects fallback-specific diagnostics after constructing the default provider, now real WASM. The negative fallback fixture must inject a genuine unsupported/fallback adapter independently of the native suite.
- nativeSketchPlanes: the failing line171 expects fallback-only `modified` diagnostics. Actual native rebuilding has different errors; the preceding source-level proof cases pass. This does not establish native face acceptance is unsafe.
- manufacturingCoach: helper rebuilds actual stock, then relabels/clones meshes and overwrites mass assertions. It used to inherit no native availableFaces; real rebuilding supplies proven start/end caps. The test expects three unverified stocks, receives one. A bounded-dimensions-only negative fixture must explicitly omit native face evidence; do not remove evidence from the kernel to satisfy it.

## Recovered unchanged in isolation

Suppressed extrusion, all placed XY/XZ circle projection tests, all measurement tests, and nativeFeatureSignature pass. The full-run failures here are not reproduced; load-sensitive sketch solve budgets are a plausible cause, not proved for every case.

## Other evidence classes (read-only)

Six nativeStep failures dereference OCCT-specific `.FS` or `BRepPrimAPI_MakeBox_1`; three multiBodyHoles failures spy on private `unionSolids`. Those require separate provider-contract equivalents, not emulated OCCT internals. nativeEdgeProofCache fabricates an OCCT-shaped private handle rather than obtaining a provider-issued shape. Numerous repeated negative tests expect an uninitialized default constructor to produce fallback; they need explicit fallback injection before judging native behavior.

The three thread modeling failures are 45–61s timeouts under concurrent broad suites. They remain unresolved performance qualification, not established geometry failures. No deadline was changed and no kernel optimization is justified by this triage alone.

## App-only readiness correction proposal

The existing project-open patch fixes original Parts Browser but the independent held-import diagnostic proves an old in-flight native rebuild can still publish `succeeded`. A robust correction should keep pending import ownership as an explicit state invariant: rebuild completion may update drawable result while effective readiness stays queued for the newest open; cancellation restores the most recent same-document result; obsolete opens cannot restore anything. This belongs in app state/publication handling, not native geometry. Existing diagnostic and frozen patch remain at `/tmp/plaincad-parts-readiness-audit.json`.

No concrete repeatable PlainSolid kernel geometry defect was identified in the examined failures, so no speculative kernel patch was made. Remaining unexamined failures and performance gates remain open.
