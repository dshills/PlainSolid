# Role-independent parabolic roof topology import

`ReadMappedParabolicRoofSolid` removes source record-order assumptions for the
existing exact polynomial roof family. It accepts independent directory/entity
ordering, vertex-list order, edge-list order, shell face order and cyclic loop
start positions. The original canonical reader keeps its prior strict contract.
No new shape formula, tolerant correspondence or undocumented placement inference
is introduced.

The unique type186 root must own one correctly oriented closed type514 shell.
Each source face owns its original support and loop. Dimensions/translation are
recovered as exact differences from the eight actual source vertices, then all
vertices are bijectively identified with geometric roles. Directed edge endpoints
and exact source curve coefficients determine edge roles. Full support coefficients
determine face roles; the original source shell must own each exactly once.
Within each face, edge-list membership, coedge sense and owner pcurve coefficients
identify each directed use. Only a cyclic change of loop start is permitted;
a noncyclic permutation or reversed directed boundary is rejected.

All 58 source entities must participate in the resulting one-to-one graph mapping.
Original raw scalar and lossless weight gates, unique ownership, source directory
semantics and exact full parameter domains remain mandatory. Geometry comparisons
cover the complete source spline basis/poles/weights, not samples or endpoint-only
matches. The existing exact original model/pcurve composition and complete native
material-embedding proof then applies through that explicit graph/coefficient
identity. Returned geometry remains keyed by the original source entity IDs.
Native geometry has mathematically identical coefficients; nothing is fitted.

`MappedParabolicRoofSolid` embeds the existing material result with source IDs
rewritten to their original values. Additional provenance records canonical roles
to original one-based vertex, edge and coedge list positions, original loop IDs,
and the actual authored shell-face sequence. This retains the distinction between
source topology order and native geometric role order. Directory labels/subscripts
and model coordinates/unit remain original. Source header bytes are not replayed.

The original `WriteParabolicRoofSolid` can emit a fresh canonical representation
of the recovered descriptor and unit. This is explicit normalization, not a claim
to reproduce source IDs or metadata. General chart reparameterization, independent
edge reversal, arbitrary rotations, IGES transform entities and different roof
families remain unsupported. Translation already present in source coordinates
is recovered exactly. Source/proof limits and cancellation are unchanged.

## Independent reordered, translated source

`author_iges_mapped_roof.py` independently authors and then remaps the entire
standard graph: all directory IDs reverse, the eight vertices are permuted, the
twelve edge-list positions reverse, shell face order reverses, and every loop's
start rotates. Model geometry is translated by (1/4,−2,3); UV curves are not
translated. The root moves from ID115 to ID1 and the vertex list from ID1 to ID115.
The retained source is rejected by the canonical reader and accepted by the new
mapped reader with original identity preserved.

Pinned OCCT `4dbfdcc448c0b3e565488afcd626c84483172db9` transfers this source and
its canonical Go re-export through ordinary IGES TransferRoots/BRepCheck:
one solid, six faces, volume4/3, area20/3+√5/2+asinh(2)/4 and centroid
(13/16,−3/2,37/10). Set `PLAINSOLID_OCCT_IGES_SOLID` to the existing material
helper for both directions. These are independent material measurements; native
acceptance still requires exact source identities and a complete certificate.

Negative tests cover reused authoritative curves or owner pcurves, changed
supports/vertices, incorrect shell orientation, noncyclic loop permutations,
resource limits and cancellation. A bounded fuzz target exercises the actual
public mapped reader and checks atomic failure. Previous APIs remain unchanged.
