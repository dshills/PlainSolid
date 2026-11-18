# Implementation status

PlainSolid is an experimental kernel. It is **not** an OCCT replacement, and
general-purpose production use is unqualified. The working goal is to be the
kernel PlainCAD runs on; see [the repository instructions](../AGENTS.md).

This page states what is true now. Edit it in place when a fact changes; do
not append dated entries. The narrative that this page replaced is in git:
`git show c35d3358:docs/STATUS.md`.

## Current state

As of 2026-11-18, with kernel code at `c35d3358`.

| Check | Last result | Kernel commit |
|---|---|---|
| PlainCAD application with PlainSolid selected (app `efae4d2`) | Passed all eight gates: 345 development, 66 production, 15 cross-browser, 4 WASM contract and 1,691 unit cases | `9f95a188`, 2026-11-16 |
| Complete Go suite, macOS ARM64, Go 1.27.4 | 6,121 passed, 608 optional skips, 0 failed; vet and Linux and Windows amd64 cross-builds passed | `9f95a188`, 2026-11-16 |
| Complete Go suite, Linux ARM64, Go 1.26.0 | 6,018 passed, 599 optional skips, 0 failed | `c7afb429`, 2026-11-16 |
| Complete Go suite with the race detector, macOS ARM64 | 5,557 passed, 580 optional skips, 0 failed | `ab33a412`, 2026-11-15 |
| Fast Go tier (`go test ./...`), macOS ARM64 | 5,293 passed, 1,615 skipped, 0 failed | `c35d3358`, 2026-11-18 |

`main` is 97 commits past the last complete Go and PlainCAD runs. Only the
fast tier has been run on those commits. Commands, evidence files and what the
results do not establish are in [the verification record](VERIFICATION.md).

Not qualified: native Windows runtime, the complete suite on the current
commit, curved modeling beyond the subsets in the matrix below, independent
exchange beyond the listed families, and the release matrix in
[the specification](SPECIFICATION.md).

## PlainCAD gates

The gates PC0 to PC5 and their ordering are defined in
[the implementation plan](IMPLEMENTATION_PLAN.md). Their acceptance cases are
existing PlainCAD end-to-end tests, listed in
[PlainCAD readiness](PLAINCAD_READINESS.md). Those tests run inside the
PlainCAD development, production and cross-browser suites, which last passed
in full on kernel `9f95a188`. No PlainCAD case is known to fail.

The PlainSolid-backed app lives on the `codex/plainsolid-backend` branch of
the sibling PlainCAD checkout. The acceptance inventory is a critical-path
selection, not the whole PlainCAD suite; the remaining native application
tests and PlainCAD's `check:item` and `release:check` gates are still to be
enumerated and run before claiming replacement of the current feature set.

The next kernel task is the first failing PlainCAD acceptance case. With none
failing, new work needs a request from the user.

## Capability matrix

Phases refer to [the implementation plan](IMPLEMENTATION_PLAN.md). Each
"Delivered" entry is a restricted, tested input domain, not a general
capability. Unsupported input returns an explicit error.

| Plan phase | Status | Delivered | Still required for phase completion |
|---|---|---|---|
| 1: Foundation | Implemented initial domain | Go module/CI configuration; vectors/transforms; rational curves/surfaces; meshes/primitives/extrusion; bounded exchange; box Booleans; end-to-end example | Remote multi-platform CI execution and longer validation campaigns |
| 2: Geometry algorithms | Partial | Exact-sign predicates, analytic conics and selected intersections, exact raw-plane traces, spatial spline/segment contacts, bounded rational subpatches and rational evaluation, certified spline-pair coverage, curve/surface distance and length bounds, exact chart identity across spline bases, curve/tensor/scattered fixed-basis fitting, uniform periodic curves and wrapped spline curves/surfaces with exact seam proofs | Fitting with unknown parameters or weights, full periodic surface editing, more curve and surface pairs, production numerical envelope |
| 3: B-rep | Planar and curved subsets | Shared topology and shell/compound access; planar holes; exact analytic patches with seams and poles; rational quadratic UV topology certificates; continuous planar, spline-section and selected analytic edge agreement; anchored and closed certified spline-tube face splits; exact sewing of spline rectangles and selected analytic periodic faces; revolutions, circular sweeps, elliptic station lofts and affine maps | Certified arbitrary trim validation, general loft and sweep, richer topology history |
| 4: Booleans | Restricted subsets | Rational nonconvex planar BSP, Boolean face and edge ancestry, oblique splits and section loops; orthogonal cells and common-height prisms; sphere, parallel-cylinder and coaxial sphere/cylinder Booleans; oblique cylindrical bores and spherical box pockets; exact quadratic plane tangencies and subdivision-corner reconciliation; certified spline–plane sealing of single open, disjoint or nested closed contours; open and closed sealing of two spline graphs and shared-base graph Booleans across open, closed, nested and mixed contours; spline-pair tubes, mixed-axis open/closed branches, paired trim evaluation, automatic discovery and independent tube coverage checks | General curved operations, complete curved branch discovery, curved coincidence/tangency assembly and complete topology history |
| 5: Advanced modeling | Restricted subsets | Planar sewing with remaps/displacement; exact same-domain planar face unification with retained/deleted edge intervals and vertex locations; faceted shell promotion; convex mitered offsets and chamfers; rational square-metric offsets for orthogonal footprints; rounded rectangular bodies, selective unequal-radius convex prism fillets, draft prisms, affine support edits, two-distance edge chamfers, planar sketch protrusions/pockets, convex hollowing, support removal, and cylindrical, spherical and toroidal thickening | General fillets/offsets/thickening, draft, local edits, wider sewing and repair |
| 6: Tessellation/queries | Partial | Conforming planar and curved meshes, derivative bounds for supported analytic and clamped rational spline surfaces, curved UV-trim refinement and certified drawing chords on spline model edges, exact planar boundary distance and point projection with BVH pruning and exact point location, exact mesh contact classification, signed trim-strip mass/inertia integration, analytic trimmed point and ray queries and exact unit-chart spline graph classification, certified spline line contacts, single-shell embedding proof for selected spline rectangles, exact planar assembly embedding and nesting, mesh BVH | Guaranteed arbitrary curved trim bounds, general exact-shape queries and curved embedding beyond certified subsets |
| 7: Interoperability | Restricted CAD and mesh subsets | Faceted and planar advanced STEP, convex planar STEP cavities, body and face colors, conversion-based units, analytic primitives, tubes, stepped shafts and mixed cylinder/cone profiles, spherical caps and zones, cylindrical/conical sectors and torus elbows, cylinders with an oblique cap, rigid planar STEP assemblies with product-local face colors, STEP conic/spline geometry, bounded rectangular and orthogonal spline faces and selected certified closed spline shells, IGES analytic and spline geometry plus planar types 142/144, native topology including verified implicit spline–plane and spline-pair tube recipes and opt-in exact planar embedding checks, OBJ/STL/PLY, embedded glTF/GLB scenes | General curved STEP B-reps, broader IGES, wider assemblies/metadata, additional glTF and PLY features |
| 8: Documents/presentation | Partial | Mixed assets, typed labels/attributes, bounded feature graph regeneration, STEP occurrence import/export, shared assets and planar body colors, versioned snapshots, history, orthographic/perspective projection, picking and mesh-occluded edge spans/SVG, stable box face roles for bores and planar sketch features; colored STEP document adapters | General stable topological naming, broader feature dependencies, exact curved occlusion and silhouette extraction, optional interactive rendering |
| 9: Qualification | Initial differential cases pass | Locally built pinned OCCT, primitive and restricted Boolean comparisons; independent STEP and IGES qualification; Go unit/race/vet and focused fuzzing | Broad differential corpus, platform CI execution, adversarial numerics, API and packaging decisions |

General curved Booleans, arbitrary curved trim boundaries and broad CAD
exchange remain open. Package comments define each operation's input domain
and its numerical and work budgets. The per-package summary is in
[the README](../README.md#current-capabilities).
