# Independent contained sphere cavity distance

`sphere_cavity_distance.cxx` is a standalone pinned OCCT oracle. It constructs
an original radius-5 sphere at the origin and subtracts an original radius-1
sphere centered at `(1,0,0)` using `BRepAlgoAPI_Cut`. No PlainSolid-produced
solid, mesh, points, surfaces or trims are input to OCCT. `BRepCheck_Analyzer`
must accept the result. The reference has one solid and two closed shells;
independent volume is `496*pi/3` and total boundary area is `104*pi`.

Nearest queries use `BRepExtrema_DistShapeShape` against a compound of the
original boundary faces, avoiding solid/interior-point distance zero. Cases:

- `(3.5,.5,0)`: outer sphere nearest.
- `(3.25,.5,0)`: inner cavity sphere nearest.
- `(3.5,0,0)`: two distinct feet `(5,0,0)` and `(2,0,0)` tie at squared `9/4`.
- `(1,0,0)`: every inner-sphere point minimizes, squared distance `1`.

`TestOCCTSphereCavityNearest` compares the checked native `SpherePairSet`
owner report with independently calculated sphere distances and OCCT values.
It verifies native tie ownership and explicit center family, but does not
interpret OCCT's finite solution count as exhaustive ties or a continuum.
The exact Go comparison uses guarded radical signs; OCCT is a floating-point
reference with search deflection `1e-10` and comparison allowance
`3e-7*max(1,squared distance)`. No Go tolerance or strict containment is weakened.

Pinned source revision: `4dbfdcc448c0b3e565488afcd626c84483172db9`.
Archive SHA256:
`32b1e918a697a0398e8ce623099af2c1d62cdeda2b2eefa86f07c0aee9d4aaf7`.
Example build against its existing local build:

```sh
c++ -std=c++17 -O2 \
  -I/tmp/plainsolid-occt-build/include/opencascade \
  -L/tmp/plainsolid-occt-build/mac64/clang/lib \
  -Wl,-rpath,/tmp/plainsolid-occt-build/mac64/clang/lib \
  tools/occtref/sphere_cavity_distance.cxx \
  -o /tmp/plainsolid-sphere-cavity-oracle \
  -lTKBO -lTKPrim -lTKTopAlgo -lTKBRep -lTKGeomBase -lTKG3d -lTKG2d -lTKMath -lTKernel
PLAINSOLID_OCCT_SPHERE_CAVITY=/tmp/plainsolid-sphere-cavity-oracle \
  go test ./internal/occtcheck -run '^TestOCCTSphereCavityNearest$' -count=1
```

Fresh executable SHA256:
`baf76b1f69148a9ea3d9a60d43ac58969e3229df9cec05bca4b244cc7641c2b0`.
Observed squared values: `2.1446609406726242`, `1.7027277713535562`,
`2.25`, and `1`. OCCT returned two representatives at both the isolated tie
and the center-family query, illustrating why that count is not a locus proof.
Generated executable and JSONL output are local verification artifacts only.
