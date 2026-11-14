# Independent placed oblique cavity display comparator

Pinned OCCT source: `4dbfdcc448c0b3e565488afcd626c84483172db9`.
The original shape is a radius8, height12 cylinder starting at Z=-6, minus
an R3/r1 torus centered(1,0,0) with independent axis(4/5,0,3/5).
A second proper whole-body pose is applied to the actual completed OCCT Cut:

    [ 9/25  -4/5  12/25   1/10 ]
    [12/25   3/5  16/25  -2/5  ]
    [-4/5    0     3/5    3/8  ]

This pose has irrational radial extrema in two world axes. The oracle emits
original analytic volume, boundary area and optimal world AABB; no PlainSolid
STEP or triangulation is consumed. The Go test independently compares displayed
volume/area within2%, and each world extreme within the requested0.5 distance.
These numerical checks do not constitute a volume-error certificate. Exact
source/display distance, rounding, intersection and nesting proofs remain Go
certificates independent of this oracle.

Build with the pinned include/library paths and BO/Prim/TopAlgo/BRep/GeomAlgo/
GeomBase/G3d/G2d/Math/Kernel libraries. Run
`PLAINSOLID_OCCT_OBLIQUE_CAVITY_MESH=/path/to/oracle go test ./brep/trimmed -run '^TestPlacedNativeObliqueCavityDisplay$'`.
