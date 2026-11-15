# Canonical convex line/circle extrusion STEP

`ConvexArcExtrusionBody` retains a name, proper signed-coordinate sketch frame, positive height and counterclockwise line/circular-arc segments. `BuildConvexArcExtrusion` uses the existing native `trimmed.ExtrudeArcLoop`; `WriteConvexArcExtrusion` emits its actual planar/cylindrical topology, model curves and both owner pcurves. `ReadConvexArcExtrusion` reconstructs the recipe and requires complete canonical graph equivalence. This is an explicit canonical dialect, not a vendor-tolerant analytic reader.

The bounded profile contains 3–32 segments, at least one line and one positive minor arc, and at most four circular runs. Directed tangent and chord half-plane tests plus the native constructor restrict the profile. Up to 50 faces, 144 edges and 96 vertices are supported. Source parsing is capped at 1 MiB, 8,192 entities, 65,536 values and depth 16. Caller limits can reduce these bounds.

The reader recovers original cap frame/height, planar wall endpoints, cylinder center/radius and original vertical-owner pcurve angles. Adjacent cylinder charts are joined only when center, radius, endpoint and chart angle agree. Sweep subtraction can lose one binary64 ULP; at most three adjacent represented candidates per arc are considered (81 total). Every original subdivision angle and complete regenerated graph must match. This is finite canonical reconstruction, not approximate fitting or permission to change source supports. Unsupported graph records, ownership, pcurve modifications, flipped senses or detached geometry reject the whole result. Output is buffered and validated before caller writes.

## Numerical contract

These APIs deliberately retain `ExtrudeArcLoop` semantics. General circular arcs use bounded trigonometric endpoint anchoring, including internal analytic chart splits, rather than an exact algebraic incidence certificate. Native geometry reconstruction uses tolerance `max(1,height,radii)*1e-10`; endpoint discrepancies are additionally limited by the constructor's 1,024-ULP gate. Analytic area/perimeter and floating integration are estimates. Exact source-record regeneration does not imply zero geometric endpoint discrepancy. Signed frames or translations that cannot construct represented native coefficients fail rather than round them. There is no STEP assembly or negative-height normalization in this API.

## Qualification

The original native 3–4–5 triangular prism fillet at radius 5/16 supplies the profile through `ConvexPrismFilletsHistory.Section`. It retains three planar walls, four cylinder charts and two caps. Tests compare native reload and canonical STEP roundtrip, two axis orientations, and a second adjacent radius-1/4 fillet. Existing quarter-sector and circular-lens tests remain unchanged.

Optional `PLAINSOLID_OCCT_CONVEX_ARC_STEP` points to the pinned standalone STEP reader built with `tools/occtref/build_author_ap242_spline_roof_void.sh`. Its `read` command uses ordinary `STEPControl_Reader::TransferRoots`, `BRepCheck_Analyzer` and precise surface/volume integration. Both generated poses must be valid one-solid/nine-face material. With r=5/16 and w=atan2(3,-4), independent cross-sectional formulas are A=6−r²(3−w/2), P=12−6r+rw, V=5A, surface=2A+5P. The helper's roof-authoring mode is not used as substitute geometry.

Mutation tests alter cylinder radius, plane type, owner pcurve, edge sense and detached records. A fixed 60-execution single-worker fuzz campaign exercises bounded raw decoding. No existing analytic reader acceptance is widened.
