# Independent overlapping pocket evidence (no importer)

The existing circular-hole constructors and analytic pocket import path require
disjoint footprints. This milestone supplies independently authored OCCT Boolean
fixtures and analytic measures, not a new native constructor or accepted source
family. No PlainCAD files are modified.

## Side-opening fixture

Stock is [0,10]×[4,10]×[0,6]. Two parallel +Z cutters have radius5/2, centers
(7/2,4) and (13/2,4), and start at z3; their height4 crosses the stock top. Their
union is subtracted and same-domain faces unified. Exact disk intersections are
(5,2) and (5,6); only the latter lies in the stock. The opening along y4 has width8.

Put θ=acos(3/5), full disk-union area A=(25/2)(π−θ)+6, full perimeter
P=10(π−θ), and retained half-footprint area B=A/2. Then:

- V=360−3B = 309.48192308272178.
- Area=312−24+3P/2 = 321.21446153382266.
- Centroid=(5,(2464−12B)/V,(1080−13.5B)/V)
  = (5,7.3087554510455002,2.7551484926127241).

The top area loss and floor gain cancel. The side loses8×3 area; the cylindrical
walls add3P/2. The removed half-footprint first moment about y4 is exactly56/3:
∫[-5/2,3/2]((5/2)²−t²)dt. This gives the y centroid independently of OCCT.

OCCT validates9 faces (7 planes,2 cylinders), one solid,14 vertices and21 edges.
The retained STEP file has584 entities. Entry face#311 has one seven-edge outer
loop (five lines/two arcs); floor#492 has one three-edge loop (two arcs/the opening
line). Cylindrical faces#419/#541 have inward material sense and share the
vertical model edge#424, from(5,6,3) to(5,6,6). The side-plane face#568 has an
8-edge notched boundary. There is no full-circle wall seam or separate inner
entry loop: existing full isolated-cylinder pocket proofs cannot cover it.

## Interior control

Stock [0,10]×[0,8]×[0,6] with the same cuts retains V=480−3A and
Area=376+3P, centroid(5,4,(1440−13.5A)/V). OCCT reports respectively
378.96384616544321,442.42892306764514 and z2.6000826139872153.
It has9 faces,12 vertices,18 edges and10 face bounds (the entry has an inner
union loop), in524 STEP entities.

## Exact source limits

The favorable model vertices are exact dyadic coordinates. Nevertheless shared
line#428 has owner pcurves#434/#440, degree1 splines with constant U values
0.927295218002 and2.214297435588. These differ from the exact circle angles.
Existing rational Taylor/Bernstein enclosures prove cos(U) is not ±3/5, so the
lifted owner curves do not coincide exactly with model x5. Their source V/knot
ranges are extended to[-5.999999999062e−7,4.0000006]; no authored finite edge
selection is inferred from that support interval. Tests keep both current pocket
readers rejecting the fixtures. A future exact model-only importer or explicit
bounded repair would need new continuous owner and partial-wall coverage proofs.

## Exact two-arc native alternative

The upper union boundary can be represented without angular endpoint scalars:
left poles[(1,4),(1,9),(5,6)] with raw weights[1,1/2,5/4], and right
poles[(5,6),(9,9),(9,4)] with raw weights[5/4,1/2,1]. Global poles and raw weights
are dyadic. Tests prove the complete homogeneous identity
(X−cx W)²+(Y−4W)²=(25/4)W² coefficient by coefficient. Positive weights and
strictly negative Bernstein coefficients of XcYc′−YcXc′ establish continuous
clockwise upper-circle traversal between the exact endpoints. This is a concrete
exact profile candidate, not an implemented shell constructor. Do not normalize
weights by5/4 in binary64: that introduces nonbinary ratios.

Build the standalone `tools/occtref/overlapping_side_pockets.cxx` helper with its
build script. `PLAINSOLID_OCCT_OVERLAPPING_SIDE_POCKETS` enables fresh OCCT source
and analytic measure comparisons. It uses OCCT primitives/Boolean operators only,
not Go geometry helpers, and does not modify the shared main oracle.
