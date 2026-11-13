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
