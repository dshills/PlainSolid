#!/bin/sh
# Usage: sh tools/occtref/build_anisotropic_station_author.sh OCCT_INCLUDE OCCT_LIB OUTPUT
set -eu
[ "$#" -eq 3 ] || exit 2
exec "${CXX:-c++}" -std=c++17 -O2 -I"$1" -L"$2" "-Wl,-rpath,$2" \
 "$(dirname "$0")/anisotropic_station_author.cxx" -o "$3" \
 -lTKDESTEP -lTKXSBase -lTKXCAF -lTKCAF -lTKLCAF -lTKBO -lTKPrim -lTKTopAlgo -lTKBRep -lTKGeomBase -lTKG3d -lTKG2d -lTKMath -lTKernel
