# Native rational mapped planar document

`RationalMappedPlanarDocument` is a separate immutable document envelope for the
exact rational mapped STEP domain. It does not install approximate transforms or
identity stand-ins in ordinary `State.Nodes`. Original shared local planar
assets, their names, units, source identity and exact occurrence graph survive
native persistence. Accessors return local bodies, copied map IDs and immutable
exact occurrence poses. This type does not add rational transforms to the generic
floating document schema.

`ReadRationalMappedPlanarSTEP` validates original source topology, model/owner
curves, material and exact poses through the dedicated exchange reader. Native
version1 stores the original source and existing native document encoding of the
local assets. `ReadRationalMappedPlanarDocument` bounds the entire envelope,
validates the native payload, independently re-proves the source, and compares
canonical native local geometry and metadata before returning anything. Forged
native assets, source/metadata mismatch, unknown fields, invalid versions,
trailing payload and resource limits fail atomically. No cache bypass or raw
rational-to-float coercion is used.

`ExactWorldBounds` compares transformed planar vertices using exact rationals.
These bound all finite planar faces and material, including nonconvex/holed local
polygons. Returned extrema are owned copies in model coordinate units.
`BakeOccurrence` first requires every transformed vertex to be binary64-exact;
only then does it build a native planar solid with original edge/face ordering.
Any unrepresentable point returns no partial material. Original shared local
assets are unchanged. There is no implied general floating world-solid cache.

Tests retain two local assets and three occurrences across native reload.
The noncardinal3:4:5 pose gives exact world bounds(−10,0,0) to(56/5,22,4), while its
small original box cannot be baked without rounding and is rejected. A box with
side lengths10,15,20 has exactly representable rotated vertices and bakes with
volume3000. An additional translation component10^−40 survives persistence and
appears exactly in the rational maximum bound, even though a floating transform
would lose it. Mutating returned bound rationals cannot alter future queries.

`WriteSTEP` restores the validated original graph. Independent pinned OCCT
TransferRoots/BRepCheck/AddOptimal after STEP→native→reload→STEP confirms the
same bounds for the original retained independently authored fixture. Enable
`TestOCCTRationalMappedNativeBounds` with `PLAINSOLID_OCCT_MAPPED_BOUNDS`.
OCCT's floating measurement is independent interoperability evidence for that
fixture, not a promise to preserve every rational component in OCCT doubles.
