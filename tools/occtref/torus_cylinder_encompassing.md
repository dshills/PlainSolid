# Strict encompassing torus–cylinder reference

The standalone C++ fixture builds original OCCT torus R=5,r=2 and finite
cylinder c=8, with caps [-3,4] and [-4,3], then performs actual Common,
Cut and Fuse. It checks validity and one/zero/one solid and shell counts,
original mass properties and independently intersected X-axis parameters.
No PlainSolid display or exported shape supplies the reference.

Pinned OCCT source: 4dbfdcc448c0b3e565488afcd626c84483172db9.
Build using the same include/library flags as torus_cylinder_separated.md,
substituting torus_cylinder_encompassing.cxx and binary name.
Run TestOCCTEncompassingTorusCylinder with
PLAINSOLID_OCCT_TORUS_CYLINDER_ENCOMPASSING pointing to that executable.
The comparator allows 1e-6 absolute mass/area/first-moment discrepancy and
1e-7 centroid/line-parameter discrepancy; these are finite-precision OCCT
qualification thresholds, separate from exact rational native proof.
