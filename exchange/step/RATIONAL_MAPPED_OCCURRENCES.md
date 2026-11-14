# Exact rational mapped occurrence poses

`ReadRationalMappedPlanarMaterials` retains local planar bodies separately from
`ExactMappedOccurrence` poses. It accepts the previously verified cardinal or
Pythagorean local material domain and exactly rationally normalizable proper
orthogonal occurrence axes. No rounded `geom.Affine` is stored or implied.

The pose owns private rational coefficients. `Matrix` returns deep copies;
`ApplyExact` returns independently owned rational world coordinates.
`ApplyRepresented` succeeds only if every world coordinate is exactly binary64
representable. For example a3:4:5 rotation of local(2,0,0) with translation(10,0,0)
yields exactly(56/5,8/5,0), and represented-point conversion rejects. The same pose
maps(5,0,0) to exactly(13,4,0), which represented conversion accepts. Local native
BRep geometry remains original; no floating transformed native material or
material classification beyond the local proof is constructed.

The original eight-field operator is checked for exact scale1, explicit origin,
perpendicular raw X/Z ratios with rational lengths, and independently normalized
Y equal to Z×X. Reflection, ignored inconsistent Y, irrational normalization and
unsupported operator fields reject. Original graph reachability is verified
before creating a private proof view with identity occurrences. The established
local material/context/topology/ownership proof validates that view. Original
operator data and source IDs remain in Source; no source geometry is repaired.
The original cardinal and local-Pythagorean reader contracts remain unchanged.

`WriteRationalMappedPlanarPlacements` copies caller rational poses and authors
new private operator records for every original occurrence. All items must be
identified exactly once. Local products, names, units and membership remain.
Origin decimals must terminate exactly; a1/3 translation rejects. Direction
ratios are scaled to exact integer records, avoiding serialization of nonbinary
unit coordinates. Input bit counts, decimal lengths, exponent limits, cumulative
parser bounds and sixteen-occurrence maximum bound work. Full readback precedes
publication. `WriteRationalMappedPlanarSource` is a distinct validated source
write-back API.

## Independent observation

`author_rational_mapped_occurrence.py` changes only the first mapped operator in
the independently OCCT-authored two-product source, adding raw(3,4,0) and(−4,3,0)
direction records. It does not invoke the Go writer or change local model/owner
geometry. Pinned OCCT `4dbfdcc448c0b3e565488afcd626c84483172db9` ordinary
TransferRoots/BRepCheck and AddOptimal accept both that source and Go-authored
poses, with world bounds minimum(−10,0,0),maximum(56/5,22,4).
`TestOCCTRationalMappedBounds` uses `PLAINSOLID_OCCT_MAPPED_BOUNDS`.

Tests cover copied rational ownership, exact versus represented application,
source write-back, missing/duplicate items, nonterminating origins, malformed
frames, inconsistent source Y/scale, foreign maps, detached geometry and limits.
This is an exact separate pose API, not a promise that all rational world points
can be stored in the native floating document schema or OCCT doubles losslessly.
