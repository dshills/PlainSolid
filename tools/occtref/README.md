# Optional OCCT differential oracle

The Go library and ordinary Go tests have no native dependencies. This small,
independently authored C++ test program calls OCCT for independent comparison.
Use OCCT commit `4dbfdcc448c0b3e565488afcd626c84483172db9` from
[the upstream repository](https://github.com/Open-Cascade-SAS/OCCT/tree/4dbfdcc448c0b3e565488afcd626c84483172db9).
Keep upstream source and compiled libraries outside PlainSolid.

Build the reference with a C++17 compiler and shared OCCT libraries. For
example, configure OCCT with CMake, Release mode, the main modules disabled,
`BUILD_ADDITIONAL_TOOLKITS=TKPrim;TKBO;TKDESTEP;TKDEIGES;TKFillet`, `USE_TK=OFF` and
`USE_FREETYPE=OFF`. OCCT automatically includes prerequisite toolkits. In a
source build, use the generated `include/opencascade` and platform library
directory, or use a matching OCCT installation.

```sh
sh tools/occtref/build.sh "$OCCT_INCLUDE" "$OCCT_LIB" /tmp/occtref step-iges
PLAINSOLID_OCCT_ORACLE=/tmp/occtref PLAINSOLID_OCCT_STEP=1 \
  go test -v ./internal/occtcheck
```

Omit the last build argument for geometry-only support, or use `step` for
geometry and STEP without IGES. The tests run the executable, verify OCCT
topological validity, and compare area, volume and centroid; they do not compare
unstable entity order or claim general kernel parity. Additional modes:

```sh
/tmp/occtref /tmp/properties.jsonl
/tmp/occtref /tmp/result.jsonl input.step
/tmp/occtref /tmp/box.step write-step
/tmp/occtref /tmp/curves.igs write-iges
/tmp/occtref /tmp/result.jsonl iges input.igs
```

The IGES oracle validates imported shapes and reports total curve length and
edge count. All exchange fixtures produced by the executable are generated
at test time; no OCCT source, binaries, or test fixtures are vendored.

`frustum-erosion` constructs each tapered reference by intersecting an
inward-shifted full lateral cone with a separate cap slab. The Go test
checks volume, area and centroid for rising and falling taper, then checks
the closed walls against the corresponding OCCT subtractions.

The native prism comparator receives original profile rings and heights,
builds both prisms independently, and executes OCCT Common, Fuse, or Cut.
The Go test compares volume, area, centroid, solid and shell counts, and
Euler characteristic. The cases include rotated and axially reflected
placements, cavities, retained holes, and disconnected material.

```sh
sh tools/occtref/build_native_prism_boolean.sh "$OCCT_INCLUDE" "$OCCT_LIB" /tmp/occt-prism
PLAINSOLID_OCCT_NATIVE_PRISM_BOOLEAN=/tmp/occt-prism \
  go test -race -v ./brep/trimmed -run '^TestNativePrismBooleanAgainstOCCT$'
```

### Three transverse polynomial nodes

The `node-triple` mode independently builds two complete original roof
stocks with a cubic and a quartic contact branch. Their difference is
`(1-4u)(1-2u)(3-4u)(1+u)/256`. The test compares Common and Fuse volumes
for both chart axes and operand orders against the native solid and exact
integrals. On the pinned OCCT revision each case returns one valid solid
and the largest difference from its exact volume is about `9.64e-11`.

```sh
sh tools/occtref/build_two_polynomial_roofs.sh "$OCCT_INCLUDE" "$OCCT_LIB" /tmp/occt-polynomial-roofs
PLAINSOLID_OCCT_THREE_POLYNOMIAL_NODES=/tmp/occt-polynomial-roofs \
  go test -v ./brep/trimmed -run '^TestNativeThreePolynomialNode'
```

The ordinary native test independently uses the full-gap variant, with
denominator 128. Its exact analytical volumes qualify separately.
At the full gap, the pinned OCCT comparison on the swapped chart in reverse
operand order differs from the exact volume by approximately `5.33e-7`
for both Common and Fuse.

### Tangent polynomial-roof diagnostic

The `tangent` mode builds the same two original mixed-degree roof stocks as
the native tangency tests. The cubic and quartic contact branches differ by
`(2u-1)^2(1+u²)/64` and meet at `u=1/2`.

```sh
sh tools/occtref/build_two_polynomial_roofs.sh "$OCCT_INCLUDE" "$OCCT_LIB" /tmp/occt-polynomial-roofs
/tmp/occt-polynomial-roofs tangent
go test -race -v ./brep/trimmed -run '^TestNativePolynomialTangency'
```

On the pinned revision, the unrestricted Common and Fuse calls return empty
shapes in both chart axes and operand orders. Cut returns one solid with the
volume of its first operand, about `3.092384338` or `3.00390625`. The exact
analytic Cut totals are about `0.088478377` and `2.8838436e-7`, each divided
into two layers with one shared point. The Go tests check each analytical
component volume, original coedge agreement and the exact contact ledger
after native reload.

Run all optional exchange checks with:

```sh
PLAINSOLID_OCCT_ORACLE=/tmp/occtref PLAINSOLID_OCCT_STEP=1 PLAINSOLID_OCCT_IGES=1 \
  go test -v ./internal/occtcheck ./exchange/step ./exchange/iges -run OCCT
```

The reference also supports `write-step-curves`, `step-curves input.step`,
`write-step-surface`, `step-surface input.step`, `write-iges-surface` and
`iges-surface input.igs`. Curve checks measure edge count and length. Surface
checks measure face count and area. The OCCT-generated STEP surface is a
shell-based model; the Go test decodes its spline support entity without
claiming to import its trim topology.

`write-step-assembly` exports an XCAF document containing a named box part reused
in two named occurrences. STEP-enabled oracle builds additionally link TKXCAF,
TKCAF and TKLCAF; these remain optional development-only OCCT dependencies.

`step-assembly input.step` independently reads a STEP assembly through XCAF and
reports named products, occurrences, local placements, root validity and mass.

`simultaneous-spline-shell` independently sews two sets of biquadratic
supports and compares union, intersection and subtraction at a simultaneous
original edge crossing. The curved supports share a determinant-one
vertical quadratic warp. No STEP or native geometry from Go is consumed.

`write-iges-pcurve` independently authors a type142 record over a bilinear plane,
with matching rational UV/model curves. It supports exact correspondence and
import/export qualification without generating a trimmed-face claim.

`write-iges-trimmed` authors a type144 affine planar face with a polygon outer
contour and a polygon hole. Both contours use explicit type142 UV/model splines.

`draft` independently intersects manually authored OCCT halfspaces for one-sided,
four-sided, oblique-neutral-plane and apex-collapse draft fixtures. It reports
unique vertex/edge/face/solid counts as well as area, volume and centroid.
`TestOCCTPlanarDraft` compares all of these against native `csg/draft` results.
The reference does not read Go-generated geometry or exchange files:

```sh
PLAINSOLID_OCCT_ORACLE=/tmp/occtref go test -v ./internal/occtcheck -run TestOCCTPlanarDraft
```

The `draft` mode also includes unequal two-distance selected-edge chamfer and
four-edge simultaneous chamfer fixtures. Independently authored diagonal
halfspaces define the OCCT bevels. The same differential test compares their
unique topology counts and mass properties against `draft.Chamfer`.

`feature` independently authors OCCT prism tools and fuses/cuts them against a
box for protrusion, blind-pocket, and concave oblique versions of both features.
`TestOCCTPlanarFeatures` compares area, volume and centroid with `csg/feature`.
Native generated triangle counts deliberately need not equal OCCT polygon-face
counts; both outputs are validated as solids.

The `feature` mode also includes straight and oblique through-pocket tools ending
at the opposite box face. `TestOCCTThroughPockets` compares volume, area and
centroid and verifies genus-one topology in the native result.

`feature` additionally constructs a closed box cavity and open box tray through
independent OCCT box subtraction. `TestOCCTHollow` compares native exact support
hollowing with OCCT area, volume and centroid.

`draft` includes `defeature-two` and `defeature-all` fixtures that independently
reconstruct a cube with two or zero remaining diagonal corner supports.
`TestOCCTMultipleSupportRemoval` compares these against simultaneous exact
support removal from a four-chamfer native solid. This qualifies the resulting
halfspace geometry, not OCCT's general feature-recognition algorithms.

`spline-shell-boolean` independently authors six bilinear OCCT B-spline patches
for `0<=x,y<=2`, `0<=z<=1+xy/4`, sews them into a checked solid, and performs
native OCCT union/common/cut against a scaled nested copy, translated disjoint
copy, and mirror copy sharing the full `x=2` face. Native Go construction uses
a degree-two elevated top patch and `SewSplineRectangles`. No native geometry or
exchange output is passed to OCCT. `TestOCCTSeparatedSplineShellBooleans` compares
all nine outcomes for area, volume, centroid and shell count, including an
inward cavity shell and empty regularized intersections. It qualifies separated
and complete-face-joined shell operations, not crossing freeform trimming.

The spline-shell mode also includes an inset planar interface: B is transformed
by diagonal scale `(.5,.5,-.5)` and translation `(.5,.5,0)`, joining a rectangle
strictly inside A's bottom face. The twelve-case test compares inset union,
common and cut; native union has 11 faces and retains the host interface trim.

An overlapping-interface fixture reflects B across Z and translates it by
`(1,.5,0)`. Its rectangular interface crosses A's host-face boundary. The
fifteen-case test compares union/common/cut geometry and shell counts without
requiring equal planar fragment counts, since OCCT may coalesce those fragments.

`transverse-spline-shell` independently sews a quadratic B-spline roof
`z=(5/16)^2-(x-.5)^2-(y-.5)^2` over `[.125,.875]^2` with floor `z=-.5`,
and uses OCCT Fuse/Common/Cut against the unit box. Bernstein control values,
not sampled native geometry, define the paraboloid exactly. The overlap volume
is analytically `pi*(5/16)^4/2`; the live OCCT reference differs by approximately
`1.94e-10`. The common-region centroid height is `(5/16)^2/3`.

The optional `TestOCCTTransverseSplineShells` is additionally gated by
`PLAINSOLID_OCCT_TRANSVERSE=1` because it requires roughly a minute per operation. Each operation has a
two-minute context budget. It
uses the public certified coverage path with analytic tube hints, never the
internal assembly bypass. Its mesh comparisons are explicitly approximate and
do not imply exact curved mass integration.

`two-contour-spline-shell` independently constructs a C0 quadratic roof with five
U poles and an internal multiplicity-two knot, over `[1/16,15/16] x [1/4,3/4]`.
Two paraboloid peaks of radius `5/32` intersect the unit box floor in disjoint
circles. Native OCCT common has two shells; union has one shell and one handle;
cut has one genus-zero shell. Euler calculations subtract face inner-wire counts
from `V-E+F`, since holed OCCT faces are not disk cells.

`TestOCCTTwoContourReference` verifies analytic volumes, centroid symmetry and
shell/Euler invariants of the independently authored OCCT fixture. Its native
counterpart is `trimmed.TestTransverseSplineTwoClosedContours`. This fixture
qualification does not claim a direct native curved-mass comparison.

`open-spline-box` independently constructs the unit box and the offset box
`[.5,1.5] x [.5,1.5] x [-.5,.5]` through OCCT primitives. The public native
`CombineOpenSplineShells` test builds degree-one spline shells and six analytic
face-pair tube sections with twelve segments. `TestOCCTOpenSplineShells` compares
union/common/cut volume, area and centroid and checks one shell, Euler 2 and twelve
shared section edges. Reference volumes are 1.875, .125, .875 respectively.

`multiple-open-spline-box` independently intersects the unit box with the square
bar `[.25,.75] × [.25,.75] × [-.5,1.5]`, producing two separate section wires.
`TestOCCTMultipleOpenSplineReference` checks analytic union/common/cut volume,
area and centroid against the primitive OCCT operations. The corresponding
native `TestCombineMultipleOpenSplineWires` authors the box using split spline
charts, exercises repeated splits on original edges and multiple cuts per bar
face, and independently checks those same invariants plus Euler characteristic
and retained-fragment ancestry. The OCCT test does not compare fragment counts,
since the reference can merge coplanar chart seams.

`repeated-spline-shell` independently authors a quadratic roof prism over
`[-.5,1.5]^2`, from z=-45/16 to `(x-.5)^2-1/16`, then applies native OCCT
Fuse/Common/Cut with the unit cube. `TestOCCTRepeatedSplineShellReference`
checks analytic volume, area and centroid integrals. The Go repeated-pair
fixture uses explicit, independently checked tube proposals; its regular
volume comparison is against a bounded closed sampled mesh, not exact native
curved integration. It verifies two common shells and a union handle.

`mixed-spline-shell` reuses the independently authored two-peak C0 roof but
intersects it with `[0,23/32] × [0,1]²`. The left lobe is retained completely
and the right lobe is cut in half through its center. The live optional test
checks analytic overlap `3πr⁴/4`, r=5/32, and union/common/cut shell/Euler pairs
1/0, 2/4, 1/2. It also checks the analytic common area and first moments.
Absolute volume tolerance is 1e-8; common area tolerance is 1e-7 and centroid-X
tolerance is 1e-6 because the reference trimmed-curve result differs from the
analytic moment by approximately 1.5e-7. This is an independent OCCT fixture;
the native mixed-shell tests check their separate construction.

`selective-fillet` uses OCCT's actual `BRepFilletAPI_MakeFillet` on a 4×3×2
box, selecting opposite vertical edges with radii .25 and .5. The optional
native differential checks analytic removed profile area, mass, centroid and
boundary topology. This development oracle additionally links TKFillet; the
Go library retains no external dependency.

`build_multi_plane_cuts.sh INCLUDE LIB OUTPUT` builds the independent
three-cap fixture. It intersects the unit box with three OCCT halfspaces,
checks that the result is one valid solid and reports volume, area, centroid,
and unique vertex/edge/face counts. Set `PLAINSOLID_OCCT_MULTI_PLANE_CUTS` to
`OUTPUT` and run `go test ./brep/trimmed -run '^TestSplineMultiPlaneCuts'` to
compare it against the native result. The ordinary tests also check exact
three-way seam ancestry and the volume of a body with an uncut curved face.
The same binary's `curved-band` mode independently sweeps an exact cubic
floor and applies two finite-height-separating, nonparallel halfspaces.
`TestClipSeparatedRationalGraphSplineShell` compares its measure and topology
with the native cubic band, also checked against independent integrals.
`crossing-band` mode intersects the two caps inside the swept stock, with
one end of their seam on the original cubic floor. The differential is
`TestClipCrossingRationalGraphSplineShell`.
`sleeve` independently cuts a square through-void in stock and intersects it
with two planes whose intersection traverses the void. The native comparison
uses Euler and material measures because the Go stock has split side charts.
`two-strips` independently sweeps a quadratic floor and clips it with two
planes. The counted original-floor trims remain separated on one support;
`TestRationalGraphTwoRetainedRunsOnOneStockFace` additionally checks analytic
volume, source identity, and native reload.
## Paired affine shell material

`affine_paired_shells.cxx` independently authors a stock box and its oblique
partner as a planar polygon swept over the original oblique vector. It checks
both input orders and all three material operations for a two-contact variant
and a manifold one-contact variant.

```sh
c++ -std=c++17 -O2 -I"$OCCT_INCLUDE" -L"$OCCT_LIB" -Wl,-rpath,"$OCCT_LIB" \
  tools/occtref/affine_paired_shells.cxx -o /tmp/occt-affine-paired \
  -lTKBO -lTKPrim -lTKTopAlgo -lTKBRep -lTKGeomBase -lTKG3d -lTKG2d -lTKMath -lTKernel
PLAINSOLID_OCCT_AFFINE_PAIRED=/tmp/occt-affine-paired \
  go test ./brep/trimmed -run '^TestOCCTAffinePairedMaterial$' -count=1
```

The native check verifies independent exact volumes against all 12 valid
OCCT outputs. It requires closed circular vertex fans for each native
single-shell result. The two-contact Cut therefore rejects in both input
orders. OCCT reports one and two solids for those cases respectively.
