# Independent conical rolling-fillets oracle

Build `conical_mixed_fillets.cxx` against the OCCT revision specified in
`README.md`. This standalone executable does not read PlainSolid geometry.
For the existing macOS development build:

```sh
c++ -std=c++17 -O2 \
  -I/tmp/plainsolid-occt-build/include/opencascade \
  -L/tmp/plainsolid-occt-build/mac64/clang/lib \
  -Wl,-rpath,/tmp/plainsolid-occt-build/mac64/clang/lib \
  tools/occtref/conical_mixed_fillets.cxx -o /tmp/conical-mixed-oracle \
  -lTKFillet -lTKBO -lTKPrim -lTKTopAlgo -lTKBRep \
  -lTKGeomBase -lTKG3d -lTKG2d -lTKMath -lTKernel
PLAINSOLID_OCCT_CONICAL_FILLET=/tmp/conical-mixed-oracle \
  go test ./internal/occtcheck -run '^TestOCCTConicalMixedFillets$' -count=1
```

OCCT authors a cone with radii 8/16 and height 6, then rolls radii 15/16
and 15/8 on its lower/upper circular rims using `BRepFilletAPI_MakeFillet`.
It subtracts radius-1/2 cylinders at x=-2 (through), x=2 (lower depth 1),
and x=2 (upper depth 4). No native faceting or extracted PlainSolid profile
is used. The Go side independently constructs its exact rational supports.

The comparison checks native area, volume, centroid, one valid OCCT solid
and shell, and Euler characteristic 0 for the bored result. Euler counts
face contributions as 2 minus the number of wires; counting every holed
face as a disk would give the wrong genus. Removed volume is independently
11*pi/4 and added area 21*pi/2. Three finite lines compare 14 intersection
parameters against exact algebraic root enclosures, including reversed
parameter direction and both blind zones. Reference duplicate line hits
are merged at 1e-8; comparison tolerance is 3e-7, not an exact OCCT claim.

Observed reference values: stock volume 2301.2509568979344, area
1151.2281711040541; mixed volume 2292.6115771005625, area
1184.2148939667472. The optional test skips without its environment variable.

The local verification used the previously built OCCT libraries corresponding
to `/tmp/plainsolid-occt-4dbfdcc.tar.gz`; its independently inspected archive
root is `OCCT-4dbfdcc448c0b3e565488afcd626c84483172db9/`. The source checkout is
an extracted archive, not a Git clone. Area and volume comparisons allow
3e-7 relative error, centroid comparisons 3e-7 model units; exact Go roots
are enclosed to parameter width 1e-9 before the reference comparison.
