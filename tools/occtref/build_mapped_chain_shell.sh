#!/bin/sh
# Usage: sh tools/occtref/build_mapped_chain_shell.sh OCCT_INCLUDE OCCT_LIB OUTPUT [step]
set -eu
case "$#" in
  3) ;;
  4) [ "$4" = step ] || exit 2 ;;
  *) exit 2 ;;
esac
compiler="${CXX:-c++}"
source="$(dirname "$0")/mapped_chain_shell.cxx"
if [ "$#" -eq 4 ]; then
  exec "$compiler" -std=c++17 -O2 -I"$1" -L"$2" "-Wl,-rpath,$2" \
    -DPLAINSOLID_AUTHORED_STEP "$source" -o "$3" \
    -lTKDESTEP -lTKXSBase -lTKTopAlgo -lTKBRep -lTKGeomBase -lTKG3d -lTKG2d -lTKMath -lTKernel
fi
exec "$compiler" -std=c++17 -O2 -I"$1" -L"$2" "-Wl,-rpath,$2" \
  "$source" -o "$3" -lTKTopAlgo -lTKBRep -lTKGeomBase -lTKG3d -lTKG2d -lTKMath -lTKernel
