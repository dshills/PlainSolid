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
