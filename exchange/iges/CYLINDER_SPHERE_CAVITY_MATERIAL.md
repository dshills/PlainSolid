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
