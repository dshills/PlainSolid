# Native analytic STEP export of promoted elliptical prism

`WriteEllipticalPrismMaterialSTEP` authors a new rooted AP214 model-only
elliptical prism from a validated `EllipticalPrismMaterial`. It is separate from
`WriteEllipticalPrismShellSource`, which preserves original rounded pcurves.
The material's complete original proof, exact native quarter geometry and
ancestry are regenerated and compared before any output bytes are written.

The new 76-record graph retains:

- one MANIFOLD_SOLID_BREP / CLOSED_SHELL with three ADVANCED_FACE records;
- the analytic SURFACE_OF_LINEAR_EXTRUSION and its original natural ELLIPSE;
- two exact cap PLANE supports;
- two shared natural full ELLIPSE rim model edges and one shared LINE seam;
- both opposite uses of the seam and correct cap/lateral face/bound senses;
- a complete product/context/unit graph, with a new PlainSolid file header.

EDGE_CURVE refers directly to the model LINE/ELLIPSE. Unreachable SURFACE_CURVE,
SEAM_CURVE, PCURVE and 2D representation records are removed. No decimal 2π
phase, synthetic spline ellipse approximation, finite PARAMETER trim or
new repair tolerance is emitted. This is a native authoring operation in the
independently qualified model-only dialect, not a relaxed import path. Existing
strict 121-record repair readers intentionally reject this different dialect.

`EllipticalPrismSTEPOptions` supports a new product name and exact translation.
Empty name preserves the original name; units remain millimeters. Before
writing, rational addition proves every translated model/source point and every
native quarter control point is exactly representable. Nonrepresentable
translations, invalid names, nonfinite data and stale native geometry/ancestry
fail atomically. Input native/source objects are never modified. General
rotation, scaling, arbitrary edits, unit conversions and unrelated solids are
not admitted by this API.

The copied source context's fixed uncertainty record is ordinary format
metadata; it neither grants repair permission nor changes any model coordinate.
The original source remains accessible separately and still records the bounded
repair that was necessary to create the native material. This generated STEP
file describes the new native geometry, not an exact equivalence claim about
that old authored decimal seam.

## Pinned independent qualification

Using OCCT `4dbfdcc448c0b3e565488afcd626c84483172db9` and the standalone helper
from the closed shell milestone:

```sh
PLAINSOLID_OCCT_ELLIPTICAL_PRISM=/tmp/plainsolid-step-elliptical-prism \
  go test ./exchange/step -run TestOCCTWriteEllipticalPrismMaterialSTEP -v
```

The chain genuine OCCT source → authenticated repair → exact native solid →
native JSON reload → newly generated model-only STEP passes conventional OCCT
TransferRoots and BRepCheck. Both zero translation and (8,−2,4) have one solid,
three faces, six edge uses, volume24π and area108.39617178072274. Centers are
(1,3.5,3) and (9,1.5,7), respectively. Go's integration of original native
rational charts and independent OCCT evaluation agree. Tests also inspect the
76-record graph, absent pcurves/rounded phase, direct seam model, escaped name,
source immutability, old-reader separation and atomic rejection.
