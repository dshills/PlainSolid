# Exact nonconvex planar hierarchy point mode

`LocateGeneralRationalHierarchyPoint` and
`LocateGeneralOCCTRationalHierarchyPoint` add nonconvex/holed planar material to
the separately proved rational occurrence conventions. Existing analytic-only
and convex-only APIs retain their previous meaning. Existing line entry points are unchanged; the additive transverse mode is described below.

The complete source hierarchy, product geometry, closed manifold and exact
embedded native boundary are reproved first. Every native directed outer/hole
loop is matched cyclically to exactly one original source loop, with exact source
decimal vertex equality, edge ownership and original plane support/sense. Signed
projected areas authenticate outer versus hole winding. No rounded normal,
triangle approximation or convex hull substitutes for the original face region.

A point on a supporting plane is on the material boundary only when exact finite
polygon winding (including opposite hole loops) admits it. Original incident
face IDs are filtered by this finite membership; exact finite segment and vertex
tests retain edge/vertex IDs. Infinite supporting-plane coincidence alone is not
a boundary claim. Holes are tested by winding cancellation, not bounding boxes.

Otherwise the query tries rational directions `(1,k,k²)`. A direction is accepted
only if it is nonparallel to every face plane and every forward intersection
strictly inside a finite face avoids all loop edges/vertices. Exact winding counts
these transverse crossings; odd parity means inside. A degenerate ray is discarded
and contributes no classification. The result never falls back to floating
jitter. Exhausting the explicit ray/work limits returns no partial result.

MaxRayDirections defaults to64 and is capped at4096, cumulatively across all
occurrences. MaxPlanarTests, source limits and coefficient-bit limits remain
independent gates. RayDirections reports the successful query's cumulative
attempt count. Each original local planar proof is built once and reused across
its occurrences, with all rational coordinates privately owned.

Independent source: pinned OCCT 4dbfdcc448c0b3e565488afcd626c84483172db9 authors
an L-shaped reentrant prism with integer original planes/lines and full pcurves
via `step_reentrant_planar.cxx`. The authoring script inserts its complete owned
geometry closure into each retained AP214 hierarchy without altering other local
bodies, product IDs or occurrence relationships. Both original files pass normal
OCCT transfer, and native source persistence/reload followed by exact queries
matches OCCT at inside, notch-outside, concave edge, corner and exterior points.
The live test uses PLAINSOLID_OCCT_RATIONAL_HIERARCHY_LOCATION.

A forced edge-crossing first ray proves retry/failure semantics. A separate exact
planar prism with a rectangular cap hole tests finite hole exclusion, cap/hole
wall identity and rejection of a reversed original inner loop. This latter
helper regression is not claimed as an independently authored OCCT fixture.

Coplanar nonconvex line clipping remains separate work: complete finite edge
event and boundary-span proof is required before lifting its conservative gate.

## Additive transverse line mode

`ClipGeneralRationalHierarchyLine` and `ClipGeneralOCCTRationalHierarchyLine`
reuse the immutable finite-loop proof for transverse nonconvex/holed leaves.
All finite face-plane intersections are exact rational original-t stations.
Identical stations are grouped before classification; exact source face/edge/
vertex ownership survives reentrant and shared-corner degeneracies.

Each open interval between consecutive stations is classified by the established
bounded rational-ray parity proof. Inside intervals become closed material spans;
isolated contacts remain hits without invented volume intervals. Adjacent spans
may meet at a retained boundary station: they are deliberately kept separate so
each open span remains strictly interior. Their union is the regularized material
intersection. There is no Boolean union across different assembly occurrences.

A line contained in any original supporting plane is rejected for that planar
leaf with an explicit UnsupportedReason before any of its events are emitted.
This conservative gate also rejects coplanar lines missing the finite polygon.
Parallel planes not containing the line are harmless. Existing convex line mode
retains its separately proved coplanar face/edge spans; it is not silently changed.

MaxRayDirections defaults to256 in line queries (maximum4096), cumulatively over
all station and interval classifications. Existing source, planar predicate,
coefficient-bit and algebraic-operation/subdivision quotas remain separate.
All errors return no partial ledger. Native local source proof is reused once
per product; original t is never normalized or converted to distance.

Pinned OCCT independently checks both retained reentrant hierarchy modes: four
crossings/two disjoint material spans, nonunit directions, reentrant edge contact
and a triple-face vertex. Exact tests also prove hole-preserving intervals,
source owner sets, coplanar unsupported behavior and exhausted quotas. No general
coplanar nonconvex clipping is claimed.

## Separate coplanar finite-face mode

`ClipCoplanarGeneralRationalHierarchyLine` and
`ClipCoplanarGeneralOCCTRationalHierarchyLine` remove the coplanar gate only by
adding a complete projected line/loop-segment event ledger. Each original outer
and hole segment is intersected by exact rational determinants. Collinear
segments contribute their actual endpoint parameters; finite crossings require
the exact segment parameter in [0,1]. Projection drops a nonzero normal component,
so no nonzero coplanar line can lose its direction. No coincident parameter is
chosen by approximation or sampling.

These events are deduplicated against all transverse face events at identical
original rational t. Every intervening open interval is classified using the
existing exact finite-loop/material proof. Interior intervals remain Spans;
finite face/edge-coincident intervals become BoundarySpans with all original
face and edge IDs. Since every loop crossing and vertex has become a station,
these owner sets are constant in each open interval. Isolated tangent contacts
remain hits. Clip endpoints alone are not events, even when the clipped line
lies along an edge. The transverse-only API preserves its earlier rejection.

The pinned OCCT author now also constructs an outer wire plus reversed inner
wire and extrudes the resulting holed face. Its genuine original STEP geometry
closure is inserted into both AP214 hierarchy conventions. Independent transfer
matches cap-hole, reversed-direction, hole-wall, coincident hole-edge and tangent
stations, in addition to reentrant disjoint coplanar spans. Exact checks verify
mixed interior/boundary intervals, finite hole exclusion and clipped edges with
no invented endpoint hits. Existing quotas apply to every added segment test.
This is not a widening of the source importer to arbitrary Boolean-generated
TRIMMED_CURVE edges; only the existing authenticated planar input domain is used.
