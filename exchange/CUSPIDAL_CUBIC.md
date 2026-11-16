# Original cuspidal cubic transfer

`step.WriteCuspidalCubicCommon` and `iges.WriteCuspidalCubicCommon`
transfer the source-proved intersection of two original rectangular
stocks with a genuine cuspidal cubic section. Their verifiers require
the original native Common and the intended physical units. The fixtures
use millimetres and an explicit `1e-14` coordinate bound for rounding
derived interior poles to decimal. Original vertices retain exact
coordinates.

`step.WriteCuspidalCubicFuse` and `iges.WriteCuspidalCubicFuse` transfer
the upper envelope using the same original-support and coedge proof.
The verifier for each format authenticates its transfer against its typed
source Fuse and the intended units.

Both original roof orders reopen in the two pinned OCCT readers as one
valid nine-face solid with 20 edges and 13 vertices. The independent
readback measures volume `277485/22528` and area
`32.5442218530794` for Common. Fuse keeps the same entity counts and
reopens at volume `279423/22528` and area `32.8681291383202`.
A separate C++ probe evaluates the two actual
reopened model curves at the cusp: both first derivatives are zero,
the second derivatives are `(4.5, 0, 0.3515625)`, and the
third derivatives are `(0, ±4.5, ±0.052734375)` for the fixture's
normalized outward parameters.

Build the [pinned component readers](iges/CLIPPED_POLYNOMIAL_CUT.md)
and the extra derivative check against the same OCCT build:

```sh
c++ -std=c++17 -O2 -I"$OCCT_INCLUDE" -L"$OCCT_LIB" -Wl,-rpath,"$OCCT_LIB" \
  tools/occtref/cuspidal_cubic_readback_jets.cxx \
  -o /tmp/plainsolid-occt-cuspidal-cubic-jets \
  -lTKDESTEP -lTKDEIGES -lTKXSBase -lTKTopAlgo -lTKBRep -lTKGeomBase -lTKG3d -lTKMath -lTKernel

PLAINSOLID_OCCT_CLIPPED_CUT_STEP_COMPONENTS=/tmp/plainsolid-occt-clipped-cut-step \
PLAINSOLID_OCCT_CLIPPED_CUT_IGES=/tmp/plainsolid-occt-clipped-cut-iges \
PLAINSOLID_OCCT_CUSPIDAL_CUBIC_JETS=/tmp/plainsolid-occt-cuspidal-cubic-jets \
  go test ./exchange -run '^TestCuspidalCubic(Common|Fuse)Exchange$' -count=1 -v
```

The regression also refuses a changed STEP curve master, a changed IGES
unit declaration, or a truncated Fuse coedge proof. This is canonical verification of admitted original
geometry; independent reconstruction of arbitrary foreign cusp shells
requires further work.
