# Exact Pythagorean in-plane ellipse prism STEP

`ReadPythagoreanEllipticalPrism` is a separate model-only material reader for a
noncardinal ellipse reference direction and a signed-cardinal extrusion axis.
It preserves exact rational source axes, never a rounded `geom.Frame`.
`WritePythagoreanEllipticalPrismSTEP` authors this dialect from an explicit
origin, direction ratios, signed unit axis, positive radii a>b and height.
Existing cardinal readers/writers and repaired-source paths are unchanged.

## Exact frame and control gates

The source direction ratios are parsed as bounded rationals. Their dot product
with the axis must be exactly zero, and their squared norm must have an exact
rational square root (integer-square numerator and denominator). The normalized
reference and cross-product minor axis are retained as private owned rationals;
`RationalFrame()` returns copies. All original ellipse/plane references must
retain the same declared ratios, and source axis/extrusion declarations remain
signed unit coordinate vectors. Proportional differently declared owner frames,
irrational normalization and almost-orthogonal source decimals are unsupported.

The full 76-record graph and its exact natural rim/seam/cap geometry are proved
as for the cardinal model-only family. The native plane charts are independent
cardinal charts, not rounded copies of the source plane parameterization.
Rational source axes are used to compute world and cap-UV quarter controls with
exact arithmetic. **Every required control must fit binary64 losslessly**.
Weights(1,1,2) remain exact; continuous sewing proves every model/UV use with
zero discrepancy. There is no sampled fitting or tolerance-based normalization.

For the independent 3:4:5 fixture, normalized axes are (3/5,4/5,0) and
(−4/5,3/5,0), radii10/5 and height3. All resulting quarter controls are integers,
so no exact-homogeneous fallback is needed. Radius choices producing controls
such as7/5 fail explicitly with unrepresentable data; broader arbitrary rational
native control support is not claimed by this API.

The writer's `Reference` contains represented direction ratios: use `(3,4,0)`.
Passing binary64 `(.6,.8,0)` is a different pair of exact rational numbers and
is rejected when its norm is not rational. In contrast, the original STEP
lexemes `0.6,0.8` mean exactly3/5,4/5 and are accepted by the reader. No silent
conversion between these meanings occurs. Source units are millimeters/radians.

The native result contains exact original-support geometry in four tensor walls
and two plane caps, with original body/shell/face/model-edge ancestry and rim
fragment maps. Ordinary trimmed JSON reload preserves the native exact
material. `WritePythagoreanEllipticalPrismSource` re-proves and retains original
source ratios/IDs. Native authoring creates a new PlainSolid header/product.

Inputs are bounded to64KiB,76entities,4096values,depth16 and the fixed native
6faces/8vertices/24uses. Smaller caller limits apply. Source proofs, geometry
construction and writer validation fail atomically. The private common sewing
helper was factored without changing prior cardinal native control data.

## Authentic independent qualification

Pinned OCCT `4dbfdcc448c0b3e565488afcd626c84483172db9` creates an ellipse with
`gp_Dir(3,4,0)`, radii10/5, center(1,2,3), normal+Z, then extrudes its disk3 units.
The standalone helper replaces typed `StepShape_EdgeCurve` surface-curve
wrappers by their original `Curve3d` handles and rebuilds references using
OCCT's own model writer. It does not consume Go-authored numeric tokens.
The retained 76-record source contains exact decimal ratios0.6/0.8.

```sh
sh tools/occtref/build_step_pythagorean_elliptical_prism.sh "$OCCT_INCLUDE" "$OCCT_LIB" /tmp/pythagorean-prism
/tmp/pythagorean-prism write /tmp/original.step
PLAINSOLID_OCCT_PYTHAGOREAN_ELLIPTICAL_PRISM=/tmp/pythagorean-prism \
  go test ./exchange/step -run TestOCCTPythagoreanEllipticalPrism -v
```

Original, source re-export and independently native-authored3:4-ratio output all
pass conventional OCCT TransferRoots/BRepCheck with one solid and three faces.
Native and OCCT measurements agree on volume150π, area459.485988667194 and
center(1,2,4.5). Native control/topology bytes match between the independently
loaded source and native-authoring route. Tests include proper signed placement,
source-owner/endpoint mutations, near-rational and opposite reference errors,
nonrepresentable poles, ownership of returned rational arrays, native reload,
continuous certificates, budget failures and bounded fuzz.
