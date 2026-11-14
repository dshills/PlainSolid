# Independently oblique cavity boundary distance

Pinned OCCT source: `4dbfdcc448c0b3e565488afcd626c84483172db9`.
The oracle independently cuts a torus R=3/r=1, center(1,0,0), axis
(4/5,0,3/5), X=(3/5,0,-4/5), from a radius8 cylinder with caps[-6,6].
It minimizes point distance over every resulting face (not the solid's interior).
Arguments are world XYZ; output is squared boundary distance. No PlainSolid
facets or exported shape are consumed. Comparisons allow1e-7 absolute squared
error for OCCT's numerical calculations; the Go result remains a rigorous
rational enclosure. Cases cover independently tilted torus feet, irrational
local azimuth, original cylinder wall/cap and a tied cap-wall rim.

Build like the adjacent eccentric distance oracle, with TKBO/TKPrim/TKTopAlgo/
TKBRep/TKGeomAlgo/TKGeomBase/TKG3d/TKG2d/TKMath/TKernel. Run through
`PLAINSOLID_OCCT_OBLIQUE_CAVITY_DISTANCE=/path/to/oracle go test ./geom -run '^TestObliqueCavityBoundaryDistance$'`.
