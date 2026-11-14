# Exact finite conical-frustum cavity material

`ReadConical45CavityMaterial` / `WriteConical45CavityMaterial` authenticate a
separate 39-record IGES type 186 material containing two separately owned, positive-Z,
expanding 45-degree frusta. Each shell has its original type 194 wall, two type 190
planes, two type 100 circular rims, one type 110 meridian and complete 502/504/508/510/514
incidence. The type 186 root selects the outward outer shell and explicitly negative
inner void. Both shells retain their own positive closed-shell orientation;
only their material use differs. No shared shell geometry, alternate topology,
pcurves, arbitrary placement or record reordering is inferred.

## Exact containment and orientation

The descriptor has `Outer` and `Inner` original frustum descriptors. Their XY
axis coordinates must agree exactly. Let Δz=Inner.Z−Outer.Z. The proof requires

- Δz > 0;
- Δz + Inner.Height < Outer.Height;
- Outer.Radius + Δz − Inner.Radius > 0.

All comparisons and sums in these three predicates use exact rational arithmetic
on the represented inputs. Because both walls have slope one, the radial
clearance is constant throughout the inner height interval. These inequalities
prove strict cap separation, radial containment and disjoint shell boundaries.
Touching caps, touching walls, inverted containment and eccentric axes reject.
Both original angles must be exactly 45 degrees; no rounded slope is inferred.
Source dimensions/endpoints must satisfy the existing lossless float gate.

Native promotion constructs each frustum independently from exact rational
circle-quarter tensor walls and exact caps, using the existing sewn material
builder. Each shell's complete continuous edge identities are re-proved. The
inner native face orientations are reversed and its topology IDs are rebased,
without merging vertices between shells. Strict source containment proves the
resulting two-shell material embedding. Every original shell, face, vertex list
and circular rim ID is retained, including native quarter-edge and face ancestry.
Native quarter parameterization is not claimed to equal original circle angle
parameterization. Numeric mass or mesh closure is an independent check, not the
substitute for these source and material proofs.

Source-bound version-1 native JSON repeats all source/geometry/ancestry proof and
compares the native boundary. Plain native trimmed reload is also qualified.
Source limits are 32 KiB, 512 records, 39 entities and 2048 values; native envelopes are 1 MiB.
All unsupported input and tampered payloads fail atomically.

## Independent qualification

The Python author uses no Go geometry or writer. Its outer frustum has radius 3
at z=0 and radius 7 at z=4; the inner frustum has radius 1 at z=1 and radius 3 at z=3.
Pinned OCCT transfers original and Go output as valid six-face, one-solid material.
Independent integrals give volume 290π/3, area π(68+48√2), centroid z=366/145.
Native measurements match; all shared native edge discrepancies are exactly zero,
inner face orientation is opposite, closed tessellation and native reload pass.
Mutations include void direction, reused outer shell, touching radius, cross-shell
support ownership, incorrect angle and source edge references. Focused live,
normal/race, Go 1.23, vet, previous conical/spherical cases and bounded 100-run fuzz
qualify this narrow analytic material family.
