# Strict AP242 assembly import for mixed exact conical materials

`ReadMixedConicalAP242Assembly` reads one flat AP242 product graph
containing a conical stock with two rolling rim fillets and an eccentric
through-bore, alongside a chamfered eccentric annular stock. Each leaf
geometry closure has one original product owner. Each local solid is
authenticated by its standalone exact STEP reader while retaining its
original entity IDs and length unit. Repeated NAUO occurrences share
the same local product proof.

The assembly reader checks the AP242 product envelope, representation
ownership, reachable entity graph, placement link ownership and common
units. The exact reader decodes placement frames from the original
rational coordinate records. The fixture exercises a translation, a
cyclic three-dimensional rotation, and a planar rotation with cosine
`3/5` and sine `4/5`. The unchanged body coordinates remain local, and
the returned occurrence exposes its exact rational matrix.

The pinned OCCT author independently makes both solids using cone
construction, a Boolean cut, and the fillet and chamfer builders.
XCAF authors a flat assembly with three uses and two distinct local
products. The alternate exact carrier selects the original cone
generators as two-pole splines in the OCCT STEP model. It also selects
the circle and toroid gauges and the lower rolling face's directed
bound while keeping the placement records and shared vertices.
The default author output with rounded angles remains a rejected control.

Pinned XCAF reopens the exact source with one conical rounded product
used twice and one distinct chamfered product. Signed volume, surface
area, world centroid, all original topology counts, and rigid poses
are compared per use. Both strict local solids serialize identically
to their independently authored standalone counterparts.

```sh
c++ -std=c++17 -O2 -I"$OCCT_INCLUDE" tools/occtref/author_ap242_mixed_conical_curved.cxx \
  -L"$OCCT_LIB" -Wl,-rpath,"$OCCT_LIB" -lTKDESTEP -lTKXSBase -lTKXCAF -lTKCAF -lTKLCAF \
  -lTKFillet -lTKBool -lTKPrim -lTKBO -lTKBRep -lTKTopAlgo -lTKGeomBase -lTKGeomAlgo \
  -lTKG3d -lTKG2d -lTKMath -lTKernel -o "$PWD/author-ap242-mixed-conical-curved"
PLAINSOLID_OCCT_MIXED_CONICAL_AP242="$PWD/author-ap242-mixed-conical-curved" \
  go test -race ./exchange/step -run '^(TestReadMixedConicalAP242Assembly.*|TestMixedConicalAP242PinnedAuthor)$' -count=1 -v
```
