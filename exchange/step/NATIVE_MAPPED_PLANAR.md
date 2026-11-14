# Native cardinal planar materials as mapped STEP

`WriteNativeMappedPlanarMaterials` accepts a `NativeMappedPlanarModel` with two to
eight local native `brep.Solid` products and at most sixteen explicit uses.
A use selects a product index; repeated uses share one `REPRESENTATION_MAP`.
Each product must be used. The output has one rooted geometric representation,
not an invented NAUO assembly or one original PRODUCT definition per map.
Product and occurrence names, model length unit and proper signed-cardinal
occurrence poses are explicit input. Root product metadata uses the first local
product's name, while the root representation uses the collection name.

This authoring path generates geometry from native solids rather than requiring
an original mapped source. It stages planar ADVANCED_FACE/EDGE_CURVE topology,
adds a per-owner LINE PCURVE to each of the two adjacent planes of every model
edge, and wraps that model LINE in SURFACE_CURVE. Each native local body has
private geometry records. Identity mapping origins and eight-field Cartesian
transformation operators match the independently qualified mapped dialect.

The bounded domain requires cardinal straight edges and cardinal supporting
plane frames, one outward connected material shell per local product, and exact
represented coordinates. Planar hole loops are retained. All floating native
coordinates are emitted as their exact finite binary-rational decimals. Plane
projection is computed with rational arithmetic; any UV coordinate needing
rounding fails. Reflections, scales, unsupported noncardinal frames, orphan
products, bad names, units and resource limits fail before output.

The complete staged file is accepted only after the existing exact mapped
reader proves model/pcurve correspondence, owner exclusivity, closed manifold
and global material embedding. Decoded face loops are matched back to native
oriented loops using exact points, allowing only cyclic start-index changes;
vertex allocation order is not mistaken for topology. Existing source-only and
edited-source writers/readers keep their original contracts.

The independent pinned OCCT property reader (`4dbfdcc448c0b3e565488afcd626c84483172db9`)
loads native output through ordinary TransferRoots and BRepCheck. The two local
boxes (2×3×4 and1×2×3) have three uses: translate first by(10,0,0), cycle its
coordinates with translation(0,20,0), and translate second by(−10,0,0).
Observed:3solids,18faces,volume54,area126,centroid(85/18,91/9,31/18).
This is independently checked output interoperability; geometry generation is
native Go, not claimed as an independently OCCT-authored input fixture. Existing
mapped reader fixtures separately supply independent OCCT-authored local inputs.

Run `TestOCCTNativeMappedPlanar` with `PLAINSOLID_OCCT_DISK_BOUND_MASS` pointing at
the pinned standalone property reader. Ordinary regressions cover deterministic
bytes, repeated shared products, holed planar material, corrupted owner pcurves,
unsupported inputs and atomic failures. No general arbitrary-planar, curved,
scaled or nested mapped assembly claim is made.
