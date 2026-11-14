# Independent oblique cavity finite line section

Pinned OCCT source: `4dbfdcc448c0b3e565488afcd626c84483172db9`.
This fixture builds the actual original radius8 cylinder/caps[-6,6] minus
R3/r1 torus centered(1,0,0) with X=(3/5,0,-4/5), Z=(4/5,0,3/5).
Optional `oblique` applies the independent whole-body proper pose used by the
adjacent distance oracle. No PlainSolid STEP or display triangles are read.

Arguments: origin XYZ, raw direction XYZ, min t, max t, optional `oblique`.
The oracle explicitly undoes OCCT direction normalization, returning original
raw t. It deduplicates numerical face hits within1e-8. Go comparisons permit
1e-7 absolute parameter error while native root identities remain exact.
Live cases cover rational and irrational torus hits with an independently
nonunit direction. Multiplicity, coincident generator intervals and exact
quarter-chart incidence are checked by independent analytic Go fixtures;
OCCT numerical face intersections are not used as evidence for those claims.

Build with the same pinned include/library paths and BO/Prim/TopAlgo/BRep/
GeomAlgo/GeomBase/G3d/G2d/Math/Kernel libraries as the distance oracle.
Run `PLAINSOLID_OCCT_OBLIQUE_CAVITY_LINE=/path/to/oracle go test ./brep/trimmed -run '^TestNativeObliqueCavityLine$'`.
