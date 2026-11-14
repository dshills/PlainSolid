# Finite cylinders in heterogeneous STEP material assemblies

`ReadCylinderMaterialAssembly` and `WriteCylinderMaterialAssembly` add one
bounded analytic family to heterogeneous assembly exchange: a complete finite
cylinder with one cylindrical side and two planar caps. A file must contain at
least one cylinder product and at least one planar or certified rectangular
spline-shell product. Product families remain homogeneous and explicitly typed;
assembly-only nodes contain no geometry. Repeated occurrences retain shared
product identity, names, units, source body IDs and source-local geometry.

Read classification examines every source face support. Exactly one
CYLINDRICAL_SURFACE and two PLANE supports select the established full cylinder
validator directly. Its original rim, seam, cap, orientation and pcurve checks
must all pass. Cones, spheres, sectors, tubes, split-side variants and partial
cylinders cannot reach a generic analytic fallback. Returned `Cylinders`
descriptors are recovered from the validated native result; `Analytic` retains
that material and `Assembly.Source` retains original entities and their IDs.
Existing source numeric and angular conventions of the cylinder codec remain
unchanged. This wrapper does not introduce a tolerance-based repair or expand
any simple-body importer.

The private assembly staging/ownership/budget machinery is shared with the
planar/spline workflow. Both existing public types and that workflow's acceptance
domain are unchanged. The new API uses separate public types and family tags.
All products retain the planar embedding or original spline material proofs;
poses are proper signed-coordinate rotations plus finite translations.
Cumulative body, vertex, face and use limits apply to unique local definitions,
not multiplied occurrences. Cross-product source geometry sharing and unowned
material entities fail. This is an assembly, not a fused union or a proof of
noninterference between occurrences. Output is staged and validated before I/O.

## Independent reference

`step_cylinder_material_assembly.cxx` independently constructs an XCAF assembly
from a planar 1×2×3 box, the bilinear graph material `z≤1+xy/4` over [0,2]²,
and a radius-1 height-2 cylinder. The curved spline and cylinder definitions each
have two occurrences. Cylinder placements are a translation by (0,-10,0), and
the exact cyclic axis map `(x,y,z)->(z+20,x,y)`.

Pinned OCCT `4dbfdcc448c0b3e565488afcd626c84483172db9` ordinary TransferRoots,
BRepCheck and a separate XCAF import verify independent source, native output
and source→typed decode→re-export: 3 product definitions, 5 occurrences,
5 solids and 24 faces. Independent integrals give volume `V=16+4π` and centroid
`((-7+42π)/V,(350/3-20π)/V,(139/9+2π)/V)`. The numerical area reference is
`58.63229613162359+12π`; comparisons use 1e-8 only for external numerical measures.
The retained source is `testdata/cylinder-material-assembly.step` (853 entities).

Build the isolated helper with `build_step_cylinder_material_assembly.sh` and
set `PLAINSOLID_OCCT_CYLINDER_ASSEMBLY` for optional live tests. Focused tests
cover actual original geometry decode, re-export, old API rejection, cumulative
limits, unsupported support classes, wrong source radius versus rims, invalid
product payloads and poses, staged failure, and bounded decoder fuzzing.
No runtime OCCT or other external dependency is introduced.
