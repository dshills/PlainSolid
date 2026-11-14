# Second independently constructed oblique cavity

Pinned OCCT source `4dbfdcc448c0b3e565488afcd626c84483172db9`, archive SHA256
`32b1e918a697a0398e8ce623099af2c1d62cdeda2b2eefa86f07c0aee9d4aaf7`.
The oracle constructs an original radius-6 cylinder with caps Z=-4 and Z=5,
subtracting an original torus R=2, r=1/2 centered at (-1/2,3/4,1/4), with
X=(9/25,12/25,-4/5), axis=(12/25,16/25,3/5). No PlainSolid faceting or recipe
is used for the reference geometry. Its full proper frame is
[[9/25,-4/5,12/25],[12/25,3/5,16/25],[-4/5,0,3/5]].

An input STEP file is independently imported and checked against the original
Cut: valid one solid/two shells, original oblique torus support and inward face,
volume and area differences <=1e-6, centroid displacement <=1e-8. The original
analytic solid's line through the torus center along its X axis yields six
stations: -95/16,-5/2,-3/2,3/2,5/2,85/16. Numerical station comparisons use 1e-7.
These tolerances qualify OCCT finite-precision transfer, not exact Go predicates.

The Go regression independently verifies V=324π−π², A=180π+4π², exact stations,
inward seam unit normal -X and four original chart owners, deterministic native
JSON with authenticated source rebinding, and deterministic paired STEP with
all rational frame entries recovered exactly. There is no new modeling API or
widened acceptance gate in this qualification-only milestone.

Build with adjacent STEP fixture's include/library layout and TKDESTEP, TKXSBase,
TKBO, TKPrim, TKTopAlgo, TKBRep, TKGeomAlgo, TKGeomBase, TKG3d, TKG2d, TKMath,
TKernel. Set `PLAINSOLID_OCCT_OBLIQUE_CAVITY_SECOND` to the binary and run
`go test ./exchange/step -run '^TestObliqueCavitySecondSource$'` (also `-race`).
