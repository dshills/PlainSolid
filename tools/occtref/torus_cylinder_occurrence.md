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
 -o /tmp/plainsolid-torus-cylinder-world-moments-oracle \
 -lTKBO -lTKPrim -lTKTopAlgo -lTKBRep -lTKGeomAlgo -lTKGeomBase \
 -lTKG3d -lTKG2d -lTKMath -lTKernel
PLAINSOLID_OCCT_TORUS_OCCURRENCE=/tmp/plainsolid-torus-cylinder-world-moments-oracle \
 GOCACHE=/tmp/plainsolid-go-cache go test ./geom ./brep/placement ./internal/occtcheck \
 -run '^(TestTorusCylinderNormals|TestTorusCylinderOccurrence|TestTorusCylinderOccurrenceDistance|TestTorusCylinderOccurrenceMoments|TestOCCTTorusCylinderOccurrence)$' -count=1
```

Qualified local executable SHA-256:
19ebe0e45cd20e359488252f59bfccf958c90d0ef092835b6c246dbc87dc778a.
No executable or generated reference data committed.


## Placed complete boundary distance

The oracle also measures distance from world(4,1,3/2) to an independently
assembled compound of all retained Boolean faces, for every operation. The
Go comparator requests squared-distance width1e-4 and checks OCCT within that
interval plus 3e-7*max(1,d²) numerical allowance. It does not substitute OCCT's
filled-solid zero distance for boundary distance.

The placed API executes one original distance proof. Proper rational rigidity
preserves physical squared units, all original owners and their conservative
possible/unique status. World upper witnesses retain their radicals exactly:
T+axis*(heightBase+sign*sqrt(heightRadicand)) plus the rotated original query
radial vector times sqrt(radialSquared/queryRadialSquared). Axis queries expose
a complete attained circle using two exact orthonormal world vectors instead
of division by zero. No upper witness is promoted to an exact closest point.
Unit tests check unchanged complete source reports, perpendicular/unit world
basis, attained upper distances, circle families and mutable-output isolation.


## Exact posed mass tensors

BRepGProp independently integrates every actual oblique Boolean's volume,
world center and full central inertia matrix. The comparator reconstructs
world first/raw second moments and origin inertia from those independent
values, checking all components with a1e-5 numerical allowance. Source
integration widths remain volume0.01, rawsecond0.1, centroid0.001, inertia0.2.
These are LOCAL tolerances; no identical transformed-width promise is made.

The kernel reuses one original certificate and exact proper rational pose:
Fworld=R*F+V*t;
Qworld=R*Q*R^T+(R*F)*t^T+t*(R*F)^T+V*t*t^T;
IcenterWorld=R*Icenter*R^T.
Centroid is t+R*Clocal, origin inertia is trace(Qworld)*identity-Qworld.
Sign-aware exact interval arithmetic preserves all off-diagonal bounds, with
separate reported transform operation and coefficient limits. Failures discard
the entire report. Tests include exact identity and signed-axis equivalence,
independent c=R antiderivative tensors under the rational oblique pose, one
mapping operation less than required, cancellation and output ownership.
