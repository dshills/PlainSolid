# Split-cylinder topology investigation

`testdata/split-cylinder-occt.step` is independently authored by OCCT 8 using
BRepPrimAPI_MakeCylinder(radius 2, height 3) followed by
ShapeUpgrade_ShapeDivideClosed with one split point. The dedicated helper
`tools/occtref/split_cylinder.cxx` verifies BRepCheck validity and reports four
faces, volume 12π and area 20π. It does not use PlainSolid geometry helpers.

This is retained rejection evidence, not a new accepted import family.
The ordinary cylinder reader requires one lateral face; four-face dispatch
requires two different tube radii. More importantly, exact source coincidence
fails: vertex #23 is (2,-4.898587196589E-16,3) and #25 is
(-2,2.449293598295E-16,3). Their exact squared radial residual is y² > 0.
The two generatrix models inherit these offsets, including the degree-one
B-spline #116. Rounded U lines at 3.14159265359 and 6.28318530718 are not exact
π and 2π. Model vertices cannot be repaired by merely ignoring these pcurves.

A separately authored canonical graph could use exact vertices (±2,0,z),
exact circle model curves with ordered antipodal endpoint semantics, and plain
EDGE_CURVE geometry without pcurves. Its half-circle selection can in principle
be proved from source orientation and complementary coedges without decimal π.
That is a distinct input domain and needs a complete new oriented arc/coverage
validator; the current analytic cylinder proof requires owned pcurves and a
self-used seam. Removing those checks would not establish such a validator.
No accepted canonical or original OCCT source equivalence is claimed here.

A future tolerance-aware adapter must explicitly bound vertex and continuous
edge displacement, establish complementary branch selection and exact repaired
cap/side joins, and return repaired geometry with original discrepancies. It
must not identify this tolerance-valid source with exact geometry silently.
