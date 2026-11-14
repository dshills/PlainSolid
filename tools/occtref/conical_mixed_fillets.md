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

## Independent nearest-boundary differential

Rebuild the same executable and run:

```sh
PLAINSOLID_OCCT_CONICAL_FILLET=/tmp/conical-mixed-oracle \
  go test ./internal/occtcheck -run '^TestOCCTConicalNearestBoundary$' -count=1
```

The test invokes `OUTPUT_JSONL --nearest`. The default invocation and original
five records remain unchanged. The extra nine records use
`BRepExtrema_DistShapeShape` against a compound of the actual retained OCCT
faces, **not the solid** (solid-to-interior-point distance would be zero).
No PlainSolid point, mesh, curve, or source profile is supplied to OCCT.

Cases cover irrational-azimuth torus and cone interiors, rational-azimuth
irrational torus projection, cylindrical bore interior, blind floor, entry
rim, blind-floor rim, torus meridian-center family, and axis entry-rim family.
Go reconstructs and reloads its exact native recipe, requires proved nearest
status and a squared enclosure width at most 1e-12, and compares against both
OCCT and independent analytic formulas. OCCT tolerance is
`3e-7*max(1,squared distance)`; its native extrema search deflection is 1e-10.
Input constants such as 77/12 and -19/10 undergo binary64 rounding only on the
OCCT side. The allowance covers that and the reference model's finite precision;
it does not weaken the exact Go proof. Returned OCCT witness distance is also
checked against its reported value. OCCT solution counts are **not** interpreted
as exhaustive multiplicity: it returns representative points for continua.

Fresh observed squared values include torus 0.0098174049602652406,
cone 0.00089552792959004222, bore 0.12857864376269049,
entry rim 0.14420364376269049, floor rim 0.05851821881345242,
meridian family 0.87890625 and entry circle 0.265625.
The pinned source archive SHA256 is
`32b1e918a697a0398e8ce623099af2c1d62cdeda2b2eefa86f07c0aee9d4aaf7`.
The local fresh reference executable SHA256 was
`010345be3df3e1c3672239663305c155f08ac7f0060fc3b3ce7ac5bc4660cb70`;
executables are local verification artifacts, not committed dependencies.

## Independent cone/bore competition

`--bore-cone` independently cuts an OCCT radius-1 cylinder centered at
`(6,0)`, spanning z[-1,7], from the same original rounded cone. It emits the
stock and pierced solid plus boundary distances at `(65/8,1/8,3/2)`,
`(65/8,1/8,49/32)` and `(65/8,0,3/2)`. These are cone-nearest, bore-nearest
and an exact two-point tie, respectively. The Go test reloads its native
recipe and requires a complete original-owner classification. It compares
against independent analytic feet and values, and checks OCCT's one solid,
one shell, Euler zero, removed volume `6*pi` and added net area `10*pi`.
The earlier modes and record counts remain unchanged.

`TestOCCTConicalBoreConeNearest` uses the same `3e-7*max(1,D)` squared-distance
allowance and OCCT extrema deflection 1e-10 as above. Measured squared distances
were `1.264327332677972`, `1.2739034085183991` and exactly `1.265625`.
OCCT returned two representatives at the tie; its solution count is still
not treated as proof of exhaustive nearest-owner multiplicity. The exact Go
report independently proves the two original owners and excludes the rest.

The fresh executable built against the same pinned archive had SHA256
`48fe56ab65df799ba3332d277b00d17c9cff641bed37e71fc9fdb0e5870f5b85`.
Run with `PLAINSOLID_OCCT_CONICAL_FILLET` pointing to this rebuilt standalone
oracle; no executable or generated output is committed.
