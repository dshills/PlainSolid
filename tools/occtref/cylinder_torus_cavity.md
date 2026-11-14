# Independent toroidal cavity

Pinned OCCT 4dbfdcc448c0b3e565488afcd626c84483172db9 builds a
radius8 cylinder with caps [-3,4] or [-4,3], then subtracts an original
major5/minor2 torus. BRepCheck validates one solid with two shells.
Independent volume, area, first moment and six X-axis contacts qualify the
native inward torus and exact material query. Tolerances are 1e-6 for
mass/area/first moment, 1e-7 for centroid and contact parameters; exact
source proofs do not depend on those numerical thresholds.

Build cylinder_torus_cavity.cxx using the library flags documented in
torus_cylinder_separated.md. Set PLAINSOLID_OCCT_CYLINDER_TORUS_CAVITY
and run TestOCCTCylinderTorusCavity. No sampled/native exported shape is
used by this comparator.
