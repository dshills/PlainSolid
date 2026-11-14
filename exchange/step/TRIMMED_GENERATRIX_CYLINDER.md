# Explicit parameter-trimmed axial spline edges

`ReadTrimmedGeneratrixCylinderProduct` imports a separate rooted split-cylinder
solid dialect with exactly two `TRIMMED_CURVE` axial edges. Each wraps a full,
clamped degree-one polynomial or positive rational spline. The existing
`ReadSplineGeneratrixCylinderProduct` remains strict and rejects these wrappers.

Only one `PARAMETER_VALUE` per trim selector is supported. The two values must
exactly equal the original finite basis domain endpoints, in increasing order;
`master_representation` must be PARAMETER. Rounded decimals, point selectors,
redundant point alternatives, unspecified master selection, nested/shared basis
curves and partial ranges fail. A false `sense_agreement` reverses the full basis
traversal. Effective basis direction is forward precisely when that sense equals
the independent `EDGE_CURVE.same_sense` flag. Both flags remain available in the
returned trim provenance; `Cylinder.SameSense` is the proved effective direction.

The output retains source wrapper and basis entity IDs, wrapper name, parameter
selectors and both orientation flags, keyed by original EDGE_CURVE ID. Original
basis name, weights, knots and poles remain in `Cylinder.Curves`. Source faces,
coedges, edges and product identity are checked by the unchanged bounded split-
cylinder solid proof after exact, temporary representation normalization.
The full-domain proof is essential: no arbitrary subinterval is inferred from
coincident vertices or tolerance. Every wrapper and basis has unique source
ownership. Whole-graph validation rejects all remaining unowned entities.
Any failure returns a zero result, not a partially reconstructed solid.

Input has at most 240 entities. Existing byte/value/depth, exact scalar/weight,
control, native topology and caller limits remain in force. There is no material
acceptance from incidence alone: the validated finite axial segments, complete
complementary analytic half-cylinder cells and planar caps must describe the
same exact native cylinder. This is not a general trimmed-curve or pcurve reader.
In particular, decimal approximations of π do not establish exact antipodal
cylindrical parameter boundaries and are not admitted by this extension.

## Independent interoperability

The retained `trimmed-generatrix-cylinder.step` fixture independently adds
standard parameter trim wrappers to the rational source model. One uses a
reversed trim and reversed edge; the other uses a forward trim and reversed edge.
Both preserve the finite basis domain [3,7] and unequal positive weights [1/2,1].
Pinned OCCT revision `4dbfdcc448c0b3e565488afcd626c84483172db9` accepts ordinary
`TransferRoots` and BRepCheck validity: four faces, volume 37.699113210939316,
area 62.831855123588646 and centroid (0,0,1.5) within numerical error. Analytic
values are 12π and 20π. The 3e-6 reference allowance covers independent OCCT
rational-edge quadrature only; the Go importer uses exact source equality.

Run the optional test with `PLAINSOLID_OCCT_SPLIT_CYLINDER` pointing to the
tracked `read_split_cylinder.cxx` helper. Mutation tests cover changed endpoint
parameters, rounded values, independently flipped senses, selector ambiguity,
shared/nested basis ownership, detached geometry and resource limits. A bounded
fuzz target checks atomic failure through the actual public typed reader.
