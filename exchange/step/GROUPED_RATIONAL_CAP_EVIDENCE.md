# Original grouped fillet rational cap evidence

`InspectGroupedConvexArcPCurves` is an additive evidence API for the genuine default OCCT two-fillet source retained in `testdata/occt-both-default-oblique-prism-fillet.step`. It does **not** import or repair the full pcurve-bearing material. Existing exact, model-only repair and single-fillet pcurve repair APIs retain their restrictions.

The complete source ownership graph is authenticated before making a temporary model-only proof view. The existing grouped model repair independently re-proves the five planes, two cylinders, original cap/wall/rim/vertical roles, unique tangent ownership and matched reversed circle/edge/both-coedge senses. Every removed pcurve dependency must belong to the registered owner closure; no retained record may borrow it. The source has thirty owner pcurves. Twenty-eight have the continuous LINE/CIRCLE proof from the single-fillet mode, with an additional exact negative U speed for a reversed-circle cylindrical owner: equal cosine basis and opposite sine basis prove `U=-t` without changing the original circle frame.

The two remaining plane owners are original periodic rational quadratic B-splines. The gate requires exactly seven poles and positive weights, the authored multiplicity pattern `[1,2,2,2,2,1]`, strictly increasing six knots, and complete source records. It uses exact rational de Boor restriction and Bernstein reconstruction over all three nonempty source spans. No rounded knot or pole is replaced. Each homogeneous lifted pole lies exactly in the model-circle plane. The proof then bounds

```
|X² + Y² - r²W²| / (min(W)² r)
```

over each complete span. This bounds Euclidean distance to radial projection onto the circle because `rho+r >= r`. Strict positivity of every Bernstein coefficient of `X Y' - Y X'` proves positive angular velocity in the **original circle's** oriented frame. Exact closure plus first-span upper-half-plane, middle-span strictly-left-half-plane and final-span lower-half-plane constraints establish exactly one crossing of the positive radial ray and one full positive turn. Nonzero radial coordinates follow from the strict angular numerator proof. Adjacent span continuity follows from the original degree-two spline's multiplicity-two interior knots. Exterior knots are retained; the active domain is the original `[0,6.28318530718]`, not an asserted exact `[0,2*pi]` angular domain.

For this source both exact rational radial displacement upper bounds are approximately `7.59758481344e-13`. The caller's repair tolerance separately bounds these radial projections and the other twenty-eight same-parameter owner discrepancies; it does not bound an asserted global repaired trim or source material Hausdorff distance. At source parameter `2094395102393/4000000000000`, exact evaluation of the first rational span and a certified rational sine/cosine enclosure prove squared distance to the angular model of at least approximately `9.248983878364686e-05`. Thus the same-parameter tiny-error premise is demonstrably false, despite excellent support-circle agreement. Pole IDs, original knots/weights, both source owners and original reversed-edge/coedge IDs remain in the report. Reports are independent caller-owned observations, not acceptance certificates or mutable imported solids.

## Independent qualification and remaining gap

The standalone helper is built using `tools/occtref/build_author_convex_arc_step.sh` against pinned OCCT `4dbfdcc448c0b3e565488afcd626c84483172db9`, as described in `REPAIRED_PCURVE_OCCT_CONVEX_ARC.md`. `write-both-default` authors the source using BRepFilletAPI and enabled pcurves. Ordinary `read` transfers a valid one-solid, seven-face material with volume `29.077978807115535` and area `65.622954970084379`.

The new `cap-witness` mode reads the actual original STEP entities, obtains each rational plane pcurve and associated model CIRCLE through their SURFACE_CURVE owner, converts those original entities through OCCT StepToGeom, and evaluates both at the witness parameter. Both squared discrepancies are `9.2489838783647467e-05`, independently confirming the exact lower bound. It does not use transferred/healed edge pcurves for this measurement.

Run:

```sh
PLAINSOLID_OCCT_CONVEX_ARC_ORIGINAL=/tmp/plainsolid-convex-arc-step \
  go test ./exchange/step -run '^Test(GroupedRationalCapEvidence|RepairPCurveOCCTConvexArc)' -count=1
```

The remaining ingress obligation is a proven **nonlinear reparameterized finite trim selection** connecting the original EDGE_CURVE endpoint/sense declaration to the rational pcurve's selected arc and its cylindrical owner. A full-domain oriented circle projection alone does not establish this. No native/source equivalence, corrected pcurve graph, source-bound material persistence or repaired grouped import is claimed. Tests retain strict rejection of the source by existing repair readers, and reject changed poles, closure, weights, knots, owners, traversal, individual senses, undersized tolerance, oversized inputs and cancellation. A fixed 60-execution fuzz target checks transactional failure.
