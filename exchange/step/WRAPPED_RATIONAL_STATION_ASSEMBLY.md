# Shared original weighted station products in STEP

`ReadWrappedRationalStationAssembly` adds a genuine independently authored
curved NURBS assembly route. It authenticates one weighted station loft local
product with 2–16 flat, proper signed-coordinate uses and a geometry-free root.
It reuses `ReadWrappedRationalStationLoft` for the complete local material proof;
no transform is baked into any original surface or model/pcurve edge.

The local product retains 1–16 original degree2×1 wrapped weighted-quadrant
bands, shared exact station rims and planar caps. Native geometry is reconstructed
by the existing exact station constructor, not by incidence alone. The return
value preserves the source product/definition/representation, original closed
shell face IDs, band-face IDs in station order, every wall's original seam/rim
edge and vertex IDs, and source NAUO/CDSR/relationship/placement identities.
`ExactNAUOOccurrence` keeps each exact mapping separately from shared geometry.

## Explicit assembly proof

Exactly two product definitions are supported, with one representation each:
root `SHAPE_REPRESENTATION` and one local `ADVANCED_BREP_SHAPE_REPRESENTATION`
containing one `MANIFOLD_SOLID_BREP`. Each occurrence must connect that same
root/leaf pair. Relationships and transformations cannot be borrowed by another
occurrence. The original child frame must be identity; parent frames must be
exactly represented proper signed-coordinate rigid placements. Source rational
frame proof is compared with the parsed mapping. Mixed units and nonneutral
child conventions are not admitted. Common metre and millimetre contexts retain
their original semantics; no automatic rescaling occurs.

The entire file must be covered by authenticated representation/application/
relationship roots and optional uniquely owned product categories. Every
nonmetadata geometry record must belong to the single body or an explicit
representation placement. Detached geometry, styles, other material families,
extra products and unsupported entity roots fail. Original record reordering
is accepted through graph identity, not numeric role positions.

Input limits cap source at 1 MiB/4096 entities/65536 values, products at two,
uses at 16, and retain the supplied local wrapped-surface work/pole/bit limits.
There is one local material proof, regardless of instance count. Cancellation
is observed around the bounded underlying decoders and before output, not as
an interruptible legacy parser. The writer limits names/IDs to 32768 bytes and
uses the existing bounded local writer before readback under caller limits.

## Output and native round trip

`WriteWrappedRationalStationAssembly` stages standard product/NAUO/CDSR graph
records around one exact local weighted station body, then independently reads
the complete generated assembly before writing anything. Input product IDs
and occurrence strings follow the existing assembly model conventions. Exactly
one part has a nonnil `Body`; the other is the root. `Body.LengthUnit` must equal
the model unit. Original wall supports and names are authority; cached `Solid`
and source edge numbers do not replace geometric reproof.

`WriteWrappedRationalStationAssemblySource` separately re-proves and re-emits
original source, preserving source record identities. Generated output creates
new valid identities. Neither route claims regularized union of occurrences.
The local native `trimmed.Solid` can be saved/reloaded and recovered with
`WrappedRationalStationLoftFromSolid` before generating a new shared assembly.
This milestone does not introduce a source-bound native *assembly* envelope.

The existing standalone weighted and mixed wrapped-material readers remain
unchanged. The independently authored noncircular fixture was rejected by the
older mixed route; it is selected explicitly by this new API. Bore products,
multiple independent local products, style inheritance, arbitrary rational
poses and general curved NURBS material are outside this milestone.

## Independent original and generated transfer

`tools/occtref/author_ap242_weighted_station_assembly.cxx` builds and sews original
weighted supports directly in OCCT (same independent local construction as
`weighted_station_author.cxx`), then adds an XCAF shared product and translated/
cyclically rotated uses. It never reads Go output while authoring. Pinned OCCT
is `4dbfdcc448c0b3e565488afcd626c84483172db9`.

The profile has center (1/4, −1/2), square quadrant poles, weights
[1,1/2,1,3/4,1,3/4,1,3/4,1], original knots [-2, −1, 1, 4, 8], heights [0, 1, 3]
and scales [3, 1, 2]. It is noncircular. The retained original AP242 has 258 records;
only trailing whitespace is normalized. The observer independently checks
BRepCheck, two solids/eight faces, common referenced XCAF product, both full
world poses, adaptive volume/area and centroid.

Tests qualify retained source, source re-export, a freshly authored file, and
native-save/reload→generated STEP. All agree within 1e-6 measure/1e-7 centroid
comparison tolerances. The source geometry acceptance itself has no new
rounding tolerance. Original total volume is approximately 56.1159919582 and
area 178.858998913; volume also agrees with an independent rational Green-profile
integral times the exact axial station integral. These are occurrence sums.

```sh
sh tools/occtref/build_author_ap242_weighted_station_assembly.sh \
 /tmp/plainsolid-occt-build/include/opencascade \
 /tmp/plainsolid-occt-build/mac64/clang/lib /tmp/plainsolid-ap242-weighted-station-assembly
PLAINSOLID_OCCT_WEIGHTED_STATION_ASSEMBLY=/tmp/plainsolid-ap242-weighted-station-assembly \
 go test ./exchange/step -run '^TestWrappedRationalStationAssembly' -count=1
```

Mutation checks cover raw placement decimal rounding, nonneutral source frame,
changed model endpoint, wrong product link/category, detached body/point,
limits, cancellation, short writes and atomic rejected output. Original spline
coefficient arrays are compared against independently constructed native data,
and the source-role test reverses all record order. Fuzz smoke uses 40 executions,
64 KiB input, 512 entities and a million-operation local proof ceiling.

## Source-bound native persistence

`WriteWrappedRationalStationAssemblyJSON` and
`ReadWrappedRationalStationAssemblyJSON` use the distinct version-one
`plainsolid-step-wrapped-rational-station-assembly` envelope. It stores one
local native solid, the original STEP source, original tensor support bases,
edge/vertex/face roles, product metadata, and each exact occurrence matrix.
Reload performs one complete STEP assembly/local-material reproof and compares
all stored claims; it never constructs geometry from the native claim alone.
Writer validation finishes before output, and failed reads return no partial
assembly. Byte and source parsing limits and cancellation remain enforced.
This format supports the filled family only; it does not change older native
formats or promote unrelated through-bore/assembly graphs.

The optional pinned XCAF test includes source → native envelope → reload →
source/generated STEP and checks shared identity, both poses, volume, area and
centroid. Mutation tests cover support weights, original owner IDs, source
units, occurrence matrices, native geometry and envelope versions.

## Separate through-bore assembly

`ReadWrappedRationalStationBoreAssembly`,
`WriteWrappedRationalStationBoreAssembly` and its `Source` writer admit one
strict weighted through-bore local product under the same flat two-product,
2..16-use graph. They retain separate bottom-to-top original outer and inner
wall face IDs and all annular-cap face records. The local proof is exactly
`ReadWrappedRationalStationBore`: both independently partitioned station grids,
positive weighted profiles, strict radial nesting on every station slab,
reversed inner walls, and shared exact end annuli. The shared assembly layer
maps edge identities only after that full orientation proof; false inner
`FACE_BOUND` senses are not changed in the original source.

The filled reader and filled native tag still reject this family. The STEP slice provides local native material reload plus source/generated
assembly transfer; the separate native envelope is described below. The independent XCAF author accepts
`write-bore output.step`, builds two rational lofts independently and cuts the
inner from the outer before adding two shared product uses. Its retained
450-entity source and generated output transfer as two solids / twelve face
uses with the same poses, approximately 48.711798213 volume, and agreeing
adaptive area and centroid. All values are model-coordinate measures; the
source unit remains millimetres. Source output preserves IDs, generated output
creates new IDs. No general XCAF or vendor approximation acceptance is added.

### Through-bore source-bound native envelope

`WriteWrappedRationalStationBoreAssemblyJSON` and
`ReadWrappedRationalStationBoreAssemblyJSON` now supply the separate
`plainsolid-step-wrapped-rational-station-bore-assembly` version-one format.
One complete original STEP reproof authenticates the actual bore material,
both original support chains, their separately recorded source wall roles,
annular cap names, product/unit metadata, and every occurrence relationship
and exact matrix. The returned body's auxiliary filled outer/inner profile
solids are also bound as derivation claims; they are not extra assembly
products. The source remains authoritative for all regenerated geometry.

The common claim serializer retains the original filled bounds and format;
the bore path explicitly allows at most 16 walls per owner / 32 total.
Envelopes are limited to 16 MiB and source to 1 MiB with caller source limits,
strict JSON fields, cancellation and all-or-none returns. Corrupt local
native solids, raw support weights, swapped wall roles, matrices, relationship
IDs, units and format tags fail reproof. Filled and bore envelopes reject
each other. The live XCAF chain now includes bore envelope reload before
source export and generated STEP output.
