# Concentric spherical IGES material with one void

`ReadSphericalCavityMaterial` / `WriteSphericalCavityMaterial` implement a
separate canonical 31-record IGES type 186 family. Two independently owned
spherical shells use the exact original type 196 / pole / meridian / degree
pcurve graph described in `SPHERICAL_MATERIAL.md`. The outer shell is positive
in the 186 record; the inner shell is explicitly a negatively oriented void.
Each shell's own 514 face use remains positive. Sharing the shells, pole lists,
model meridians, pcurves or supporting datums is not accepted by this contract.

The original supports must be concentric at the origin, positive-Z/cardinal
reference, with exact positive radii satisfying OuterRadius > InnerRadius.
This strict inequality proves disjoint spherical boundaries and a material
region between them; equality, inverted or nonpositive radii reject. Both pole
lists, both meridian arcs and all eight degree pcurves are independently checked
against their own radius, endpoints, direction, placement and original owner.
All records and reference fields must match the complete derived geometry graph;
no tolerance fitting, partial source acceptance, placement or reordering is added.

Promotion uses the existing analytic concentric sphere difference and trimmed
representation, explicitly checking outward outer and inward inner native faces.
Its numerical radian evaluation convention is unchanged from the filled-sphere
codec; this is not rational-patch sewing or exact equality of degree/radian
binary parameters. Continuous native edge agreement is bounded at the caller's
native validation tolerance. Source sphere support and strict nesting are proved
before numerical mass/mesh qualification.

The result preserves original solid, shell, face, pole-list, meridian and degree
pcurve IDs and the native edges belonging to each shell. Source-bound version-1
JSON reconstructs the complete original proof and native boundary on reload,
rejecting stale geometry or ancestry. Plain trimmed native reload is separately
tested. Source limits are 32 KiB, 512 records, 31 entities and 2048 values; native
envelopes are limited to 1 MiB. Unsupported cases fail atomically.

`author_iges_spherical_cavity_material.py` independently authors the graph.
Pinned OCCT transfers both the original and Go output as a valid two-face,
one-solid cavity. For outer radius 2 and inner radius 1, volume is 28π/3, area
is 20π and centroid is the origin. Native mass, both world radii, exact pole
vertices, continuous edge bounds, analytic-bound closed mesh and source/native
reload agree independently. Mutations cover touching radii, void sense, reused
outer shell, wrong pole ordinal, seam/frame and endpoint changes, forged native
payload and ancestry, and resource limits. Focused normal/live, race, Go 1.23,
vet, previous sphere variants and bounded fuzz qualify the additive family.
