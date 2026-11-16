#!/bin/sh
# Usage: sh tools/occtref/build_orthogonal_volume_offset.sh OCCT_INCLUDE OCCT_LIB OUTPUT
set -eu
[ "$#" -eq 3 ] || exit 2
exec "${CXX:-c++}" -std=c++17 -O2 -I"$1" -L"$2" "-Wl,-rpath,$2" \
  "$(dirname "$0")/orthogonal_volume_offset.cxx" -o "$3" \
  -lTKBO -lTKPrim -lTKTopAlgo -lTKBRep -lTKGeomBase -lTKG3d -lTKMath -lTKernel
