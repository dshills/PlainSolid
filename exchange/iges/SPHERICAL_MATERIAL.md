# Exact original IGES sphere, native analytic promotion

`ReadSphericalMaterial` / `WriteSphericalMaterial` authenticate a canonical
16-record, origin-centered positive-Z type 196 sphere of positive radius. One
186 material owns one 514 shell and one 510 face. Its 508 loop contains two
pole-vertex degeneracies and two opposite uses of a single 504 meridian edge.
The original type 100 arc runs from (0,-R) to (0,R) in its local plane, with a
proper cardinal type 124 mapping it to the positive-X meridian in world XZ.
Both original pole vertices are exactly (0,0,±R).

The four original type 110 pcurves use IGES spherical degree coordinates:
U=0/360 and V=-90/+90. Their ordered endpoints, direction, owner, multiplicity,
pole uses and shell/face senses are checked against the complete analytic graph.
The increasing semicircle is the positive-X meridian; its two surface branches
coincide physically, the two horizontal chart boundaries collapse to the poles,
and the rectangle covers the sphere once with outward material orientation.
These support and topology checks establish the source material. Positive mass
or a closed display mesh alone is not a material proof.

Every original numeric token must represent its decoded binary64 value exactly;
all references, integer fields, metadata, transforms and dependencies are checked.
There is no snapping or tolerance-based admission. Other graph layouts, arbitrary
placements, cropped spheres, alternate seam arcs and additional geometry are
unsupported. Source limits are 32 KiB, 512 records, 16 entities and 2048 values.

## Native qualification and limits

Promotion uses the existing `parametric.Sphere` and `trimmed.FromParametric`
analytic sphere path. It preserves the analytic world sphere, exact pole vertices
and periodic/degenerate topology, then rebuilds with the caller's native validation
tolerance. Native UV coordinates use binary64 radians and the established
trigonometric evaluation convention. This is **not** exact rational patch sewing
and does not assert equality of the IGES degree coordinates and binary64 native
radian coordinates. Continuous native edge agreement is separately validated
within the numerical tolerance; seam coincidence, poles and world radius are
also checked independently. `NativeEdges` lists south pole, meridian, north pole;
original pole ordinals 1 and 2 correspond to native vertices 1 and 0.

The version-1 source-bound envelope regenerates the complete original graph proof,
analytic native material and ancestry, and compares native serialized data.
Standalone native trimmed reload is also tested. The envelope is limited to 1 MiB;
forged geometry or source/ancestry changes reject before source output.

## Independent interoperability

`author_iges_spherical_material.py` manually authors the exact graph independently
of Go. Pinned OCCT imports it and Go output as a valid one-face, one-solid sphere:
for R=2, volume is 32π/3 and area is 16π. Native mass and closed analytic-bound
mesh provide separate numerical checks.

`author_iges_vendor_sphere.cxx` records the genuine OCCT writer graph. It produces
rounded pole coordinates and meridian circle/frame values (about 1e-16), retained
in `occt-rounded-spherical-material.igs`. The strict reader deliberately rejects
that source. This is evidence of the exactness boundary, not an accepted repair.
The helper builds against the same pinned OCCT libraries as the independent solid
comparator, additionally linking TKPrim; it takes an output filename.

Focused ordinary/live, race, Go 1.23, vet and bounded 100-execution fuzz cover the
new parser mode and previous torus modes. Mutation tests cover support radius,
meridian transform/endpoints, pole ordinal, seam use, shell sense, malformed
native payload and input limits.

## Separate proper signed-axis placement

`ReadPlacedSphericalMaterial` and `WritePlacedSpherical` authenticate a separate
17-record family. The original type 196 support has one proper signed-coordinate
type 124 placement. The meridian's distinct type 124 must be its exact composition
with the local XZ circle frame. Both type 502 poles are explicitly in world space
and must invert exactly to the original ±R poles. All four degree-based type 110
pcurves remain untransformed source UV; attaching a world transform to one is
rejected. No transform is inherited from a face, shell or solid.

The native analytic sphere is placed in that same signed frame. Its world pole
vertices and all six cardinal radius witnesses must be losslessly representable;
large translations that lose equator coordinates reject even when the source
poles survive. The existing numerical angular convention still applies between
those witnesses. Native continuous agreement is bounded at the caller's stated
tolerance, without repeatedly inflating tolerance during persistence rebuild.
The placed native recipe retains the common pose, original world poles and
source metadata and repeats source/geometry validation on reload. Plain trimmed
JSON reload is qualified as well.

The independently authored `--placed` fixture maps the axis to +X and centers the
sphere at (10,-3,5). Pinned OCCT source and Go output are valid one-face solids
with volume 32π/3, area 16π and that centroid. Tests separately check numerical
coincident seams and radius, exact pole positions, source/native persistence,
invalid or disagreeing transforms, forbidden pcurve placement, metadata tamper,
lossy translations and limits. Normal/live, race, Go 1.23, vet and bounded fuzz
cover the additive mode. Original unplaced acceptance remains unchanged.
