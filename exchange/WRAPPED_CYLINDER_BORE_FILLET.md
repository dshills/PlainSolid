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
