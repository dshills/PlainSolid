# Independent anisotropic weighted station reference

`anisotropic_station_material_author.cxx` authors original OCCT quadratic-by-linear
NURBS bands, planar terminal faces and a real `BRepAlgoAPI_Cut`. It never reads
PlainSolid geometry, tessellation, recipes or exported STEP. The generated STEP
is an independently authored artifact; this test does not claim importer support.
OCCT remains an optional test dependency, not a runtime dependency.

Qualified against OCCT source `4dbfdcc448c0b3e565488afcd626c84483172db9`
(version reported as 8.1.0). Example build using the existing pinned layout:

```sh
c++ -std=c++17 -O2 \
  -I/tmp/plainsolid-occt-build/include/opencascade \
  -L/tmp/plainsolid-occt-build/mac64/clang/lib \
  -Wl,-rpath,/tmp/plainsolid-occt-build/mac64/clang/lib \
  tools/occtref/anisotropic_station_material_author.cxx \
  -o /tmp/plainsolid-anisotropic-station-material-author \
  -lTKDESTEP -lTKXSBase -lTKXCAF -lTKCAF -lTKLCAF -lTKBO -lTKPrim \
  -lTKTopAlgo -lTKBRep -lTKGeomBase -lTKG3d -lTKG2d -lTKMath -lTKernel
PLAINSOLID_OCCT_ANISOTROPIC_STATION_MATERIAL_AUTHOR=/tmp/plainsolid-anisotropic-station-material-author \
  go test ./brep/trimmed -run '^TestIndependentAnisotropicStationOCCT$' -count=1 -v
```

## Original geometry

The profile has the nine canonical quadrant control positions
`(1,0),(1,1),(0,1),(-1,1),(-1,0),(-1,-1),(0,-1),(1,-1),(1,0)`.
Weights are `1,.5,1,.75,1,.75,1,.75,1`; the unequal first weight produces
nonzero transverse centroid shifts. Original U knots are `-2,-1,1,4,8`,
with multiplicities `3,2,2,2,3`. V spans `[2,5]`. The center is `(1/4,-1/2)`.
Station entries are `(height, X scale, Y scale)`:

- Outer: `(-4,3,1),(-3,1,2),(-1,2,1)`.
- Inner: `(-4,.5,.25),(-2,.25,.5),(-1,.375,.25)`.

Thus both aspect ratios vary, and the two source wall grids have distinct
internal heights. The filled body has one shell and Euler characteristic 2;
the cut body has one shell, two annular faces and Euler characteristic 0.
Euler accounting subtracts the two additional face wires.

## Numerical evidence and limits

Volume/centroid use OCCT's span-aware `VolumePropertiesGK` with requested
precision `1e-12`. Area uses original OCCT surface `D1` integrated separately
on each original U span at Gauss orders 32 and 48. Terminal areas use a separate
rational planar strip from the original inner to outer profile (the filled
strip collapses its inner row to the center). This is an integration chart,
not a replacement for the independently sewn planar faces. The quadrature
change must be at most `1e-9`; this convergence check is numerical evidence,
not a rigorous interval proof. PlainSolid's certified enclosures are checked
separately with `1e-7` comparator slack. Ordinary native boundary quadrature
must also agree within `1e-7` for volume/centroid and `1e-6` for area.

Ordinary OCCT `SurfaceProperties` is retained as `WholeFaceArea`. On this
fixture it differs from the converged D1 area by approximately `4.16e-5`.
Its internal integration discrepancy is not treated as a kernel error or
hidden by widening the primary comparison; its precise source is not isolated.

The oracle checks 40 filled / 80 bore original wall and rim samples against
the actual resulting OCCT boundary with `BRepExtrema_DistShapeShape` (distance
at most `1e-8`). The Go test maps each sample to the authenticated original
native face after JSON reload/reindexing, compares exact rational evaluation
within `2e-11`, and verifies inward bore ownership and exact material boundary
classification. This is finite independent qualification, not exhaustive
numerical proof of all source points.

Observed reference values:

| Body | Volume | Area | Centroid |
| --- | --- | --- | --- |
| Filled | 22.3424781002 | 51.6190403738 | (0.229039835805, -0.518127709574, -2.60465116279) |
| Bore | 21.1409204117 | 57.8894390747 | (0.228094691709, -0.518897252905, -2.60714285714) |
