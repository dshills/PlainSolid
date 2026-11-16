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

## Reused subassembly

`ReadNestedMixedConicalAP242Assembly` authenticates one assembly-only
group referenced two to eight times by the root. The group has two
to eight original material uses, including both curved families. Its
local product geometry is validated once. Each expanded instance
retains both original NAUO IDs and the exact product of the two source
poses. The flat reader remains an independent two-leaf entry point.

The pinned nested author reuses the same mixed group twice. Its two
parent rotations use the rational 3–4–5 ratios around different axes.
Six leaf instances expand from five source uses and two unchanged
native solids. Path `[58,416]` composes two azimuth rotations with
cosine `-7/25` and sine `24/25`. The other occurrence of that
chamfered part, at path `[422,416]`, sends local point `(1.5,2,4)`
to exact world coordinates `(-1007/10,-1154/25,378/25)`.

The reference check reopens the nested source in pinned XCAF, verifies
both group references point to one original definition, and compares
all six world poses, topology counts and signed volumes. The fixture
also tests a valid common assembly graph that misattaches a direct leaf.
The focused reader rejects that graph, duplicate transformation
ownership, and inexact source directions at either level.

```sh
PLAINSOLID_OCCT_MIXED_CONICAL_AP242="$PWD/author-ap242-mixed-conical-curved" \
  go test -race ./exchange/step -run '^(TestReadNestedMixedConicalAP242Assembly.*|TestNestedMixedConicalAP242PinnedAuthor)$' -count=1 -v
```
