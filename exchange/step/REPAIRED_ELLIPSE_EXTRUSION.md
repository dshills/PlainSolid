# Explicit repaired elliptical extrusion face

`RepairRoundedEllipseExtrusionFace` reconstructs one genuine OCCT-authored
`OPEN_SHELL` face on `SURFACE_OF_LINEAR_EXTRUSION`. This is an **opt-in repair**,
not exact STEP import, a material solid, or a generic swept-face reader.

The retained 89-entity fixture is generated independently by OCCT from an ellipse
wire extruded three units. The ellipse has center (1,2,3), reference −Z, normal
+Y, and radii 4 and 2. OCCT chooses extrusion direction −Y and V ∈ [−3,0].
Both rims are original natural ellipses. One exact model LINE is used twice:
forward on U=0 and backward on authored U=6.28318530718. Its two PCURVEs are
separate, owned by the same original extrusion support. The decimal period is
not mathematical 2π, so exact lifted closure is false.

## Supported proof

The complete rooted graph is matched bijectively, preserving arbitrary source
entity numbering and bounded names. Every entity belongs to the qualified
shape/product/context; no detached geometry or cross-owner pcurve is ignored.
Both seam coedges, their opposite senses, both natural rim uses, face/bound
orientation, model endpoints, and all four original pcurves are independently
checked against the source topology. The only numeric template substitutions
are re-derived geometry and the explicitly bounded seam period.

The source profile has a signed-cardinal right-handed frame and radii a>b>0.
The extrusion is the negative unit profile normal. Required world points and
height must be exactly binary64-representable from exact decimal arithmetic;
no rounded coordinate sums are accepted. Source units are millimeters and
radians. Other metadata/unit graphs remain unsupported.

For every v and normalized s ∈ [0,1], compare the authored chart S(P*s,v) to the
reconstructed chart S(2π*s,v). Its displacement is at most a*|P−2π| because the
ellipse speed is at most a. Exact source rationals and certified rational π
bounds enclose the signed period error and this physical upper bound. The same
bound applies to the second seam's disagreement with the authoritative LINE.
The first seam and both natural rim supports agree exactly. Caller tolerance
must be positive, finite, no more than b/1,000,000, and at least the certified
bound (about 1.655e−12 model units for this fixture).

No finite trim parameter is invented for the untrimmed seam LINE: native segment
selection is a reconstruction from the exact endpoint pair and monotonic V.
The periodic rectangle has exact natural ellipse rims and identified seam uses;
positive radii and strict nonzero axial height make its quotient an embedded
open side face. Its source surface normal is preserved; no outward material
orientation is asserted. Evaluators use ordinary floating-point trig; the
mathematical repair bound does not include evaluator roundoff.

The returned native face uses an analytic unit cylinder under an exact diagonal
ellipse/frame affine map, not an approximate rational ellipse. Its U domain is
[0,2π], V domain [−Height,0], and edge roles are seam, upper rim, lower rim.
`WriteRepairedEllipseExtrusionFaceJSON` and its reader persist a version-1 repair
recipe, original STEP source, enforced tolerance and reconstructed native
support. Reload repeats the full proof; stale support, uses or ancestry fail.
This is not a repaired STEP writer. Retained original STEP remains original and
still contains its rounded phase; native serialization does not claim otherwise.

Inputs are bounded to 64 KiB, 89 entities, 4096 values and depth 16; smaller
caller limits apply. Native recipes are bounded to 256 KiB. Rejections return
no partial result.

## Independent qualification

Pinned OCCT commit `4dbfdcc448c0b3e565488afcd626c84483172db9`:

```sh
sh tools/occtref/build_step_ellipse_extrusion.sh "$OCCT_INCLUDE" "$OCCT_LIB" /tmp/ellipse-extrusion
/tmp/ellipse-extrusion write /tmp/source.step
PLAINSOLID_OCCT_ELLIPSE_EXTRUSION=/tmp/ellipse-extrusion go test ./exchange/step -run 'TestOCCTRepairEllipseExtrusion' -v
```

Source and source retained through native reload transfer through ordinary
`TransferRoots`, pass `BRepCheck_Analyzer`, and have one face and four edge uses
(three unique edges). Independent numerical integration of OCCT surface
derivatives gives area 58.130689323286056 and center (1,3.5,3), matching Go's
independent analytic speed integral times height. This numerical differential
is validation evidence, not the exact acceptance proof.

OCCT's `BRepGProp::SurfaceProperties` is observably inaccurate on this source:
default area ~58.1325761185 and adaptive area ~57.7353159787. The helper reports
`LibraryArea` separately and does not silently substitute it for proof. Edge
lengths computed independently by OCCT total 44.753792882190709 counting the
seam twice. Mutation tests cover either seam sense, multiplicity, foreign owner,
period, floor height, extrusion direction, rim radius, endpoint, face sense,
units, undersized/oversized tolerance and bounded parsing. A translated rotated
signed-coordinate fixture and changed height verify geometry is not hard-coded.
