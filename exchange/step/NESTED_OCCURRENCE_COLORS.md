# Nested AP242 whole-body occurrence appearance

`ReadNestedOccurrenceColoredAssembly` is an additive, source-authenticated
planar assembly reader. The supported grammar is one geometry-free root with
at least two uses of one geometry-free intermediate group. Group uses reference
single-body planar local products. Each leaf has an explicit whole-body default
fill; an internal use may carry the independently observed ordinary
`OVER_RIDING_STYLED_ITEM` / `PRESENTATION_STYLE_BY_CONTEXT` chain.

This is **shared internal-use appearance**: selecting that NAUO affects every
incoming path to its parent group. It is not a path-specific SHUO override,
general style inheritance, group override, per-face override, or arbitrary DAG.
The existing flat reader, native format, and edit APIs retain their scope.

Every override is checked against its original base body style, context
representation, SDR, PDS, NAUO, CDSR, transformation and placement. Equal duplicate
claims emitted by OCCT retain both record identities; inconsistent duplicates
fail. Each expanded path retains `[outer NAUO, inner NAUO]`, root/group/product/body
IDs, effective color, override status and composed pose. No numeric record order
is used to identify geometry or style ownership. Output order follows the source
occurrence order. The two levels and all products must be reachable, with no
extra branch, cycle, additional local body, or borrowed style target.

Original transformation frames must have neutral child placement and exactly
represented proper signed-coordinate parent axes/origins. Composition uses
rational arithmetic and rejects any translated result not represented exactly
in binary64. This does not enable arbitrary rational rotations, mixed-context
unit conversion, or alternate nonneutral child-frame conventions. Local planar
geometry and product metadata retain the existing `ReadColoredAssembly` proof
and unit contract; there is no new general geometry acceptance claim.

The result's containers are private. `Paths` and `Overrides` return value copies;
`NativeAssembly` returns fresh source-reproved native local bodies and occurrence
metadata. `WriteNestedOccurrenceColoredAssemblySource` re-proves source and emits
it semantically unchanged; it does not synthesize edited derived geometry.

## Persistence and bounds

`Write/ReadNestedOccurrenceColoredAssemblyJSON` use the distinct version-one
`plainsolid-step-nested-contextual-appearance` format. Original source, local
native topology, product/face IDs, placements, default/contextual color claims,
and expanded paths are re-proved and compared at reload. Supplied native data
is never an alternative reconstruction authority. The old flat format rejects
this tag and this reader rejects the old tag.

Limits: 1 MiB source, 4096 entities, 16 products/bodies, 64 contextual claims,
64 expanded leaf paths, and 8 MiB native envelope, with the existing stricter
source/copy limits also enforced. Smaller caller limits apply. Cancellation is
checked before/after the bounded legacy source proof, during expansion/copying,
and before staged output; it is not an interruptible parser. Rejection returns
no partial model, and validation completes before writing.

## Independent evidence

`testdata/occt-ap242-nested-occurrence-colors.step` is the original AP242 export
from `tools/occtref/author_ap242_nested_occurrence.cxx`, OCCT commit
`4dbfdcc448c0b3e565488afcd626c84483172db9`. Only line-end whitespace was removed.
The author creates two unlike boxes, three leaf uses in one group, a blue
component override on the second shared red-box use, and two placed group uses.
It does not call the Go writer or use its numeric/entity templates.

The live observer uses STEPCAF transfer, BRepCheck, standard mass/area, original
XCAF component/default colors, recursive references and composed locations.
Six depth-two leaf uses have volume sum 108 and area sum 252. Blue is present at
(100,20,0) and (0,100,20); siblings retain their red/green product defaults.
Tests compare all six translations, all rotation coefficients, component-color
presence and RGB values for retained source, native-reloaded source re-export,
and a freshly authored original file. These are assembly occurrence sums, not
regularized union measures.

Build and run:

```sh
sh tools/occtref/build_author_ap242_nested_occurrence.sh \
 /tmp/plainsolid-occt-build/include/opencascade \
 /tmp/plainsolid-occt-build/mac64/clang/lib \
 /tmp/plainsolid-ap242-nested-occurrence
PLAINSOLID_OCCT_AP242_NESTED_OCCURRENCE=/tmp/plainsolid-ap242-nested-occurrence \
 go test ./exchange/step -run '^TestNestedOccurrenceColors' -count=1
```

Mutations cover incorrect contextual target/placement, wrong product body,
conflicting duplicates, cyclic/wrong product references, native provenance,
geometry, source, paths and tags. The focused fuzz target bounds input to 64 KiB
and 1024 entities; a reproducible smoke campaign uses
`-run '^$' -fuzz '^FuzzNestedOccurrenceColors$' -fuzztime=40x -parallel=1`.
