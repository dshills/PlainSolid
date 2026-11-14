# Independent overlapping spherical Boolean nearest reference

Pinned OCCT revision: `4dbfdcc448c0b3e565488afcd626c84483172db9`.
Original source archive `/tmp/plainsolid-occt-4dbfdcc.tar.gz`, SHA256
`32b1e918a697a0398e8ce623099af2c1d62cdeda2b2eefa86f07c0aee9d4aaf7`.

The standalone reference independently constructs original radius-5 spheres at
(-3,0,0) and (3,0,0), then OCCT Fuse/Common/Cut. Each must be valid and have
one solid. It receives no PlainSolid topology or approximations. Boundary-face
compounds avoid zero solid distance for interior queries. The intersection
circle is independently x=0, y²+z²=16.

| Operation | Query | Exact squared distance | Nearest foot |
|---|---|---|---|
| Union | (0,9/4,0) | 49/16 | (0,4,0) |
| Common | (0,36/5,0) | 256/25 | (0,4,0) |
| Difference | (3,9/2,0) | 37/4 | (0,4,0) |
| Union | (0,36/5,0) | 196/25 | two retained spherical feet |
| Common | (0,9/4,0) | 25/16 | two retained spherical feet |
| Difference | (0,9/4,0) | 25/16 | retained cutter spherical foot |

The first three reject both full-sphere radial minima because those points are
buried by the Boolean. The common circle foot is isolated and belongs to both
original faces. OCCT extrema search uses 1e-10; comparisons allow
3e-7·max(1,squared distance), and compare the isolated foot within 1e-6.
Solution counts establish existence only, not exhaustive multiplicity.

```sh
c++ -std=c++17 -O2 -I/tmp/plainsolid-occt-build/include/opencascade \
-L/tmp/plainsolid-occt-build/mac64/clang/lib \
-Wl,-rpath,/tmp/plainsolid-occt-build/mac64/clang/lib \
tools/occtref/sphere_overlap_distance.cxx -o /tmp/plainsolid-sphere-overlap-oracle \
-lTKBO -lTKPrim -lTKTopAlgo -lTKBRep -lTKGeomBase -lTKG3d -lTKG2d -lTKMath -lTKernel
PLAINSOLID_OCCT_SPHERE_OVERLAP=/tmp/plainsolid-sphere-overlap-oracle \
go test ./internal/occtcheck -run '^TestOCCTSphereOverlapNearest$' -count=1
```

The exact query is deliberately limited to coordinate-axis center displacement,
rational positive radial norms and rational intersection radius. It rejects
unresolved radicals and axis/center families. Native sewing tolerances remain
independent from the ideal original-sphere query proof.

## Irrational query norms

Optional `--algebraic` keeps the independently authored original spheres and
Boolean bodies unchanged. It queries these exact represented integer points:

| Operation | Query | Squared minimum | Feature |
|---|---|---|---|
| Union | (0,2,1) | 21−8√5 | shared circle |
| Common | (0,7,1) | 66−8√50 | shared circle |
| Difference | (3,4,1) | 42−8√17 | shared circle |
| Union | (0,7,1) | 84−10√59 | both retained spheres |
| Common | (0,2,1) | 39−10√14 | both retained spheres |
| Difference | (0,2,1) | 39−10√14 | retained cutter sphere |

The isolated circle foot is (0,4y/√(y²+z²),4z/√(y²+z²)). The Go comparison
checks it independently and retains both native original faces. The same
finite-precision allowances apply. Run
`TestOCCTSphereOverlapAlgebraicNearest` with the same environment variable and
a freshly built binary. The default original six records remain unchanged.
The additive exact query permits irrational query norms; it still requires a
rational intersection-circle radius and rejects axis/center minimizer families.

## Exact axis minimizer families

Optional `--axis` compares numerical distances only for complete symbolic
families. On the same R=5, centers±3 sources: union at either source center has
squared distance25 and a whole retained cap (including its two-owner rim);
union at the midpoint has squared distance16 and the whole circle; union at
x=-10 has squared distance4 and a single pole. Common at the midpoint has two
isolated poles at squared distance4. Common at x=-3 has the other sphere's pole
at squared distance1. Difference at cutter center x=3 has a whole inward-facing
retained cutter cap at squared distance25; at stock center x=-3 its nearest pole
has squared distance1. A separate union of R=2 spheres at centers±1 has a whole
radius-√3 circle at midpoint, exact squared distance3.

`TestOCCTSphereOverlapAxisFamilies` compares those nine independently built
OCCT results. OCCT representative points and solution counts do **not** prove
complete families. The exact Go proof uses a constant or monotone axial linear
functional on each entire retained cap, with explicit signed cap inequality,
original owner and two-owner boundary circle. No guessed representative is
returned for non-singleton families. Default and `--algebraic` modes remain
unchanged; rebuild the binary before running all three modes.

## Arbitrary original axis and irrational circle radius

Optional `--transverse` constructs radius-5 spheres at (0,0,0) and (2,2,1).
The original center axis is not a coordinate axis. Their common circle has
center (1,1,1/2), radius squared91/4 and plane normal(2,2,1). Queries:

| Operation | Query | Exact squared minimum | Feature |
|---|---|---|---|
| Union | (2,0,1/2) | 99/4−2√(91/2) | circle |
| Common | (5,−3,1/2) | 219/4−2√728 | circle |
| Difference | (6,−2,1) | 57−2√728 | circle |
| Union | (5,−3,1/2) | 237/4−10√(137/4) | retained spheres |
| Common | (2,0,1/2) | 117/4−10√(17/4) | retained spheres |
| Difference | (2,0,1/2) | 117/4−10√(17/4) | retained cutter |

An additional radius-2 centers±1 union at query(0,1,1) has exact distance
squared5−2√6 and an isolated foot on its irrational-radius circle. This case
also binds the native adapter; arbitrary center directions remain geometric
queries because the existing native constructor still accepts coordinate axes
only. `TestOCCTSphereTransverseNearest` compares all seven independent OCCT
results with the prior finite-precision allowances, including isolated feet.
Other three oracle modes remain unchanged.

## Arbitrary-axis complete families

Optional `--vector-axis` independently constructs R=5 spheres at (0,0,0) and
(2,2,2). The common plane has raw normal(2,2,2), point(1,1,1), circle radius²22.
The union at either source center has squared distance25 and an entire retained
cap. At the midpoint its minimum is the entire circle at squared distance22.
The common at midpoint has two algebraic pole minima, squared28−10√3. At source
center A, both common and A-minus-B have the retained B pole, squared37−10√12.
At B's center, A-minus-B has an entire inward-facing B cap, squared25. Union at
(-2,-2,-2) has a single retained A pole, squared37−10√12.

`TestOCCTSphereVectorAxisFamilies` checks eight numerical distances and
representative feet with the existing allowances. Complete minimizer sets are
established by the exact Go axial functional proof, not the number of OCCT
solutions. This arbitrary-axis case uses geometry queries; the native adapter
retains its existing coordinate-axis construction restriction. All four prior
oracle modes retain their original definitions.
