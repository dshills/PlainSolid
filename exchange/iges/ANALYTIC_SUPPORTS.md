# Mixed independent analytic IGES supports

`ReadAnalyticSupports` / `WriteAnalyticSupports` exchange an ordered mixture of
190 plane, 192 cylinder, 196 sphere and 198 ring-torus form1 supports. Each
`AnalyticSupport` contains exactly one typed variant. Root directory generation
order, all dependency labels/subscripts and explicit length units survive.
The explicit IGES-only `ReadAnalyticSupportsWithCones` and
`WriteAnalyticSupportsWithCones` also admit type194 form1 cones with the
proved 45-degree generatrix convention. The original mode and the STEP bridge
exclude these cones because the STEP axial parameter differs.

Each support uniquely owns one point116 and two direction123 entities across
the entire file. Shared axes/centers, unknown roots, unused entities and wrong
reference kinds fail before per-root construction. Source numeric tokens are
checked for exact representability before remapping; staging cannot turn a
Hollerith string or malformed numeric token into an accepted scalar. Each
partition then passes its existing specialized codec. Global source metadata
outside explicit units and directory labels/subscripts is not represented;
presentation style and geometry usage statuses follow the specialized codecs.

The writer stages each variant, remaps only owned references, and strictly
re-reads the whole output before caller I/O. Validation errors are atomic;
short writes and caller I/O errors propagate. Independent equal-valued supports
are allowed, but shared source identities are not flattened. There are no
assemblies, finite trims, faces or material inferred by this API.

Bounds are 64 supports, 256 entities, 1024 records, 8192 values and 4 MiB input,
with smaller caller limits respected. Root shape/parameter/frame restrictions
remain those of each strict support codec. The independent OCCT fixture has
four types with distinct centers. Optional
`PLAINSOLID_OCCT_IGES_ANALYTIC_SUPPORTS` tests compare original and Go-authored
analytic support points and both derivatives in root order and both axis signs.
The separate mixed-cone fixture has all five types and checks both nappes of
the original and re-emitted cone against OCCT. The cone's labels/subscripts
use the same independent datum ownership proof as the other roots.
This is geometry-entity qualification, not BRep transfer.
