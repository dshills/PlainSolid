# Retained torus/cylinder boundary distance

Pinned OCCT4dbfdcc448c0b3e565488afcd626c84483172db9, original archive SHA-256
32b1e918a697a0398e8ce623099af2c1d62cdeda2b2eefa86f07c0aee9d4aaf7.

The oracle independently constructs torus R5/r2 and cylinder radius6,z[-3,4],
then actual Common/Cut/Fuse. Faces are separated by analytic support type into
original torus/wall/lowercap/uppercap compounds. BRepExtrema computes distance
to each retained group and to their boundary compound, never to the filled
solid (which would report zero for interior queries). Three original points
(4,1,3/2),(0,0,0),(6,0,2) exercise irrational azimuth, axis families, and
trim-end projection. All nine global results and each exposed support result
are checked against the kernel enclosure requested at squared width1e-4.
OCCT extrema tolerance1e-10; numeric comparison allowance3e-7*max(1,d²).

The kernel proof is independent of OCCT. Rotational symmetry reduces distance
to radial/height meridian coordinates. Every retained torus branch is covered
by rational radial cells; monotone square-root bounds (including the height
maximum at rho=R) enclose complete positive/negative branches. Distance to
these boxes is a lower bound. Exact symbolic analytic endpoint/midpoint samples
are attained upper witnesses. Refinement splits the least lower-bound cell.
Cylinder-wall projection is clamped to every retained height segment using
exact squared-height comparisons; disk projection is clamped radially.
The reported upper witnesses can be irrational and are not claimed stationary.
For an axis query a witness denotes its whole rotational circle, which may
collapse to a point. This is not a complete exact minimizer-locus report.

All exposed original support bounds contribute to the global interval. A
unique owner is reported only when every other support lower bound exceeds
the global upper bound. Overlapping intervals retain possible owners; they
never establish a tie. Tests cover an exact shared rim with no false unique
claim, buried regions, independent exact simple distances, witness distances,
monotone refinement and atomic cell/operation/cancellation failures.

```sh
c++ -std=c++17 -O2 \
 -I/tmp/plainsolid-occt-build/include/opencascade \
 -L/tmp/plainsolid-occt-build/mac64/clang/lib \
 -Wl,-rpath,/tmp/plainsolid-occt-build/mac64/clang/lib \
 tools/occtref/torus_cylinder_distance.cxx \
 -o /tmp/plainsolid-torus-cylinder-distance-oracle \
 -lTKBO -lTKPrim -lTKTopAlgo -lTKBRep -lTKGeomAlgo -lTKGeomBase \
 -lTKG3d -lTKG2d -lTKMath -lTKernel
PLAINSOLID_OCCT_TORUS_DISTANCE=/tmp/plainsolid-torus-cylinder-distance-oracle \
 GOCACHE=/tmp/plainsolid-go-cache go test ./geom ./internal/occtcheck \
 -run '^(TestTorusCylinderDistance|TestOCCTTorusCylinderDistance)$' -count=1
```

Qualified binary SHA-256:
f4a87a9737e328aee0b61fd04ce2d3949fcddbefe37ca9eef864da2cc8efd4e7.
No executable/generated reference data committed. Existing strict source
transversality and clear-cap requirements are unchanged; no native assembly
or exact algebraic global minimizer claim is made.
