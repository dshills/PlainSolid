#!/bin/sh
# Usage: sh tools/occtref/build.sh OCCT_INCLUDE OCCT_LIB OUTPUT [step|step-iges]
set -eu
if [ "$#" -lt 3 ] || [ "$#" -gt 4 ]; then
  echo "usage: $0 OCCT_INCLUDE OCCT_LIB OUTPUT [step|step-iges]" >&2
  exit 2
fi
include=$1 lib=$2 output=$3
case "${4:-}" in
  "") set -- ;;
  step) set -- -DPLAINSOLID_OCCT_STEP -lTKDESTEP -lTKXSBase -lTKXCAF -lTKCAF -lTKLCAF ;;
  step-iges) set -- -DPLAINSOLID_OCCT_STEP -DPLAINSOLID_OCCT_IGES -lTKDESTEP -lTKDEIGES -lTKXSBase -lTKXCAF -lTKCAF -lTKLCAF ;;
  *) echo "unknown option: $4" >&2; exit 2 ;;
esac
exec "${CXX:-c++}" -std=c++17 -O2 -I"$include" -L"$lib" "-Wl,-rpath,$lib" \
  "$(dirname "$0")/main.cxx" -o "$output" "$@" \
  -lTKFillet -lTKBO -lTKPrim -lTKTopAlgo -lTKBRep -lTKGeomBase -lTKG3d -lTKG2d -lTKMath -lTKernel
