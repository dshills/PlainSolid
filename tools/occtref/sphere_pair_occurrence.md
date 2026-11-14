# Proper rational placed sphere-pair query reference

Independent OCCT source: `4dbfdcc448c0b3e565488afcd626c84483172db9`.
Archive `/tmp/plainsolid-occt-4dbfdcc.tar.gz`, SHA-256
`32b1e918a697a0398e8ce623099af2c1d62cdeda2b2eefa86f07c0aee9d4aaf7`.

The standalone C++ fixture constructs analytic radius-5 spheres directly at
world images of (-3,0,0) and (3,0,0), then independently Fuse/Common/Cuts them.
The proper orthogonal matrix rows and translation are:

```
(3/13, -12/13, 4/13; 1/7)
(36/65, 5/13, 48/65; -2/9)
(-4/5, 0, 3/5; 3/11)
```

There is no PlainSolid-generated geometry or faceting input. Each OCCT Boolean
must be valid and contain one solid. OCCT uses finite precision representations
of these exact inputs; this is a numerical differential, not an exact oracle.

Nearest queries are world images of (0,2,1), (0,7,1), and (3,4,1), for union,
common and difference respectively. All have isolated nearest shared-circle
feet with irrational azimuth. Boundary distance uses an independent compound
of retained OCCT faces (not distance to the filled solid). The Go test checks
both world foot coordinates and squared distance against the exact symbolic
occurrence report, and verifies both original native owners. OCCT solution
counts are not used to establish complete owner sets.

Finite lines originate at the transformed source origin with transformed raw
direction ±(2/3,0,0). Original t is preserved: OCCT's normalized physical line
parameter is multiplied by 3/2. Windows [-20,20] and [-9,9] test real boundary
crossings and interior clipping. The latter produces zero boundary-owner masks
at clipped material endpoints. Each independently obtained interval midpoint
must classify inside the OCCT solid. Forward and reversed lines cover 12 cases.

OCCT intersection/extrema tolerance is 1e-10. Comparator allowances are 1e-7
for raw t, 1e-6 for world feet, and 3e-7*max(1,d²) for squared distance.
Oracle-only duplicate intersections within 1e-8 are merged on these widely
separated roots; no tolerance establishes kernel owners, roots or ties.

Build from repository root:

```sh
c++ -std=c++17 -O2 \
 -I/tmp/plainsolid-occt-build/include/opencascade \
 -L/tmp/plainsolid-occt-build/mac64/clang/lib \
 -Wl,-rpath,/tmp/plainsolid-occt-build/mac64/clang/lib \
 tools/occtref/sphere_pair_occurrence.cxx \
 -o /tmp/plainsolid-sphere-occurrence-oracle \
 -lTKBO -lTKPrim -lTKTopAlgo -lTKBRep -lTKGeomAlgo -lTKGeomBase \
 -lTKG3d -lTKG2d -lTKMath -lTKernel
PLAINSOLID_OCCT_SPHERE_OCCURRENCE=/tmp/plainsolid-sphere-occurrence-oracle \
 GOCACHE=/tmp/plainsolid-go-cache go test ./internal/occtcheck \
 -run '^TestOCCTSpherePairOccurrence$' -count=1
```

Qualified local executable SHA-256:
`a93744cf19cae9053898e507d9c38be872b056e7388eea11093eb6158edeaf85`.
No executable or generated reference data is committed.
