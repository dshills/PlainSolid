# Natural sphere products in independent STEP collections

`ReadIndependentSphereMaterials` / `WriteIndependentSphereMaterials` extend the
explicit independent-product workflow with full outward natural spheres. At least
one sphere is required. Other members use the established planar, elliptical and
model-only cylinder proofs; existing APIs and native envelope tags stay strict.
No NAUO assembly, placement inference, Boolean union or disjointness is claimed.

The sphere's original body dependency graph must contain exactly one closed
shell, outward advanced face, forward FACE_BOUND, VERTEX_LOOP and pole vertex,
one SPHERICAL_SURFACE, two points, two directions and one placement. Complete
product ownership is established before this graph is inspected. Centers,
radii and source poles must be losslessly represented binary64 numbers; frame
directions are orthogonal signed coordinate units. Exact rational subtraction
proves that the authored vertex is precisely one of the two poles. Near-pole
values accepted by a tolerance-only test are rejected. Extra trims, changed
owner links, multiple faces, inward senses and unsupported graph types fail.

Native promotion deliberately uses the existing analytic parametric sphere,
then `trimmed.FromParametric`. It preserves the exact sphere locus/center/radius,
canonicalizes the seam, and records every native face's original source face.
It does **not** claim exact rational trigonometric evaluation or zero-discrepancy
rational sewing. The original pole and its graph IDs remain separate ancestry.
Native trimmed JSON reload is qualified. The separate
`Write/ReadIndependentSphereMaterialsJSON` API uses the distinct
`plainsolid-step-independent-sphere-materials` version-one tag. One strict source
reproof reconstructs all members before native bytes and every product/shape/role
claim are compared. Envelope allocation and STEP limits remain separate, failure
is atomic, and older tags are not widened. Original source re-export after native
reload is independently qualified with OCCT.

`occt-independent-sphere-product.step` is the original pinned OCCT sphere export
(center 1,2,3; radius 2), retaining its south-pole topology and standard AP214
product chain. The seven-product fixture appends these original records with
only reference-ID rebasing to the previous six-product collection. The original
and generated files transfer as seven valid solids, 27 faces and 88 edge uses.
Aggregate volume is `24+(135.25+32/3)*pi`; area is
`522.168698195557+56*pi`. The independent comparator evaluates planar/ruled
faces as before and checks the sphere's full natural parameter domain before
using its OCCT analytic radius for sphere measures. It separately reports OCCT
library volume. Set `PLAINSOLID_OCCT_INDEPENDENT_SPHERE_MATERIALS` to the binary
built by `tools/occtref/build_step_independent_sphere_materials.sh`.
