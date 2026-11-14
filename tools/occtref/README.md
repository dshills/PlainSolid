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
