# Default OCCT fillet pcurve repair

`RepairPCurveOCCTConvexArcExtrusion` is an explicit bounded reconstruction of the independently authored single-fillet triangular prism in `testdata/occt-default-oblique-prism-fillet.step`. It does not widen the exact or model-only readers. The original AP214 source has 352 entities, twelve model edges and twenty-four independently owned pcurves. It was authored using OCCT revision `4dbfdcc448c0b3e565488afcd626c84483172db9`, `BRepFilletAPI_MakeFillet`, and `write.surfacecurve.mode=1`.

The supported graph has five planar faces and one cylindrical face, with one LINE or CIRCLE pcurve on each original owner. Each SURFACE_CURVE must prefer PCURVE_S1 and own exactly the two pcurves corresponding to its actual adjacent faces. All UV geometry, two-dimensional contexts, dependency ownership and retained model geometry are checked. Detached records, borrowed owner records, wrong parameter speeds, unsupported curves and conflicting preferred representations fail closed.

The existing model repair proves the original tangent-plane/cylinder construction and records the explicit displacement of each changed point or axial LINE origin. The additional proof compares each entire original owner lift with its corresponding model parameterization: exact affine coefficients for plane lines; exact sine/cosine coefficients for circles; and an exact rational Taylor enclosure for a cylinder line at constant angular U. The latter bound holds for the whole real parameter line because its axial derivative matches exactly. These are continuous proofs, not sampled endpoint comparisons. Original angular circle frames remain authoritative for this comparison even though the reconstructed model uses a geometry-identical canonical azimuth.

For the retained source, twenty-three owner bounds are zero. The remaining source U value `2.498091544797` yields a squared-distance upper bound approximately `2.3557294145279788e-26`. A caller distance budget of `1e-12` admits it; `1e-14` rejects it even though the three model-point corrections alone fit that smaller budget. The same caller budget separately bounds each repair site and each owner/model discrepancy; it is not an asserted combined global Hausdorff bound. This proof does not assert exact original finite trim closure or infer a missing parameter selector.

`OriginalSTEP` retains all original authoritative pcurves. `CorrectedModelSTEP` explicitly returns the checked corrected **model-only** graph, not a corrected version of the original pcurve graph. `Descriptor` can generate independently transferable exact model-only STEP via `WriteExactCircularLoopExtrusion`. The native reconstructed solid has its own exact rational boundary proof. The distinct JSON format `plainsolid-step-pcurve-convex-arc-repair`, version 1, retains original source, tolerance, source roles, all bounds and canonical native bytes; reload re-proves the source and compares all claims. It never parses untrusted rational claim strings before comparison.

The genuine default two-fillet file is retained as `occt-both-default-oblique-prism-fillet.step`. Its reversed-circle cap owners include rational quadratic pcurves with a different parameterization. This API rejects that graph rather than discarding the unchecked pcurves. Posed source graphs and grouped default-pcurve graphs are outside this milestone.

## Independent reproduction

Build the standalone helper (no dependency on the main OCCT oracle dispatcher):

```sh
sh tools/occtref/build_author_convex_arc_step.sh \
  /tmp/plainsolid-occt-build/include/opencascade \
  /tmp/plainsolid-occt-build/mac64/clang/lib \
  /tmp/plainsolid-convex-arc-step
/tmp/plainsolid-convex-arc-step write-default /tmp/default-fillet.step
/tmp/plainsolid-convex-arc-step write-both-default /tmp/default-two-fillets.step
PLAINSOLID_OCCT_CONVEX_ARC_ORIGINAL=/tmp/plainsolid-convex-arc-step \
  go test ./exchange/step -run '^TestRepairPCurveOCCTConvexArc' -count=1
```

The live test reauthors the original, persists/reloads the proof, and transfers original, corrected model and generated exact output through ordinary OCCT TransferRoots. Each is valid with one solid, six faces, volume `29.145041881053835` and area `66.186284791166074` within the independently stated numerical test tolerances. Mutations cover owner binding, phase/speed, preferred representation, context dimension, detached geometry, envelope claims and tolerance. Fuzzing is bounded by one MiB and fixed execution count (`-fuzztime=60x -parallel=1`).
