# Mixed analytic cylinder stock and spherical cavity

`ReadCylinderSphereCavityMaterial` / `WriteCylinderSphereCavityMaterial` add a
separate canonical 39-record IGES material. The positive-Z cylinder is centered
on the origin axis, while the spherical cavity is centered at the origin. The
cylinder retains its original type 192 wall, two type 190 caps, complete type 100
rims, meridian and four degree-space pcurves. The sphere independently retains
its type 196 support, two exact pole vertices, type 100/124 meridian and four
original degree-space pcurves. One type 186 root owns the outward cylindrical
shell and the explicitly negative spherical void. No source geometry is shared
between shells or inferred from rounded coordinates.

The descriptor retains the original cylinder and sphere radius. Positive radii
and cylinder height are required. With cylinder lower station z0 and upper
station z1, strict containment is exactly

- Cylinder.Radius > SphereRadius;
- z0 < -SphereRadius;
- z1 > SphereRadius.

The cylinder must have exactly zero X/Y axis offset. Its upper station must be
representable without rounding; the inequalities compare exact represented
values. These predicates prove disjoint nested boundaries and exclude contact
with the cylindrical wall or either cap. Every source scalar, reference, sense,
endpoint, pcurve and transform is then rechecked against the complete typed graph.
Unexpected sharing, nonconcentric data, touching/inverted nesting and unknown
graphs reject atomically.

Native promotion combines independently validated material boundaries. The stock
uses exact rational circle-quarter tensor walls and planar caps, with zero
continuous edge discrepancy. The spherical inner face uses the previously
qualified analytic angular sphere convention and is explicitly reversed.
Its numerical native radian behavior is bounded separately; no exact rational
sphere-patch claim or equality with IGES degree parameters is made. Original
shell, face, vertex-list, sphere meridian and both four-pcurve ancestry groups
are retained along with native stock quarter-rims and spherical edge IDs.

Source-bound version-1 JSON reconstructs the entire source and native proof and
rejects stale geometry or ancestry. Plain trimmed reload and closed tessellation
are also tested. Limits are 32 KiB, 512 records, 39 entities and 2048 values, with
1 MiB native envelopes. Existing cylinder, sphere and cavity APIs are unchanged.

The independent Python fixture uses cylinder radius 3, z∈[-3,4], and sphere
radius 1. Pinned OCCT accepts original and Go output as valid four-face, one-solid
material. Independent formulas give volume 185π/3, area 64π and centroid
(0,0,189/370). Native mass and topology agree. Tests reject touching caps/wall,
oversized sphere, offset axis, wrong void direction, reused shell, cross-support
and vertex references, nonzero sphere center, forged ancestry and resource limits.
Focused live/normal, race, Go 1.23, vet, earlier sphere/cone cases and bounded fuzz
qualify this mixed-support material. Arbitrary placement/reordering are not added.

## Common proper signed-axis placement

`ReadPlacedCylinderSphereCavityMaterial` / `WritePlacedCylinderSphereCavity`
add a separate 40-record family. One proper signed-coordinate type 124 attaches
to the cylinder wall/caps/rims/model meridian and the sphere support. The sphere's
own type 100 meridian retains a distinct type 124, which must equal the exact
composition of the common pose with its original XZ circle frame. Both type 502
lists contain world vertices and are independently inverted before complete
local support/containment proof. Cylinder and sphere degree pcurves remain UV
records; neither family may acquire a world transform.

Native cylinder controls are rebuilt with exact rational coordinate sums in the
posed frame, retaining zero continuous edge discrepancy. The sphere retains the
qualified bounded native angular convention; all six cardinal radius witnesses
must map losslessly. It is transformed and reversed as the void, then joined at
the original caller tolerance without repeated tolerance inflation. Native pole
coordinates, sphere radius/seam agreement and stock edge identities are tested
separately. Original source vertices, shared and composed transform ownership,
local ancestry and both native edge groups survive source-bound/plain reload.

The independent `--placed` source maps the axis to +X with sphere center
(10,-3,5). Pinned OCCT imports original and generated output as valid four-face,
one-solid material with volume 185π/3, area 64π and centroid
(10+189/370,-3,5). Tests reject detached inner support/meridian, disagreeing composed
frame, shifted sphere datum, world transforms on either degree-pcurve family,
forged native pose and resource limits. Focused normal/live, race, Go 1.23, vet,
earlier mixed/spherical cases and bounded fuzz qualify the separate placement
path. No record reordering is included.

## Independently reordered source graphs

`ReadReindexedCylinderSphereCavityMaterial` additionally authenticates arbitrary
DE addresses/order, per-shell vertex/edge/face-list permutations, and cyclic
starts of the four-use cylindrical and spherical loops. Roles come from exact
support geometry and directed incidence, never sorted source IDs. A bounded
bijection restores the canonical proof, including every original degree pcurve
and transform; the canonical readers remain unchanged. Cross-shell geometry
sharing, cycles, inconsistent void direction, changed uses, and UV transforms
are rejected. Only the previously qualified proper signed-axis placement is
admitted.

The result retains original record IDs, vertex/edge/face ordinals, cyclic use
ordinals, native face source IDs, model curves and both degree-pcurve families.
Source export and the versioned JSON envelope reprove these identities before
writing; JSON reload reconstructs the maps from the original source bytes.
Cylinder continuity remains exact; the spherical native angular evaluation has
the same documented bounded discrepancy as the canonical codec. Reordering
adds no general IGES acceptance or new geometric approximation.

`tools/occtref/reorder_iges_cylinder_sphere_cavity.py` independently reverses DE,
vertex, edge and face lists and rotates the four-use loops in the hand-authored
placed/unplaced fixtures. Pinned OCCT 4dbfdcc448c0b3e565488afcd626c84483172db9
loads both as valid one-solid/four-face materials: volume 185π/3, area 64π,
centroid (0,0,189/370), or (10+189/370,−3,5) after placement. The qualifying
comparator is `tools/occtref/iges_topology_solid.cxx`, selected by
`PLAINSOLID_OCCT_IGES_SOLID`; this is a material transfer check, not merely a
surface import. Focused normal/live/race, Go 1.23, vet and a bounded 100-execution
fuzz campaign cover this additive API.
