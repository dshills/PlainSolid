# Exact polynomial NURBS planar assemblies

`ReadNURBSPlanarAssembly` is an explicit source-preserving importer for a
bounded representation emitted by OCCT `BRepBuilderAPI_NurbsConvert` and its
AP242 XCAF writer. It is not a general NURBS solid importer. Existing planar,
rectangular spline, affine spline and contextual appearance readers retain
their existing contracts.

## Original source proof

The supported original faces have simple `B_SPLINE_SURFACE_WITH_KNOTS`
records: degree one in both directions, four independent polynomial poles,
clamped two-value knot vectors, nonperiodic/nonclosed flags, and
`PIECEWISE_BEZIER_KNOTS`. Exact rational coefficient arithmetic proves
`S(u,v) = O + u*DU + v*DV`. Both derivatives must be orthogonal signed cardinal
unit vectors. Nonaffine four-pole nets, scaled derivatives, rational weights,
nonrepresentable scalar conversions and additional spline spans fail closed.
The original support rectangle and original pole, surface and face IDs remain
in copied evidence.

Model edges must be simple clamped degree-one polynomial splines with exactly
two poles, a signed cardinal unit derivative, and exclusively owned
`SURFACE_CURVE` / `EDGE_CURVE` records. Exact source endpoints, including
`EDGE_CURVE.same_sense`, must select the complete finite model domain.

Each model edge has exactly two distinct owner pcurves, one for each actual
incident face. Their original degree-one coefficients, finite knot domain,
model endpoints, and complete same-parameter lifted equation are checked by
the existing exact affine pcurve proof. Both UV poles must lie inside the
original owner support rectangle; their affine convex hull therefore stays
inside it for the whole finite interval. Shared poles, substituted owner
surfaces and duplicate owner uses are rejected even when they happen to
produce the same physical edge.

Only after these proofs does a private equivalent PLANE/LINE view enter
`ReadColoredAssembly`, which validates native closed planar topology,
product ownership, common units, occurrences and product-local styles.
The NURBS adapter additionally validates exact global embedding and material
orientation with the planar boundary prover.
Original occurrence frames must be exact signed cardinal frames, with a
neutral child mapping frame; there is no rounded inverse or vendor convention
selection. Contextual overrides, unsupported metadata and mixed units retain
the existing strict rejection behavior.

## Results and output

`Geometry` contains derived native local bodies, shared assembly uses, names,
units and supported styles. Its `Assembly.Source` is explicitly the equivalent
proof view. `OriginalSTEP()` returns the original source, including its spline
bases and AP242 metadata. `Surfaces()`, `Curves()` and `PCurves()` return copies
of exact source-role evidence. Their coordinates are rational strings.

`WriteNURBSPlanarAssemblySource` re-proves and emits the retained original bytes;
it does not serialize edits to the derived `Geometry` field. Native planar
products can separately be authored with the existing `WriteColoredAssembly`
API; that is new planar geometry encoding, not preservation of original spline
bases. This milestone does not introduce a new native JSON schema.

Input is capped at 1 MiB, 4096 entities, 65536 parsed values, nesting depth 16,
4096 edge uses and 16 material bodies, further restricted by caller limits.
Each native body is limited to 512 proof triangles, 32768 collision pairs,
250000 embedding operations and 4096 coefficient bits. Cancellation is checked
before and after bounded validation and during support
conversion. Failure returns no partial assembly. Source writes are staged
before touching the destination; a destination I/O error may still be partial.

## Independent qualification

Pinned OCCT commit: `4dbfdcc448c0b3e565488afcd626c84483172db9`.

`tools/occtref/author_ap242_nurbs_planar.cxx` independently constructs two boxes
(2×3×4 and 1×2×3), converts each with `BRepBuilderAPI_NurbsConvert`, and authors
three XCAF occurrences including a repeated cyclically rotated product. The
retained `testdata/occt-ap242-nurbs-planar-assembly.step` is its original
681-record AP242 output, with only trailing whitespace stripped. No geometry
or pcurve token is replaced by Go output. It contains 12 original affine spline
supports, 24 model splines and 48 owner pcurves, including shifted chart bases.

The live test reads the original, source re-export, equivalent proof view and
independently generated planar output through ordinary XCAF transfer. Each has
volume 54, area 126, centroid `(85/18,86/9,5/2)` and the two product colors.
Native local volumes are 24 and 6. The legacy planar and affine-spline readers
still reject the original representation.

```
sh tools/occtref/build_author_ap242_nurbs_planar.sh OCCT_INCLUDE OCCT_LIB OUTPUT
OUTPUT write fixture.step
PLAINSOLID_OCCT_AP242_NURBS_PLANAR=OUTPUT go test ./exchange/step -run '^TestNURBSPlanarAssembly' -count=1
go test -race ./exchange/step -run '^TestNURBSPlanarAssembly' -count=1
go test ./exchange/step -run '^$' -fuzz '^FuzzNURBSPlanarAssembly$' -fuzztime=40x -parallel=2
```

Mutations cover nonaffine surfaces, nonlinear curves, sub-binary64 source
perturbations, exponent underflow attacks, wrong/shared owner graphs, domain
and endpoint mismatch, inconsistent sense, nonneutral/rounded placement,
limits, cancellation and atomic invalid writes. Reversing source record order
remains accepted without recovering geometry roles from numeric entity order.
