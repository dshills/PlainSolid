# Curved-rim parabolic roof IGES solid

`ReadParabolicRoofSolid` and `WriteParabolicRoofSolid` add a distinct type186
closed material family with two genuinely quadratic shared rim edges. The
axis-aligned translated descriptor has positive Width, Depth, Height and Rise:

z = Origin.Z + Height + Rise ((x−Origin.X)/Width)²,

above its rectangular planar bottom. The top and two side bands retain their
original polynomial tensor charts. The side bands are geometrically planar but
nonaffine in UV; they are not flattened or relabeled as affine supports.

The source contains 58 entities: one solid/shell, six faces/loops, shared vertex
and edge lists, six supports, twelve authoritative model curves and twenty-four
independent owner pcurves. Two model curves have degree2 and all other model
curves have degree1; owner pcurves are affine unit-square edges. All charts use
full clamped unit domains and polynomial unit weights (lossless common positive
weight normalization is allowed). The result retains original decoded geometry,
directory labels/subscripts, surface/model/owner IDs, and certified native material.

This is an explicitly **canonical role-order dialect**, not an arbitrary IGES
BRep classifier. Source directory order is the retained fixture's vertex list,
model curves, edge list, then each support/owner curves/loop/face, shell and solid.
The vertex list uses binary X,Y,top indexing. Dimensions and rise are recovered
from those actual model vertices as exact differences; no recipe label supplies
or overrides geometry. Every source chart, model curve, pcurve coefficient and
reference is independently compared to the exact polynomial family formulas.
Unknown roles, reordering, rounded coefficients, shared owner curves, partial
trims, transforms, unsupported metadata and property records are rejected.

All twenty-four original surface/pcurve compositions must also be identically
equal to their authoritative model curves over the full parameter interval.
Native sewing uses the original charts and continuous isocurve proofs. Acceptance
requires a complete `ValidateSplineRectangleEmbedding` certificate: face
injectivity, global separation/contact ownership, connected closed manifold and
outward material orientation. Source incidence or endpoint equality alone cannot
promote a model to material. No tolerance-based approximation is accepted.

All descriptor arithmetic uses exact rationals and rejects unrepresentable
float64 controls before output. The hard source envelope is 64KiB, 1024 records,
58 entities, 4096 values and 104 control points, reduced by caller limits.
Proofs use bounded exact arithmetic and caller cancellation. Readers return zero
results on failure; writers stage complete source/material revalidation before
I/O. Earlier tetrahedron, affine-hexahedron and straight-rim dome APIs are unchanged.

## Independent source and analytic qualification

`author_iges_parabolic_roof.py` independently authors the retained standard graph
for Height=Rise=Width=Depth=1 at the origin. Direct integration gives:

- Volume 4/3.
- Centroid (9/16,1/2,7/10).
- Area 20/3 + √5/2 + asinh(2)/4.

Pinned OCCT `4dbfdcc448c0b3e565488afcd626c84483172db9` transfers the source and
Go output through ordinary IGES TransferRoots/BRepCheck as one six-face solid,
matching the analytic volume/centroid and area within numerical quadrature error.
The measured area is8.1456095236660104 (test allowance1e-8). Native mass also
matches the direct integral. `PLAINSOLID_OCCT_IGES_SOLID` selects the existing
independent material helper.

During fixture authoring the independent mass check caught a decimal token split
across parameter records: pinned OCCT read the first part of0.5 as0 and produced
incorrect mass, while the Go parser concatenated the source token. The retained
new author splits only after commas, as the production Go writer already does.
No geometry tolerance or expected measure was relaxed to conceal that discrepancy.

Tests cover source geometry/ownership/orientation mutations, invalid dimensions,
small budgets, cancellation, atomic writer rejection, and a translated fractional-
coordinate round trip. The public reader has a bounded fuzz target. Role-order
variants and other polynomial roofs remain explicit future work.
