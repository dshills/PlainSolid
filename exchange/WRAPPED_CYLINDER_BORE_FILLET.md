# Exact exchange for a rounded circular through-bore

The STEP and IGES `WriteWrappedCylinderBoreRimFillet` functions take a native
trimmed solid. They re-prove its parallel cylinders, annular caps, inside
quarter-toroid bands, and exact shared model edges. The fillet can be
at either end or both ends. The supports retain their original axes, heights
and radii. Material boundaries retain the model vertices and the four
original quadrant edges on each circular rim.

The STEP file contains one analytic manifold solid with explicit
`CYLINDRICAL_SURFACE` and `TOROIDAL_SURFACE` supports and model-space circular
trim edges. IGES uses one type 186 solid with analytic type 192 cylinders
and type 198 toroids. Each toroidal meridian is a shared circular trim edge.
Its IGES arc has a type 124 placement that maps the arc's XY plane to
the world XZ meridian. Both formats use metres per file length unit.

The live comparator reopens each file in pinned OCCT and checks positive
signed volume, surface area, every original vertex and edge incidence, and
the model-space trims on the analytic supports. At a cap, it checks that
the external boundary and the through-hole are separate wires.

```sh
c++ -std=c++17 -O2 -I"$OCCT_INCLUDE" tools/occtref/read_wrapped_cylinder_bore_fillets.cxx \
  -L"$OCCT_LIB" -Wl,-rpath,"$OCCT_LIB" -lTKDESTEP -lTKDEIGES -lTKXSBase \
  -lTKBRep -lTKTopAlgo -lTKGeomBase -lTKGeomAlgo -lTKG3d -lTKG2d -lTKMath -lTKernel \
  -o "$PWD/read-wrapped-cylinder-bore-fillets"
PLAINSOLID_OCCT_ECCENTRIC_BORE_RIM_TRANSFER="$PWD/read-wrapped-cylinder-bore-fillets" \
  go test ./exchange -run '^TestWrappedCylinderBoreRimFilletNeutralTransfer$' -count=1 -v
```

## Strict model-only STEP import

`ReadWrappedCylinderBoreRimFillet` reads one standalone analytic STEP
solid with explicit physical units. It checks the independent stock and
bore axes, annular cap bounds, toroidal meridian arcs and exact model
vertices. Every edge must have two opposite effective coedge uses. Full
circular rims use the same proof as the four-quadrant form. The imported
body records the original STEP face, edge and vertex IDs, and each native
quarter retains the source ID of its original full circle.

The pinned OCCT positive is a Boolean-cut tube with a 0.75 upper fillet
authored at stock Y=64 and bore Y=66. The author's model-only writer records
its seam line and vertices exactly at their analytic support. The origin
control records the outer seam at Y=-1.530808498934E-15 on a cylinder
centered at Y=0. The shifted lower control has two same-direction effective
uses of the shared shoulder. Both controls fail the strict import.

Compile `tools/occtref/eccentric_cylinder_bore_rim_fillets.cxx` with
`-DPLAINSOLID_OCCT_STEP` and link `TKDESTEP`, `TKXSBase`, `TKFillet`,
`TKBool`, `TKBO`, `TKPrim` and their geometry dependencies to regenerate
the independent author fixtures.

```sh
"$OCCT_BORE_RIM_AUTHOR" upper "$PWD/occt-upper.step" exact-model
"$OCCT_BORE_RIM_AUTHOR" upper "$PWD/occt-off-support.step" origin-model
"$OCCT_BORE_RIM_AUTHOR" lower "$PWD/occt-conflicting-bound.step" exact-model
go test ./exchange/step -run '^TestReadWrappedCylinderBoreRimFillet' -count=1 -v
```
