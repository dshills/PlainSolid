#!/bin/sh
set -eu
[ "$#" -eq 3 ] || { echo 'usage: build_frustum_bore_distance.sh OCCT_INCLUDE OCCT_LIB OUTPUT' >&2; exit 2; }
exec "${CXX:-c++}" -std=c++17 -O2 -I"$1" -L"$2" "-Wl,-rpath,$2" \
  "$(dirname "$0")/frustum_bore_distance.cxx" -o "$3" \
  -lTKBO -lTKPrim -lTKTopAlgo -lTKBRep -lTKGeomBase -lTKG3d -lTKG2d -lTKMath -lTKernel
