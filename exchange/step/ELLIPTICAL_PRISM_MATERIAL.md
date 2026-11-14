# Exact native material after explicit ellipse-prism seam repair

`PromoteRepairedEllipticalPrism` re-authenticates the original closed source and
its explicit caller-bounded periodic repair, then returns a `trimmed.Solid`.
It does not treat the original rounded seam pcurve as exactly closed. The old
shell API and all exact/default STEP import paths keep their existing contracts.

## Exact construction and material proof

For each local quarter, the rational quadratic control points are
(a,0), (a,b), (0,b), with weights (1,1,2), rotated by cardinal quarter turns
before anisotropic scaling. In normalized parameter t:

```
x/a = (1−t²)/(1+t²)
y/b = 2t/(1+t²)
```

Thus (x/a)²+(y/b)²=1 identically. There is no rounded sqrt(1/2) weight.
Each quarter has a strictly increasing angle on t∈[0,1], stays in its own
quadrant, and meets adjacent quarters only at exact cardinal endpoints.
Extrusion is affine in the exact source height h>0. Four tensor side charts
therefore meet without crossings, and two plane-chart disks close the distinct
end planes with the same exact rational model curves. This construction gives
an embedded bounded ellipse prism with outward orientation; embedding is not
inferred from incidence alone.

Every world pole is formed by rational arithmetic on the source signed-cardinal
frame and required to fit binary64 exactly. A nonrepresentable required pole
fails explicitly. The normalized weights are exactly dyadic. `trimmed.Sew`
proves full continuous same-parameter edge identities; every use has certified
zero squared discrepancy. Tolerance controls native validation, never changes
source coordinates or permits rounding. The result contains 6 native faces:
lower plane, upper plane, then 4 rational side charts; 8 vertices and12 edges.

Native caps are installed before side faces so the authoritative shared rim
models are the explicit rational Bézier curves. This lets the existing exact
plane/curve and spline-isocurve checkers independently certify both owners.
No native checker or persistence codec was broadened for this operation.

## Ancestry and persistence

All four native side faces retain the original lateral ADVANCED_FACE ID. The
four native fragments on each rim retain that original full ellipse EDGE_CURVE
ID via `NativeRimEdges` and `SourceRimEdgeIDs`. Cap face IDs are retained. New
quarter seams are construction boundaries, not fabricated source edges.
The source U angle and native rational parameter are related by quarter-angle
plus 2 atan(t); these are different parameterizations and are not equated.

`WriteEllipticalPrismMaterialJSON` / `ReadEllipticalPrismMaterialJSON` use a
separate version-1 envelope retaining original STEP, enforced seam-repair
bound, native geometry, and face/rim ancestry. Both re-prove source and
regenerate exact native geometry; forged native payload, stale ancestry and
wrong tolerance fail atomically. Reader payloads are bounded to512KiB plus
existing STEP limits. Native support also survives ordinary trimmed JSON
reload independently. Re-export remains explicitly original STEP source,
including its original decimal phase; no repaired STEP writer is implied.

## Qualification

Focused tests require complete zero-error continuous edge certificates before
and after native reload, stable native recipe bytes, closed conforming mesh,
correct area/volume/centroid, cancellation and forged-payload rejection.
The pinned OCCT helper from [the shell milestone](REPAIRED_ELLIPTICAL_PRISM.md)
checks the retained source after native material roundtrip. Native quadrature
and independent OCCT agree on volume24π, area108.39617178072274 and
centroid(1,3.5,3). Numerical quadrature and mesh tests supplement, not replace,
the constructive exact material proof.

```sh
PLAINSOLID_OCCT_ELLIPTICAL_PRISM=/tmp/plainsolid-step-elliptical-prism \
  go test ./exchange/step -run TestOCCTEllipticalPrismMaterial -v
```
