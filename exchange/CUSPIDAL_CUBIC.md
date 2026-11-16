# Original cuspidal cubic Common transfer

`step.WriteCuspidalCubicCommon` and `iges.WriteCuspidalCubicCommon`
transfer the source-proved intersection of two original rectangular
stocks with a genuine cuspidal cubic section. Their verifiers require
the original native Common and the intended physical units. The fixtures
use millimetres and an explicit `1e-14` coordinate bound for rounding
derived interior poles to decimal. Original vertices retain exact
coordinates.

Both original roof orders reopen in the two pinned OCCT readers as one
valid nine-face solid with 20 edges and 13 vertices. The independent
readback measures volume `277485/22528` and area
`32.5442218530794`. A separate C++ probe evaluates the two actual
reopened model curves at the cusp: both first derivatives are zero,
both second derivatives agree, and the third derivatives have opposite
lateral directions.

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
  go test ./exchange -run '^TestCuspidalCubicCommonExchange$' -count=1 -v
```

The regression also refuses a changed STEP curve master or IGES unit
declaration. This is canonical verification of admitted original
geometry; independent reconstruction of arbitrary foreign cusp shells
requires further work.
