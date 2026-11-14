# Exact oblique original NURBS planar assemblies

`ReadObliqueNURBSPlanarAssembly` is a separate, bounded extension for original
polynomial planar spline products whose common proper rational rigid frame is
not signed-cardinal. The independent positive is an actual OCCT AP242 NURBS
assembly with 3:4:5 oblique local geometry. Existing unit and scaled-cardinal
NURBS entry points remain strict.

## Source and native proof

Each local product must own exactly one advanced body; geometry-free assembly
groups retain the existing assembly rules. An original first face supplies two
orthogonal, nonzero chart directions from its original four spline poles. Their
squared lengths must have exact rational square roots. Exact rational
normalization and the cross product construct a proper orthonormal basis; no
binary64 unit vector is substituted for a nonbinary ratio.

The adapter traverses the entire original body dependency graph and transforms
only its original three-dimensional CARTESIAN_POINT records into a private
cardinal coordinate system. All incoming references to each changed point must
belong to that body. Geometry cannot be borrowed across products; only the
permitted parametric context records may be shared. UV records, knot parameters,
face/edge senses, occurrence frames and all product/style metadata are unchanged
in this phase. Cycles and excessive traversal depth fail closed.

Every original and transformed point must be exactly representable as binary64;
the transformation itself is evaluated with exact rationals. The resulting
local proof graph must pass the complete scaled-cardinal NURBS importer:
original affine surface identity, finite model and UV domains, both actual
incident pcurve owners, continuous lifted model identity, topology, source
styles and assembly ownership. This also rejects a first-face basis that cannot
correctly account for every other original support and model curve.

The derived native vertices are then transformed back with exact rational
arithmetic. Each must be exactly representable and match an actual original
source vertex; the source/native vertex sets must agree. Native faces and edges
retain their proved orientation and topology. Exact global planar embedding and
material orientation are independently validated in original coordinates.
Native geometry therefore lives in the ORIGINAL product frame. The authored
occurrence transforms are never changed or silently composed with a rounded
product frame.

The unit NURBS importer now also performs this global embedding check after its
local material proof. This addition does not alter the legacy generic planar
reader. Per-body embedding work is capped at 512 triangles, 32768 candidate
pairs, 250000 operations and 4096 coefficient bits, with context cancellation.
All three NURBS import modes retain the source bounds of 1 MiB, 4096 entities,
65536 values, depth 16, 4096 uses and 16 bodies, or smaller caller limits.

## API and retained identity

`Geometry` contains native local bodies in original coordinates, original
assembly products/uses and original `Assembly.Source`. Body face indices still
follow the original CLOSED_SHELL ordering. `ProductFrames()` returns copied
source-derived proof-frame evidence: actual product-definition and PRODUCT IDs,
body/first-face/support/pole IDs, rational origin and basis columns.
`SourcePoints()` returns copied original/proof coordinates and original point,
body and product-definition IDs. These are rational strings, not rounded
observations.

`OriginalSTEP()` and `WriteObliqueNURBSPlanarAssemblySource` preserve the complete
original source. The writer re-proves that source and never serializes edits to
derived native fields. There is deliberately no public "normalized assembly"
STEP: the private local coordinate changes are proof machinery, not authored
occurrence changes or an equivalent world-coordinate assembly on their own.
This milestone introduces no native JSON format or general geometry authoring
API.

Unsupported cases include irrational basis norms, skew/nonorthogonal chart
axes, loss during either coordinate transformation, multiple bodies in a local
product, curved/nonaffine supports, rational weights, and any geometry outside
the previously proved planar spline topology. This is not an approximate
rotated-plane import or a generic NURBS material reader.

## Independent source and live evidence

Pinned OCCT: `4dbfdcc448c0b3e565488afcd626c84483172db9`.

`author_ap242_oblique_nurbs_planar.cxx` constructs independent 5×10×15 and
5×5×10 boxes, converts/reparameterizes them as original NURBS, applies an OCCT
3:4:5 XY transformation, and writes AP242 through XCAF. The source must pass
BRepCheck before export. The retained 681-record fixture has exact integer
original 3D poles/vertices; Go proves the encoded coefficients rather than
assuming the nominal C++ transform was exact. No STEP tokens are replaced with
Go geometry to make this positive fixture.

The source has two local products, three uses (one shared product reused under
a cyclic occurrence rotation), 12 original surfaces, 24 original model edges,
48 owner pcurves, and 112 original 3D point records. A second source is freshly
authored with shifted UV chart knots during the live test.

Native original local measures:

| Local body | Volume | Area | Centroid |
|---|---:|---:|---|
| Large | 750 | 550 | (-5/2, 5, 15/2) |
| Small | 250 | 250 | (-1/2, 7/2, 5) |

Original and re-exported sources, including the shifted-chart variant, pass
ordinary XCAF transfer and BRepCheck with 3 solids and 18 face uses. Native
occurrence summation and OCCT agree on V1750, A1350, and world centroid
`(69/14,66/7,99/14)`. These are assembly measures, not a regularized union.

```
sh tools/occtref/build_author_ap242_oblique_nurbs_planar.sh OCCT_INCLUDE OCCT_LIB OUTPUT
OUTPUT write fixture.step
OUTPUT write-shifted shifted.step
PLAINSOLID_OCCT_AP242_OBLIQUE_NURBS_PLANAR=OUTPUT go test ./exchange/step -run '^TestObliqueNURBSPlanar' -count=1
go test -race ./exchange/step -run '^Test(Oblique|Scaled)?NURBSPlanar' -count=1
go test ./exchange/step -run '^$' -fuzz '^FuzzObliqueNURBSPlanarAssembly$' -fuzztime=40x -parallel=2
```

Tests prove rational proper basis identities and every source/proof inverse,
original local native mass/geometry, source re-export and topology stability,
record reordering, unchanged old modes, copied evidence, bounds and cancellation.
Adversaries cover rounded poles, nonaffinity, irrational/nonorthogonal basis,
cross-product geometry, occurrence borrowing of a geometry point, exponent
underflow and an exact shear outside the rigid domain.
