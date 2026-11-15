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
knot values, multiplicities and UV coordinates are retained. In a
canonical wire, each trim control hull must lie in one closed original
knot rectangle. A trim on the knot may use either incident cell by C0
agreement. The foreign reader's option for an unsplit wire is below.

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
result as one valid seven-face solid.

### Mixed support crossings in one original cell

`CertifyMappedSplineChainMixedSupportKnotsAgreement` proves one shared
crossing and one single-owner crossing inside the same original common
chain cell. One roof has two interior knots and the other has one. The
version 6 native boundary recipe rederives both transverse crossings from
the original supports and raw trims. Versions 4 and 5 retain their
synchronized and distinct-cell asynchronous arrangements.

The mixed fixture crosses both roofs' original support cells at model
station `1/8`, with raw parameters `41` and `51`. At `3/16` only the
first roof changes cells, with raw parameters `65` and `43` on the paired
coedges. Both roofs retain the crease at the shared crossing. The second
crease changes only one roof's one-sided derivative away from the shared
trace. The existing canonical STEP path uses `AllowMultiSpanRoofs` and
`AllowTwoInteriorRoofKnots` for explicit paired coedges.

### Three support crossings in one original cell

`CertifyMappedSplineChainThreeSupportKnotsAgreement` proves three
transverse stations in one original common chain cell. Both roofs have
exactly two original support knots. At one station both owners change
cells, and at each of the other stations only one changes cells. Version
7 of the native boundary recipe rederives all three cuts from the source
supports and raw trims. Versions 4 through 6 retain their existing
arrangements and byte representations.

The three-cut fixture has stations `1/16`, `1/8`, and `3/16`, with raw
parameter pairs `(93,443)`, `(197,387)`, and `(317,283)`. The canonical
explicit STEP path uses `AllowMultiSpanRoofs` and
`AllowTwoInteriorRoofKnots` and retains all original raw restrictions.
Pinned OCCT independently opens the explicit output as one valid solid.

The pinned author mode `write-three-knots` constructs both original
support charts and the six shared model edges independently in OCCT.
It supplies exact affine model lifts of the source UV curves, with extra
decimal precision for the dyadic coordinates. The recorded AP242 file
imports through the bounded foreign reader with explicit coedges. Its
individual source pcurve bases survive a native recipe round trip.
Set `PLAINSOLID_OCCT_MAPPED_CHAIN_THREE_AUTHOR` to the pinned author
built with absolute include and library paths for live qualification.

### Asynchronous support crossings

`CertifyMappedSplineChainAsynchronousSupportKnotsAgreement` proves one
transverse crossing per roof in distinct original common chain cells. At
each station the other roof stays within its original support cell. Both
raw parameter restrictions and their complete model equality are proved
at every resulting coedge. The version 5 native boundary recipe stores
the original charts and authored trims and rederives both cuts on reopen.

The fixture meets along `y=1/4, z=x/4`. The first roof's U knot lies at
model station `1/8`, with raw owner parameters `17` and `27`. The second
roof's V knot lies at station `3/8`, with raw owner parameters `65` and
`21`. Both roofs retain their distinct one-sided creases away from the
shared edge. The canonical STEP path uses `AllowMultiSpanRoofs` and
writes explicit paired coedges at both stations. Output reopens and
rewrites byte-identically, and pinned OCCT independently opens it as one
valid seven-face solid.

### An unsplit foreign STEP edge

Set both `AllowMultiSpanRoofs` and `AllowUnsplitSupportKnot` for
`ReadOCCTMappedChainRectangularShell`. One original shared `EDGE_CURVE`
may store a two-piece clamped rational model spline with one interior
knot of degree multiplicity. Also set `AllowTwoInteriorRoofKnots` for an
unsplit three-piece model spline with two degree-multiplicity knots.
Each original cubic or quartic UV trim must cross each corresponding
support knot transversely. One positive parameter map per owner must
satisfy every junction and the full homogeneous lift of each restriction.
The reader uses the native support-knot proof to rebuild the shared
vertices. `Crossings` is ordered by native model station and records the
original edge, source model knot, native model station, two raw parameters,
and adjacent rebuilt edge IDs. `Crossing` identifies the sole repair in
the single-crossing case. `Chain` retains the original STEP edge inventory.

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

`standard_mapped_chain_unsplit_two_knots.step` likewise re-encodes all
three exact model branches as one degree-five curve with two interior
knots and two original unsplit owner restrictions. Pinned OCCT independently
validates the unsplit input as one seven-face solid. PlainSolid retains both
junctions and reopens the native source before exporting explicit coedges.

For asynchronous foreign input, add `AllowAsynchronousSupportKnots` to
`AllowMultiSpanRoofs` and `AllowUnsplitSupportKnot`. Two distinct original
shared edges each store one model Bézier. On each edge exactly one owner
crosses an original support knot. Its exact first model tangent determines
the owner parameter law. Both raw restrictions of each pcurve are proved
equal to the corresponding rational restrictions of that original model
curve. The reader then applies the native version 5 proof. Each
`Crossings` record identifies the corresponding foreign edge and the roof
that changes cells, and reports both original raw parameters.

`standard_mapped_chain_unsplit_asynchronous_knots.step` re-encodes the
asynchronous fixture with two unsplit edges and no model-curve knots at
the repairs. Pinned OCCT independently validates the original unsplit
source as one seven-face solid. Combining both crossings into one
original edge requires another common-cell partition.

For one unsplit mixed edge, set `AllowMixedSupportKnots` together with
`AllowMultiSpanRoofs`, `AllowTwoInteriorRoofKnots`, and
`AllowUnsplitSupportKnot`. The source edge stores two model Béziers with
one degree-multiplicity knot at the shared material crease. Both owners
must cross there. The other exact crossing lies inside one stored model
piece and changes only one original support chart. An exact tangent ratio
proposes the two-knot owner's whole raw parameter law. All three
restrictions of both pcurves must match their exact stored model
restrictions before the reader applies native version 6.

`standard_mapped_chain_unsplit_mixed_knots.step` retains one original shared
edge across both stations, with just the model knot at `1/8`. At the
inferred `3/16` station, the companion stays in its original support cell
and preserves its exact raw parameter `43`. Pinned OCCT independently
validates both the unsplit source and the emitted canonical split wire as
the same seven-face solid.

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
