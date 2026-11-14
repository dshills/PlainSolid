# Proper rational torus/cylinder occurrence

Pinned OCCT source4dbfdcc448c0b3e565488afcd626c84483172db9, original archive
SHA-25632b1e918a697a0398e8ce623099af2c1d62cdeda2b2eefa86f07c0aee9d4aaf7.
The fixture constructs analytic OCCT supports directly in the world frame,
not from a kernel shape, sampled mesh or imported source.

Original torus R5/r2 and cylinder radius6,z[-3,4] are authored under proper
rational pose with rows (3/13,-12/13,4/13;1/7),
(36/65,5/13,48/65;-2/9),(-4/5,0,3/5;3/11).
OCCT receives floating approximations to those rational frame values. Actual
Common/Cut/Fuse shapes must be valid. Each original line(t,0,t/10),t[-8,8]
is independently transformed and intersected; normalized OCCT line parameters
are restored to rawt. Retained interval midpoint classification must be inside.

For each operation the actual retained toroidal face is found by support type.
OCCT D1 at the inner equator(Common) or outer equator(Cut/Union), corrected by
face orientation, supplies an independently evaluated normal and world point.
The Go test compares exact placed classification and normal directions after
normalizing only for numerical comparison. It also compares all finite raw line
roots. Additional kernel tests cover torus/wall rim dual normals, inward
Difference wall orientation, cap-wall normal pairs, buried support rejection,
nonbinary translation, source equivalence, output mutation and atomic budgets.

Intersection tolerance1e-10; world point, unit direction and rawt allowances
1e-7. Numerical oracle deduplication1e-8 applies only to these separated roots.
This does not prove exact owners or a native transformed BRep; the API retains
an immutable original analytic source plus exact pose, and returns unnormalized
rational directions for every exposed original owner.

```sh
c++ -std=c++17 -O2 \
 -I/tmp/plainsolid-occt-build/include/opencascade \
 -L/tmp/plainsolid-occt-build/mac64/clang/lib \
 -Wl,-rpath,/tmp/plainsolid-occt-build/mac64/clang/lib \
 tools/occtref/torus_cylinder_occurrence.cxx \
 -o /tmp/plainsolid-torus-cylinder-occurrence-oracle \
 -lTKBO -lTKPrim -lTKTopAlgo -lTKBRep -lTKGeomAlgo -lTKGeomBase \
 -lTKG3d -lTKG2d -lTKMath -lTKernel
PLAINSOLID_OCCT_TORUS_OCCURRENCE=/tmp/plainsolid-torus-cylinder-occurrence-oracle \
 GOCACHE=/tmp/plainsolid-go-cache go test ./geom ./brep/placement ./internal/occtcheck \
 -run '^(TestTorusCylinderNormals|TestTorusCylinderOccurrence|TestOCCTTorusCylinderOccurrence)$' -count=1
```

Qualified local executable SHA-256:
935983d1944c9ddaaf91c6fa5675f2fe51db984d7b3e03647f2cc97066c6e6fa.
No executable or generated reference data committed.
