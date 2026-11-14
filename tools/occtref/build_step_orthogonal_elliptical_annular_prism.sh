#!/bin/sh
set -eu
[ "$#" -eq 3 ] || exit 2
exec "${CXX:-c++}" -std=c++17 -O2 -I"$1" -L"$2" "-Wl,-rpath,$2" \
 "$(dirname "$0")/step_orthogonal_elliptical_annular_prism.cxx" -o "$3" \
 -lTKDESTEP -lTKXSBase -lTKShHealing -lTKBO -lTKBool -lTKPrim -lTKTopAlgo -lTKBRep -lTKGeomBase -lTKG3d -lTKG2d -lTKMath -lTKernel
