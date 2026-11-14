# Mixed planar and original spline material assembly

`WritePlanarSplineAssembly` and `ReadPlanarSplineAssembly` are an explicit,
additive AP214 rigid-product workflow. A file must contain both planar advanced
B-rep products and products from the certified rectangular spline-shell family.
Assembly-only nodes have no bodies. Each geometry-bearing product has one
family, potentially several local bodies. Occurrences refer to product
identities; repeated geometry is neither flattened nor duplicated.

The writer uses typed `PlanarSplineFamily` payloads. The reader determines family
from the complete source shell's supports: all faces must use PLANE or all must
use supported original rectangular B-spline surfaces. Mixed supports inside a
body or product, analytic spheres/cylinders, faceted bodies, unknown families,
and decoder fallback guessing are outside this API. This does not expand the
homogeneous planar, spline or generic STEP import contracts.

Planar bodies pass the existing advanced planar decoder plus the exact planar
embedding/contact/material-orientation validator. Spline bodies pass the existing
full source model/pcurve agreement, topology, injectivity, global embedding and
outward-material proof. Original spline bases and local coordinates are retained.
Source body IDs and product-definition IDs remain available with the original
parsed graph. Geometry-bearing topology, model/pcurves and supports cannot be
shared across distinct product definitions; explicit occurrences are the reuse
mechanism. Unowned material entities are rejected. Common representation/unit
contexts are allowed. Parser, product/occurrence and cumulative body/vertex/face/
use budgets remain enforced. The planar and spline codecs retain their existing
source numeric conventions; this wrapper introduces no rounding-based repair.

Placements are restricted to proper signed-coordinate rigid frames with finite
translation. Names, product and occurrence identities, explicit length unit and
source-local geometry remain separate. No claim is made that occurrence solids
are mutually disjoint or represent a Boolean union: the assembly can contain
interfering components. Arbitrary floating rotations, scale, mirrored occurrences,
colors, styling and generic material metadata are not a new supported contract.
The source graph remains inspectable. Writers stage the entire graph and typed
readback before emitting output; failures return no partial decoded model.

## Independent reference

`tools/occtref/step_planar_spline_assembly.cxx` independently builds two products:
a 1×2×3 planar box and a six-chart bilinear solid
`0≤x,y≤2, 0≤z≤1+xy/4`. The curved product has two occurrences: translated by
(10,0,0), and rotated exactly 90° around Z then translated by (0,20,0).
The box is translated by (-10,0,0). XCAF writes the retained independent
`testdata/planar-spline-assembly.step` fixture.

Pinned OCCT `4dbfdcc448c0b3e565488afcd626c84483172db9` accepts the independently
written input, native Go output and input→Go typed decode→Go re-export. Ordinary
TransferRoots/BRepCheck and a separate XCAF import establish 3 solid occurrences,
18 faces, 2 shared product definitions and 3 occurrence labels. Independent
integrals give total volume 16 and centroid (-7/16,175/24,139/144). The reference
area is 58.63229613162359, compared numerically within 1e-8. Those numerical
allowances qualify the external reference and never relax graph acceptance.

Build with `build_step_planar_spline_assembly.sh`; optional Go tests use
`PLAINSOLID_OCCT_MIXED_ASSEMBLY`. Pure-Go tests retain the source fixture,
source-chart identity, names/placement checks, cross-product body ownership,
orphan faces, cumulative limits, family mismatches, atomic writer failures and
bounded fuzz seeds. OCCT is an optional development oracle only.
