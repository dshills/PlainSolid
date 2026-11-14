# Exact eccentric, cardinally rotated elliptical hole

`ReadEccentricEllipticalFace` adds a separate source-supported planar face with
one strictly eccentric ellipse hole. It uses the genuine OCCT two-bound graph
already qualified for concentric annuli, but independently proves the original
hole's translated 3D frame and corresponding 2D placement. The old concentric
reader and native format remain unchanged in acceptance behavior.

The plane and outer ellipse retain their exact common signed-cardinal frame.
The hole has the same normal, an exact coplanar center with nonzero UV offset,
and a reference rotated by 0, 90, 180 or 270 degrees in that plane. The original
source model, pcurve and closing vertex must all agree continuously. No point,
axis, radius or endpoint is snapped, and no angular trim is synthesized. Source
FACE_BOUND reverses the hole while both original model edges retain forward
sense. Native inner local edge 1 is correspondingly reversed.

## Exact sufficient containment and clearance

Let outer semiaxes be a>b>0, hole semiaxes c>d>0, center offset (dx,dy), and
cardinal hole reference (rx,ry). The complete hole fits a UV rectangle with
half extents (ex,ey)=(c,d) for a horizontal reference, or (d,c) for a vertical one.
The implementation computes, using exact rationals,

`B = ((abs(dx)+ex)/a)^2 + ((abs(dy)+ey)/b)^2`.

It accepts only B<1. Thus every hole point is strictly inside the outer ellipse;
the two simple curves are disjoint and their opposite winding defines the face
with one hole. This conservative sufficient test intentionally rejects some
cases a stronger containment solver might prove. No sample-based fallback is
used.

For the normalized map A=diag(1/a,1/b), every accepted hole point p satisfies
`||A p|| ≤ sqrt(B)`, while every outer boundary point q has `||A q||=1`.
Since `||A||=1/b`, `||p-q|| ≥ b*(1-sqrt(B)) ≥ b*(1-B)/2`.
The returned `EccentricEllipseClearance` therefore contains both exact B and
`ClearanceLower=b*(1-B)/2`, a conservative physical distance in model units.
The source cardinal plane is an isometry, so this is also a 3D separation bound.

Projection into UV and construction of native model/UV endpoint coordinates use
exact rational arithmetic and require lossless binary64 representation. The
complete source model/plane/pcurve identity is then checked by the bounded
bijective owner graph proof, including separate parameter contexts and shared
plane ownership. Incorrect lifted direction, center, sense or cross-owned
pcurve fails atomically. Zero-offset holes remain the separate concentric API;
noncardinal rotations, opposite normals and general placements are unsupported.

## Native and original source workflow

`WriteEccentricEllipticalFaceSource` revalidates and re-emits original source
records and metadata. A distinct version 1 native envelope is provided by
`WriteEccentricEllipticalFaceJSON` / `ReadEccentricEllipticalFaceJSON`. It retains
the native plane, both model ellipses, original offset/rotated UV loop, vertices,
owner IDs, source STEP and exact clearance rationals. Reload re-proves the source
and compares all stored native geometry and evidence. Edited or forged clearance
bounds, source IDs, hole frame or winding are rejected. It is face persistence,
not a fictitious closed solid or an unvalidated certificate cache.

The existing input caps remain 64 KiB STEP, 66 entities, 4096 values, depth 16 and
256 KiB native JSON, with smaller caller limits honored. Writer source traversal
is also bounded. Millimeter/radian/steradian units remain explicit; model units
are not silently rescaled. Original product name and source IDs survive reload.

## Independent external qualification

`tools/occtref/step_eccentric_ellipse_face.cxx` independently builds an outer
ellipse (8,5), plane center (1,2,3), normal +Y, reference −Z, and an inner ellipse (2,1)
at model center (0,2,1) with reference −X. In source UV this is center (2,1) and
reference (0,1). The reversed inner wire is added to the planar face and exported
by OCCT. `occt-eccentric-ellipse-face.step` retains that original export, with
only line-end whitespace removed.

The exact enclosure is B=801/1600 and physical clearance is at least 799/640.
Pinned OCCT 4dbfdcc448c0b3e565488afcd626c84483172db9 accepts original and
STEP→native reload→STEP output as a valid one-face/two-edge object. Independent
area is 38π, centroid (20/19,2,59/19), normal +Y, and total adaptive edge length
51.074724292776494. Native original-model point/tangent comparisons and the
reversed UV lift agree; an independent Simpson integral checks total length.

Tests also qualify all four cardinal reference directions, signed offsets,
consistent touching/crossing source mutations, conservative rejection, improper
normal/frame/pcurve ownership, rounded decimals, native proof tampering, resource
limits and strict separation from the concentric API. Focused fuzz ran 100 inputs.
Live test environment: `PLAINSOLID_OCCT_ECCENTRIC_ELLIPSE_FACE`.
