# Explicit tangent repair of an original OCCT prism fillet

`RepairOCCTConvexArcExtrusion` is a separate opt-in reconstruction API for a model-only six-face extrusion: five planes, one +Z cylindrical wall, four profile segments, eight vertices and twelve model edges. It accepts the existing exact circular-loop product's AP214 metadata and units contract. It does not widen `ReadOCCTExactCircularLoopExtrusion` or the canonical `ReadConvexArcExtrusion`.

## Independent source defect

`tools/occtref/author_convex_arc_step.cxx` builds the original 3–4–5 triangular prism with `BRepPrimAPI_MakePrism`, then applies `BRepFilletAPI_MakeFillet` at radius 5/16 to its acute vertical edge. OCCT revision `4dbfdcc448c0b3e565488afcd626c84483172db9` writes AP214 with `write.surfacecurve.mode=0`. The retained 172-entity file is `testdata/occt-model-only-oblique-prism-fillet.step`; only trailing whitespace is removed.

OCCT emits two tangent vertices and their vertical LINE origin at Y=−1.110223024625E−16, although the original wall plane is Y=0 and the circle center/radius require a tangent at Y=0. Exact source import correctly rejects. The file is nevertheless valid under OCCT's tolerance model. The new route never treats these original decimals as zero.

## Proof and limitations

For each cylinder-incident vertex, original incidence must identify exactly one horizontal cap and one axial plane. With cylinder center C, plane point Q and plane normal N, exact rational arithmetic proves

`((C−Q)·N)² = radius² (N·N)`.

The unique tangent point is `C−N*((C−Q)·N)/(N·N)` at the cap's original Z station. Only its original vertex record may move, and its exact squared displacement must fit the caller's positive finite `MaxDistance`. That tolerance cannot exceed radius/1,000,000. The original axial model line may translate only if both endpoint corrections are identical and the original endpoints lie on that line. Its entire infinite support then has the same uniform displacement bound. Other model lines, planes, circles, cylinders, dimensions and units are not fitted or moved.

Only exclusively owned cardinal azimuth directions on +Z circles/cylinders may normalize to +X. This preserves the complete mathematical support and positive circle orientation. No source pcurves are permitted; a pcurve would make a reference-azimuth change semantically different. Reverse references forbid moved points from serving any unrelated support/frame or non-axial curve.

The corrected whole source is passed through the existing exact circular-loop importer. That proof rechecks product ownership, every original edge/vertex/face incidence and directed finite minor arc, exact circle membership, support agreement and the native exact material. There are no guessed original source roles. All original IDs remain unchanged and are exposed through copied provenance.

The result is newly reconstructed material. Reported bounds concern original point sites and the shifted axial line, not an inferred Hausdorff bound for the original inconsistent B-rep or a claim that its finite parameter selection was already exact. Original source bytes remain available separately. Corrected native rational charts have zero continuous shared-edge discrepancy; the original source does not.

## API and qualification

The opaque result exposes `OriginalSTEP`, `CorrectedSTEP`, `Descriptor`, `Solid`, `Repairs`, `AzimuthDirections` and `Provenance`. Rational evidence and slices are copied. `WriteExactCircularLoopExtrusion` can export its returned descriptor through the existing exact model-only writer; the canonical exact reader accepts that generated file. Native geometry reload is independently tested. Source-bound repair-envelope persistence is not part of this slice.

Input is bounded to 1 MiB, 1,024 entities, 8,192 values, depth 16 and exact source coordinates at most 256 numerator/denominator bits, with at most six moved records. Caller limits can reduce these. Validation failures return no partial result. Tests cover too-small/invalid tolerances, changed circle/cylinder supports, inconsistent edge sense, wrong owner, nonparallel line, unequal endpoint corrections, forbidden pcurves, cancellation and limits. The old strict readers continue rejecting the original.

Build and run the independent comparator:

```
sh tools/occtref/build_author_convex_arc_step.sh "$OCCT_INCLUDE" "$OCCT_LIB" /tmp/plainsolid-convex-arc-step
PLAINSOLID_OCCT_CONVEX_ARC_ORIGINAL=/tmp/plainsolid-convex-arc-step go test ./exchange/step -run '^TestRepairOCCTConvexArc' -count=1
```

The live test independently reauthors the original, then transfers original/corrected/generated files using ordinary `TransferRoots`, validity and precise mass checks. All are one solid with six faces, V=29.1450418810538 and A=66.1862847911661 (within 1e−8 of independent cross-section formulas). Exact native shared-edge proof and native JSON reload are separate checks. A fixed 60-execution single-worker fuzz campaign exercises the original and malformed sources.
