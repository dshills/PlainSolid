# Strict STEP import of two conical rim fillets and an eccentric bore

`ReadConicalRimFilletBore` accepts one ascending vertical conical stock
with both outer rolling-rim fillets and one eccentric vertical
through-bore. The model-only source has six original faces, including
two annular caps. Its cone carries a finite two-pole meridian revolved
about its original vertical axis. The rolling surfaces are analytic
toroids. Six complete circles, two directed meridian arcs, and two
straight generators carry the original directed edge incidence.

The reader retains the original file units and source owner IDs. It
recovers the sharp stock from the exact finite cone generator and
original cap planes. The checked native rolling-ball construction
proves tangency on the whole shared rings, and the bore constructor
proves separation. Exact edge endpoints, circle supports, arc traces,
face senses, and complete directed source loops must match the native
boundary. Full source circles subdivide into native quadrant edges.
New cardinal vertices have source ID zero.

The pinned OCCT author rolls radii `15/16` and `15/8` onto a cone of
original radii 8 and 16 and height 6. It independently cuts a radius-1
through-bore whose axis is offset by `(1.5,2)` from the stock axis. The
OCCT STEP writer transfers the complete authored solid. In a second
source carrier, the author uses an exact two-pole cone generator and
model seam, cardinal gauges for the analytic circles and toroids, and
the lower toroid bound orientation that opposes its adjacent source
uses. Both STEP files keep all original vertex entities and positions,
cap wires, faces, and edge ownership. OCCT serializes both variants.

The default author's cone angle `0.927295218002` radians has tangent
different from the exact slope `4/3` required by its original trim
circles. Its meridian circle direction also contains a small nonzero
horizontal component outside the exact meridian plane. The strict
reader rejects the original angle export and the original lower bound
orientation even when paired with exact geometry. Pinned OCCT reads
the exact carrier with positive signed volume and retains all six
original vertices and both cap holes.

The returned material exposes `TessellateCertified`. It reuses
the authenticated stock, fillet radii and bore from the original graph.
The mesh certificate covers both deflection directions and records the
original native owner of every triangle.

```sh
c++ -std=c++17 -O2 -I"$OCCT_INCLUDE" tools/occtref/author_step_conical_rim_fillet_bore.cxx \
  -L"$OCCT_LIB" -Wl,-rpath,"$OCCT_LIB" -lTKDESTEP -lTKXSBase -lTKFillet -lTKBool \
  -lTKPrim -lTKBO -lTKBRep -lTKTopAlgo -lTKGeomBase -lTKGeomAlgo -lTKG3d -lTKG2d -lTKMath -lTKernel \
  -o "$PWD/author-step-conical-rim-fillet-bore"
c++ -std=c++17 -O2 -I"$OCCT_INCLUDE" tools/occtref/read_step_conical_rim_fillet_bore.cxx \
  -L"$OCCT_LIB" -Wl,-rpath,"$OCCT_LIB" -lTKDESTEP -lTKDEIGES -lTKXSBase -lTKBRep -lTKTopAlgo \
  -lTKGeomBase -lTKGeomAlgo -lTKG3d -lTKG2d -lTKMath -lTKernel \
  -o "$PWD/read-step-conical-rim-fillet-bore"
PLAINSOLID_OCCT_CONICAL_RIM_BORE_STEP_AUTHOR="$PWD/author-step-conical-rim-fillet-bore" \
PLAINSOLID_OCCT_CONICAL_RIM_BORE_STEP_READER="$PWD/read-step-conical-rim-fillet-bore" \
  go test -race ./exchange/step -run '^(TestReadConicalRimFilletBoreSTEP.*|TestConicalRimFilletBorePinnedSTEPAuthor)$' -count=1 -v
```
