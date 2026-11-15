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
`tools/occtref/build_author_convex_arc_step.sh`:

```sh
PLAINSOLID_OCCT_MAPPED_CHAIN_STEP=/path/to/author-convex-arc-step \
  go test -race ./exchange/step -run '^TestMappedChainRectangularSTEP'
```
