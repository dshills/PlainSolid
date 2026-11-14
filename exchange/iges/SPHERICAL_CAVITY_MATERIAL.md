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

## Common proper signed-axis placement

`ReadPlacedSphericalCavityMaterial` / `WritePlacedSphericalCavity` add a separate
32-record family. Both original type 196 supports must refer to the same proper
signed-coordinate type 124. Each original meridian retains its own distinct
transform record; both must equal the exact composition of that shared placement
with the corresponding local XZ meridian frame. Both pole lists are world-space
records and must invert exactly to their own ±radius poles. All eight degree
pcurves remain source UV with no world transform.

The common rigid motion preserves strict concentric nesting and orientation.
The native analytic boundary is transformed, preserving outer/inner face sense,
then validated at the original caller tolerance. All twelve cardinal radius
witnesses must map losslessly, and each native pole must reproduce its original
world source coordinate. The numerical angular-evaluation qualification remains
unchanged. Source-bound and plain native reload are tested; source writing
revalidates the common pose, both independently owned meridian transforms and
all source ancestry before emitting any bytes.

The independent author's `--placed` fixture maps the axis to +X at center
(10,-3,5). Pinned OCCT imports original and generated output as valid two-face,
one-solid material with volume 28π/3, area 20π and that center. Native world
radii, exact poles, continuous bounded agreement, mass and persistence agree.
Specific negatives shift only the inner meridian, move only its supporting
center, or alter only an inner pole. Improper/general/lossy poses, transformed
UV curves and forged native placement also reject. The source cap is 32 entities;
all other limits remain unchanged. Focused normal/live, race, Go 1.23, vet,
previous sphere variants and bounded fuzz qualify this separate placement path.
Record reordering is not included.

## Independently reordered cavity shells

`ReadReindexedSphericalCavityMaterial` separately admits directory reindexing,
each shell's own pole-list permutation and each four-use loop's cyclic start.
The material root still determines which shell is exterior and which is the
negative void. Each original face selects its own support, loop, vertex list
and meridian list; those ownership sets must be distinct. Exact pole geometry
under the support's placement determines each local pole role. A bounded root
bijection then authenticates every datum, pcurve and transform and rejects
cross-shell sharing or detached records. The canonical cavity proof is repeated
after normalization, including strict concentric nesting and void direction.

Original vertex-ordinal maps are keyed by their source pole-list IDs; use maps
are keyed by source loop IDs. Both source faces, both meridians and all eight
degree-pcurve IDs remain available through source role/ancestry fields. Native
reload regenerates those identities from the retained original bytes; altered
maps fail source export atomically. An inverted void direction, reused shell,
shared support or shared pcurve is explicitly rejected.

An independent Python transformer reverses the entire directory sequence,
reverses both pole lists and rotates both loops while updating references and
ordinals. Pinned OCCT accepts both placed and unplaced originals as valid
cavities with the same independent mass/center measurements. Normal/live, race,
Go 1.23, vet, previous sphere/cavity regression and bounded fuzz qualify the
separate path. Graph traversal remains capped at 512 steps and 32 entities.
