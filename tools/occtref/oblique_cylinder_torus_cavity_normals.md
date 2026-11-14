# Original oblique cavity oriented normals

This standalone OCCT fixture builds a radius-8 cylinder on Z[-6,6] and subtracts
an original analytic torus of major/minor radii 3/1, center (1,0,0), X direction
(3/5,0,-4/5), and axis (4/5,0,3/5). It does not read PlainSolid topology or mesh.
The optional `oblique` argument applies the proper whole rotation
[[3/5,0,4/5],[0,1,0],[-4/5,0,3/5]] and translation (1/10,-2/5,3/8).

Arguments are point X Y Z, optionally `oblique`. Each output line contains the
original owner bit (torus 1, cylinder 2, lower cap 4, upper cap 8), followed by its
outward material unit normal. OCCT face-distance <=1e-7 selects incident faces;
analytic surface parameters at the attained point and D1 derivatives determine
the normal. Face orientation reverses the cavity normal. Analytic parameter
recovery also handles feet on a face's topological edges, where DistShapeShape's
ParOnFace method is not defined. The Go differential compares components within
1e-8 and requires the complete owner mask. These are finite precision reference
tolerances, not the exact Go acceptance predicates.

Pinned OCCT source: `4dbfdcc448c0b3e565488afcd626c84483172db9`.
Archive SHA256: `32b1e918a697a0398e8ce623099af2c1d62cdeda2b2eefa86f07c0aee9d4aaf7`.
Build with the same include/library layout as the adjacent cavity distance
fixture, linking TKBO, TKPrim, TKTopAlgo, TKBRep, TKGeomAlgo, TKGeomBase, TKG3d,
TKG2d, TKMath, and TKernel. Set `PLAINSOLID_OCCT_OBLIQUE_CAVITY_NORMALS` to the
binary and run `go test ./brep/trimmed -run '^TestNativeObliqueCavityNormals'`.
The test includes a rational non-axis torus foot, a four-chart torus vertex,
wall, both caps, cap/wall crease, and a proper whole occurrence. Independent
exact tests check unit norm, source tangent orthogonality, both material sides
at 2^-100 displacement, source ownership, output isolation, and atomic budgets.
