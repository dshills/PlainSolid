# PlainCAD acceptance adapter

This opt-in harness runs PlainCAD's existing browser acceptance fixtures against
PlainSolid compiled to Go WebAssembly. It makes a fresh temporary app copy and
never changes the source PlainCAD checkout. There is no OCCT or faceted fallback
in the replacement adapter.

Run from the PlainSolid checkout (including an integration worktree):

```sh
node tools/plaincad/run.mjs \
  --source /absolute/path/to/PlainCAD \
  --work /tmp/plaincad-plainsolid-run
```

The source app must already have its npm dependencies and Playwright Chromium
installed. The default selection is the three existing XY/XZ/YZ signed and
symmetric extrusion cases in `e2e/extrusion-hole.spec.ts`. To select a different
existing gate:

```sh
node tools/plaincad/run.mjs \
  --source /absolute/path/to/PlainCAD \
  --work /tmp/plaincad-plainsolid-shell \
  --port 5294 --test e2e/shell.spec.ts --grep 'native Shell'
```

`--work` must be empty or nonexistent. The default is a fresh timestamped
temporary directory. `--port` defaults to 5279. The harness builds its own WASM
binary, copies the matching Go JavaScript runtime, typechecks the app, runs a
backend preflight, and then runs the selected existing fixtures. It requires
local server and browser process permissions.

## What changes in the isolated app

- The `OpenCascadeKernel` import boundary is replaced by `PlainSolidKernel.ts`.
  The compatibility class name remains, but geometry is dispatched to Go WASM.
- Exact provider string literals `"opencascade"` / `'opencascade'` in app and test
  TypeScript become `"plainsolid"` / `'plainsolid'`. Existing geometry, volume,
  topology, UI, persistence, and export assertions are otherwise preserved.
  Each changed file, replacement count, and before/after hash is recorded.
- Vite receives an isolated cache and explicit serving permission for the shared
  dependency directory. An optional port changes only the copied test config.
- A separate generated backend test proves real WASM execution and rejects OCCT
  asset requests. It checks nonempty material, no-op cut, connected-join behavior,
  and export union semantics. This probe is not counted as an existing fixture.

The initial adapter supports exact straight polygon extrusion (including supported
polygon holes), single full-circle extrusion without inner loops, boxes, planar
cut/join/common, clone/rigid placement, tessellation, STL, and unchanged rectangular
extrusion shelling. Circle geometry retains analytic cylinder/cap surfaces; only
display and STL are tessellated using the requested deflection settings. Mixed
line/arc profiles, curved inner loops, curved Boolean combinations, revolutions,
fillets/chamfers, STEP, threads, lead parts, and unproved topology references fail
explicitly. Scoped atomic operations and some app feature gates are still absent.
This harness is an acceptance integration, not a published production backend.

## Evidence

`readiness.json` records source paths, git revisions and dirty states, source
hashes, provider adaptations, the WASM hash, selected cases and their actual
results. Detailed logs, Playwright JSON, and failure traces remain beside it.
Skipped/interrupted tests cannot count as passes. A successful report requires a
passing backend probe and actual passes for every selected existing fixture.
Unselected fixtures remain **untested**; these results do not establish full
PlainCAD compatibility or full OCCT replacement. The original PlainCAD unit
suite contains OCCT-specific mocks and is not silently treated as portable.
