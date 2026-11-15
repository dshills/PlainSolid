# STEP exchange for rectangular mapped-spline shells

`WriteMappedChainRectangularShell` and `ReadMappedChainRectangularShell`
round-trip the bounded seven-face source family. The ordered closed shell
has two spline roof charts, four vertical wall charts and one horizontal
floor. The same exact geometric proof used for native material admission
establishes that the source surfaces enclose a positive-height region.

The generated advanced B-rep uses standard `B_SPLINE_SURFACE_WITH_KNOTS`,
`B_SPLINE_CURVE_WITH_KNOTS`, `SURFACE_CURVE`, and paired `PCURVE` records.
Each shared roof edge uses its exact global station range in its model
spline knots. Its two pcurves point through parameter-valued
`TRIMMED_CURVE` records to their distinct complete original spline bases.
One basis is shared within each roof owner. The stored source ranges and
reversal preserve the original pcurve parameter even when the two owners
have different speed laws. The reader recovers each positive map from a
tangent ratio and verifies its complete lift against the stored model
curve. Both operations check every original vertex and closing coedge
before returning the solid or writing any bytes.

The reader accepts the canonical face and loop order, single-span spline
supports, two through eight shared roof edges, and exact source spline
data within the native limits. Original support and UV spline data must be
exactly representable in the native basis. Derived model and UV Bézier
records must have terminating STEP decimal coordinates. The generated
advanced B-rep remains usable in standard STEP consumers. General curved
shells, arbitrary external face ordering, periodic seams, and assemblies
require other exchange paths.

The optional live check uses the pinned OCCT STEP executable built by
`tools/occtref/build_author_convex_arc_step.sh`.

```sh
PLAINSOLID_OCCT_MAPPED_CHAIN_STEP=/path/to/author-convex-arc-step \
  go test -race ./exchange/step -run '^TestMappedChainRectangularSTEP'
```

## Independently authored OCCT STEP

`ReadOCCTMappedChainRectangularShell` opts into the independent source
form emitted by pinned OCCT. It finds the two original spline roof
supports among five analytic signed-axis planes and rotates each roof's
cyclic wire to the common chain. It accepts the identity representation
placement, `FACE_BOUND`, the normal pcurve master representation, and
direct separately authored cubic or quartic pcurves on shared roof
edges. Each such pcurve keeps its own original basis and local interval.
The reader assigns equally spaced global model stations because the
independent model-edge parameters are local. It requires the two
original roof wire directions to agree along the shared chain.

For analytic planes it retains the exact source origin and chart
directions and records a rational UV shift. This shift represents the
same chart after removing decimal offsets from the native coordinates.
The reader verifies raw `LINE` and degree-one spline pcurves against
the exact original planar lift, authenticates both directed source
vertices, and verifies the complete roof pcurve and model spline
identities. `OriginalSTEP` retains the complete independent input.
The ordinary solid and the standalone mapped-chain source recipe both
round-trip the independent pcurve bases through native JSON. The source
recipe uses a version-2 boundary record for the separate coedges. Face
tessellation retains both the incoming and outgoing raw parameters at each
joint, and both are tied to their own original coedges.

The recorded fixture was written by pinned OCCT with the floor first,
the roofs separated in shell order, and both roof wires started off the
shared chain. Live authoring covers the original and reordered forms.

```sh
sh tools/occtref/build_mapped_chain_shell.sh "$OCCT_INCLUDE" "$OCCT_LIB" /tmp/mapped-chain-shell step
PLAINSOLID_OCCT_MAPPED_CHAIN_AUTHOR=/tmp/mapped-chain-shell \
  go test -race ./exchange/step -run '^TestOCCTMappedChainRectangular'
```

## One original interior roof knot

Set `MappedChainShellSTEPOptions.AllowMultiSpanRoofs` for either bounded
reader and for the canonical writer. Each roof may then have one
interior support knot in either UV direction. The original support net,
knot values, multiplicities and UV coordinates are retained. The entire
trim control hull must lie in one closed original knot rectangle. A trim
on the knot may use either incident cell by C0 agreement. A trim spanning
the two cells needs an explicit source coedge split before this exchange
path can establish its exact model lift.

For native source preparation, `CertifyMappedSplineChainSupportKnotAgreement`
can derive one isolated, transverse interior crossing of the shared roof
boundary. Both original owner curves must admit exact raw restrictions at the
same model station. The native source recipe rediscovers that station on
reopen. The same multi-span writer exports the repaired boundary as
adjacent standard `EDGE_CURVE` records referencing one common
`VERTEX_POINT`. Each roof keeps its whole spline basis, and each coedge
keeps its own parameter-valued `TRIMMED_CURVE` and `PCURVE`. Model
stations must have exact terminating STEP decimals.

The repaired fixture adds model station `1/8` at support knot `29/256`.
Its original raw owner parameters are `5` and `27`, on different UV axes
and opposite raw traversal directions. PlainSolid reconstructs and
rewrites the paired coedges byte-identically, and pinned OCCT independently
opens the output as one valid seven-face solid.

### Two synchronized support crossings

`CertifyMappedSplineChainTwoSupportKnotsAgreement` proves two distinct
transverse crossings of the same original shared chain cell. Both roof
charts must recover the same two exact model stations, and both raw
owner restrictions must fit their original support cells. The version 4
native boundary recipe retains the original bases, maps, and authored
stations and rederives the two additional stations on reopen.

Set both `AllowMultiSpanRoofs` and `AllowTwoInteriorRoofKnots` to exchange
the explicitly split coedges. The fixture has exact model stations
`1/8` and `3/16`, with raw values `41`, `65` on the first roof and
`51`, `43` on the reversed roof. PlainSolid preserves both original
support knot vectors and owner spline bases, rebuilding and rewriting the
paired STEP coedges byte-identically. Pinned OCCT independently opens the
result as one valid seven-face solid. The unsplit foreign import below
covers a single crossing.

### One unsplit foreign STEP edge

Set both `AllowMultiSpanRoofs` and `AllowUnsplitSupportKnot` for
`ReadOCCTMappedChainRectangularShell`. One original shared `EDGE_CURVE`
may store a two-piece clamped rational model spline with one interior
knot of degree multiplicity. Each original cubic or quartic UV trim
must cross one support knot transversely and admit an exact raw split.
The reader recovers a positive parameter map from that event and compares
the full homogeneous model lift of each restriction with the corresponding
stored model piece. It then uses the native support-knot proof to rebuild
the shared vertex. `Crossing` records the original edge and model
parameter, the common native station, both raw parameters and both rebuilt
edge IDs. `Chain` retains the original STEP edge inventory.

Both roof chains can use direct per-edge pcurves or parameter-valued
restrictions on a single whole basis per owner. For the latter, the stored
model domains must form one contiguous chain over `[0,1]`. The exact
fixture `standard_mapped_chain_unsplit_knot.step` re-encodes the repaired
source as one shared degree-five spline across the interior knot, with
five signed-axis analytic planes and exact linear closing pcurves.
Pinned OCCT independently validates the source as one seven-face solid
with the same volume as the repaired shell. PlainSolid retains the two
original bases and raw values `5` and `27` through native reopen and
emits explicit paired coedges when writing STEP.

The independently authored creased shell has height
`x*y + max(x-1/2,0)/2` over `[0,1] × [0,3/4]` and floor height `-1`.
Both roof charts retain the interior knot at model `x=1/2` on their own
UV axes. The knot intersections with the outer roof boundary are
individual original coedge vertices on the roof and the adjoining
wall. The test covers equal one-sided knot-line lifts, distinct
one-sided slopes, native source reopen, canonical STEP, and live OCCT
authoring and transfer.

The live author's `write-knot` mode uses explicit signed-axis plane
frames and exact degree-one bounds on the wall incidences. The
`write-knot-default-planes` control lets OCCT generate its plane
parameter values. Its rounded source chart lifts disagree with the
stored model edges, so the exact reader rejects that control. Live
qualification uses the same optional binaries documented above and
selects `^Test(MappedChainRectangularSTEP|OCCTMappedChainRectangular)`.
