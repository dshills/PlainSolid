# Exact corresponding polygon loft

`Planar` constructs a convex native planar shell between equally sized convex
polygon rings on parallel distinct planes. Input cyclic order may be clockwise
or counterclockwise, but both rings must have matching correspondence. Every
ruled side quadrilateral must be exactly coplanar. This supports independently
varying rectangular widths/depths and oblique translated sections; it deliberately
rejects twisted sides instead of replacing them with an arbitrary diagonal.

Binary64 inputs define exact rational positions. The constructor retains those
positions without welding, proves complete oriented support convexity and exact
mesh embedding, and returns rational volume plus original ring vertex/segment,
connecting-edge, cap and side-face identities. All topology remains planar BRep;
this is not curved lofting, arbitrary section matching or a multi-station sweep.

Each ring defaults to at most 256 vertices, with hard ceiling 4096. Support and
embedding checks have their own configured budgets. Exact volume accumulation
uses the support rational-bit limit. A failed proof returns no partial body.
Tests independently integrate a 4×3 to 2×5 loft of height 6 (volume 70), repeat after
an oblique shear and under reversed winding, and reject twisted sides, crossing
correspondence, coincident rings, canceled contexts and exhausted proof budgets.

`PlanarStations` extends the construction to strictly ordered parallel stations.
The complete output must satisfy every oriented side support, so a reentrant
waist is rejected even when its individual bands are valid lofts. Internal rings
remain shared topology; coplanar adjacent patches are deliberately not merged.
Station and band maps retain all original vertex, segment and side identities.
The default limit is 32 stations and 2048 total vertices, with hard ceilings 64
and 16384. Support and embedding passes keep their independent limits.

Independent tests integrate square sections of sides 2,4,2 at heights 0,1,2:
volume 56/3. The globally convex angular crease passes; sides 2,1,2 fail. A
constant-slope 2,3,4 fixture preserves all four coplanar station seams and all
ten faces. Two stations reproduce the original `Planar` topology exactly.

For a nonconvex waist, `PlanarStationBands` is a distinct additive operation.
Every adjacent pair must pass the original convex `Planar` certificate. Strict
parallel station ordering separates slab interiors, and shared rings identify
exactly coincident intermediate caps. Those caps are omitted from the final
boundary while station seam edges remain. The assembled shell additionally
passes exact planarity and embedding checks. Its global support report does not
claim convexity; per-band support reports do. Exact owned band volumes sum to
the returned shell volume. The 2,1,2 square waist at heights 0,1,2 has volume
14/3 and centroid (0,0,1), while the stricter `PlanarStations` still rejects it.

Each band and the final shell have separate configured support/embedding work
limits. The bounded station count therefore bounds the number of proof passes;
there is no unbounded retry or sampled fallback. A locally nonconvex slab,
twisted side, invalid ordering or exhausted budget returns no body.

`PlanarSubdivided` accepts differing forward collinear breakpoints on corresponding
cap sides. Input index zero must be a genuine corresponding corner; automatic
cyclic matching is outside scope. Native planar faces continue to omit collinear
vertices. Every original segment receives an exact normalized interval on its
merged cap edge, and every original vertex receives an exact parameter occurrence
with native vertex ID -1 when removed. Parameters follow original ring traversal;
a separate reversed flag relates that traversal to the native edge direction.
No source occurrence is claimed to be a surviving native vertex.

The original input counts obey the per-ring vertex limit before simplification;
source/parameter rational sizes obey the support bit limit. The bounded linear
simplification is followed by the usual independently bounded loft proofs.
Tests evaluate each original point exactly from its returned merged-edge
parameter, including 1/3, multiple upper breakpoints, and reversed winding.

`MiteredStations` admits differently oriented planar station rings. Each ring
plane must strictly separate every original vertex at every earlier/later
station, and adjacent oriented normals must have positive dot product. Convex
cell proofs plus these complete separating planes exclude overlap; final exact
shell embedding remains mandatory. It is a bounded miter-cell construction,
not an arbitrary spatial path sweep. Shared miter loops keep their original
vertex and edge maps, with no hidden intermediate cap in the final boundary.

All-ring separation has its own shared support work budget. Cell and final-shell
proofs retain independent budgets and the existing global station limits. Tests
integrate a right-angle rectangular elbow: start x=0, y,z in [-1/2,1/2], miter
x+y=2, and end y=3, x in [3/2,5/2]. Its exact volume is 5 and centroid (13/8,7/8,0).
Both windings pass. An end ring crossing the miter plane fails even though its
center is past that plane, demonstrating whole-ring rather than center testing.
