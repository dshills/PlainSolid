# Eccentric toroidal cavity reference

Pinned OCCT source 4dbfdcc448c0b3e565488afcd626c84483172db9.
Build with standard non-STEP reference flags. Set
PLAINSOLID_OCCT_ECCENTRIC_CAVITY and run TestEccentricCylinderTorusCavity.
Original OCCT torus major5/minor2 centered(1,1,0) is subtracted from radius10
cylinder at XY0 with caps[-3,4]. Require valid one solid/two shells and six
Y=1,Z=0 line parameters [-sqrt99,-6,-2,4,8,sqrt99] (1e-7 comparison).
Go compares original OCCT volume, area and all centroid components with native
integration at1e-6 absolute tolerance, separately checking analytic shifted
first moments. Exact source query/containment proofs do not use these numeric
comparison tolerances. Native reload and global display contacts are checked.
