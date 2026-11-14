# Sphere-inclusive independent STEP hierarchy

`Read/WriteIndependentSphericalHierarchy` add zero or one independently owned
exact standalone sphere product to the required planar/cylinder/model-only-torus
local materials. Sphere occurrences may be shared and translated/placed through
the existing bounded hierarchy. Every local product must be used, and the torus
must be reused. The older three-family hierarchy APIs and native formats remain
unchanged and reject the sphere-bearing graph.

The new mode retains at most 16 products, 64 source relationships, eight levels,
64 expanded leaves, proper represented signed-coordinate rigid transforms,
common explicit metre/mm units, original root/product/NAUO path IDs and exclusive
local geometry closures. All source and native face/vertex/use budgets remain
active, and MaxSolids counts the three or four distinct local materials (not
repeated occurrences). No spatial disjointness or material union is asserted.

The sphere uses the existing `readIndependentExactMaterial` proof. The retained
independent fixture's genuine OCCT sphere has one ADVANCED_FACE on a natural
SPHERICAL_SURFACE, a VERTEX_LOOP and its exact authored south pole; it does not
invent a rational seam that the source never supplied. The existing native
analytic sphere construction/reload and source support/pole/frame checks are
reused. This is not a claim of exact rational tensor-patch sewing, nor a tolerance
repair of arbitrary vendor sphere trims. Radius, pole, support ownership and
orientation must pass the existing selected sphere gate.

`WriteIndependentSphericalHierarchySource` re-proves and retains original IDs.
`Write/ReadIndependentSphericalHierarchyJSON` have separate version-one tags:
`plainsolid-step-independent-spherical-hierarchy` and inner
`plainsolid-step-independent-spherical-hierarchy-claims`. The envelope retains
source, graph, expanded paths, one native body per local product, and explicit
sphere frame claims. One complete source proof checks all fields and canonical
native encodings before output/acceptance. The old three-family tag is not reused.
Source stays bounded to 1 MiB/4096 entities/65536 values/depth16 and native output
to 16 MiB, with caller source limits and the existing 1024-entity per-local native
claim cap. Cancellation and invalid data return no partial result.

## Independent evidence

`author_step_independent_spherical_hierarchy.py` independently adds the original
body closure from `occt-independent-sphere-product.step` to the AP214 three-level
assembly. It does not call the Go writer. The sphere has center (1,2,3), radius2,
and two occurrences translated by (-20,10,0) and (0,-20,10) mm. Original product
sharing is retained in native persistence and generated standard relationships.

Pinned OCCT `4dbfdcc448c0b3e565488afcd626c84483172db9` ordinary TransferRoots
validates original fixture, Go-authored output and native-reexported original:
7 solids, 22 faces, 88 edge uses;

- V = 24 + 12π² + (24+64/3)π mm³
- A = 52 + 24π² + 72π mm²
- bounds [-21,-20,-2]..[30,30,15] mm.

Surface centroid is independently computed from the known original local
materials and both sphere centers, so the check exercises translations as well
as rotation-invariant measures. Numerical oracle comparison tolerance never
widens source proof.

Focused live/legacy regression, race, Go1.23, vet and two fixed100-execution fuzz
campaigns qualify source/native parsing. Mutations cover radius/pole mismatch,
foreign frame ownership, duplicate sphere local products, changed paths and
sphere frames/native payloads/tags, limits and cancellation. A sphere-free
hierarchy remains valid under the new optional-family mode and still preserves
its older API/tag semantics independently.

Live comparator environment:
`PLAINSOLID_OCCT_INDEPENDENT_TORUS_ASSEMBLY=/tmp/plainsolid-independent-torus-assembly-oracle`.
