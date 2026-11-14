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

Optional fourth argument `oblique` applies a second independently authored
proper whole-body pose X=(3/5,0,-4/5), Z=(4/5,0,3/5), translation
(1/10,-2/5,3/8) to the completed OCCT solid before measuring. This leaves
local torus and stock axes distinct: their world axes are respectively
(24/25,0,-7/25) and (4/5,0,3/5). The native prepared-query test compares
this actual transformed original geometry, including an axial circle family
and quarter-chart vertex ownership after native JSON reload.
