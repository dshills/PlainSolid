# Disjoint ordinary and SHUO contextual appearance

`ReadCombinedOccurrenceColoredAssembly` is a separate bounded AP242 reader for
one path-specific SHUO plus one or more ordinary whole-body contextual claims
on **different inner NAUOs** in the already proved repeated-group grammar.
The shared override applies under every incoming group use. The SHUO applies
only under its explicit outer/inner pair. Overlapping targets fail even when
their colors coincide: this API does not guess precedence or inheritance.
The earlier flat, shared-only and SHUO-only APIs/tags keep their scope.

The original fixture is produced independently by
`tools/occtref/author_ap242_combined_context.cxx` using XCAF component color and
`SetSHUO`, pinned OCCT `4dbfdcc448c0b3e565488afcd626c84483172db9`.
It has 809 entities; retained text only removes trailing whitespace. The same
red local box is shared by two distinct inner uses. One inner use has a cyan
ordinary override; a different inner use has one blue SHUO path. The six leaf
colors are cyan, blue, green, cyan, red, green. Assembly occurrence sums remain
V108/A252. This is not regularized union geometry.

Each family retains its own original contextual evidence. The SHUO root/leaf
and both usage links are independently checked. Ordinary claims are checked
against their own PDS/CDSR/IDT and target body. All source geometry, units,
product defaults and proper signed-coordinate poses retain the prior strict
proof. After independently proving both selectors, only the corresponding
expanded leaf paths are assigned colors. No face-specific, group, recursive
SHUO, deeper DAG or overlapping style interpretation is admitted.

## Original unused context

OCCT emits an additional unstyled `SHAPE_DEFINITION_REPRESENTATION` and
`SHAPE_REPRESENTATION` for the ordinary component, beside its styled context.
The generic assembly reader rejects this unused context. The new private proof
view removes it only after proving:

- its representation is exclusively owned by that SDR;
- the SDR has no incoming reference and refers to an ordinary NAUO PDS;
- exactly one styled context has the same PDS and **identical full representation
  record**, including original placement and geometry-context references.

The styled counterpart still passes the complete contextual chain proof.
Different placement, context, name, PDS, extra owner, or ambiguous counterpart
is rejected. `UnusedContexts()` returns original SDR/representation/PDS and
counterpart IDs. Original source is retained and re-emitted, including these
records. This is an authenticated redundant context, not an ignored unknown
entity or geometry repair.

## Native and external qualification

The immutable result exposes copied effective paths and separate
`SharedOverrides`, `PathOverrides`, and `UnusedContexts` evidence.
`NativeAssembly` returns fresh source-reproved local geometry.
`WriteCombinedOccurrenceColoredAssemblySource` re-proves original source.
`Write/ReadCombinedOccurrenceColoredAssemblyJSON` use the distinct version-one
`plainsolid-step-combined-contextual-appearance` tag, and compare all source,
geometry, appearance, usage, redundant-context and expanded-path claims after
one complete source proof. Prior tags are unchanged. Bounded source/native
limits and cancellation behavior are inherited from the documented nested and
SHUO modes (1 MiB source, 8 MiB envelope, 4096 entities, 16 products,
64 expanded paths); at most 64 redundant contexts are admitted.

Live tests load retained source, native-reloaded source export, freshly authored
source, and a valid SHUO outer-use retarget in OCCT. They compare all six full
poses, component-color presence, RGB, BRepCheck and V/A. The retarget changes
which counterpart is blue and preserves both ordinary cyan paths. Negative
checks cover wrong body and path links, overlapping selectors, borrowed or
mismatched redundant contexts, forged native ancestry/geometry/color, limits,
cancellation and transactional rejection.

```sh
sh tools/occtref/build_author_ap242_combined_context.sh \
 /tmp/plainsolid-occt-build/include/opencascade \
 /tmp/plainsolid-occt-build/mac64/clang/lib /tmp/plainsolid-ap242-combined-context
PLAINSOLID_OCCT_AP242_COMBINED_CONTEXT=/tmp/plainsolid-ap242-combined-context \
 go test ./exchange/step -run '^TestCombinedOccurrenceColors' -count=1
```

## Face-specific authoring probe (not claimed supported)

The initially investigated public XCAF route could not author an
occurrence-local face: `AddSubShape(component, locatedFace)` returns null and
`SetInstanceColor(locatedFace, ..., true)` returns false. The pinned source
requires a top-level simple shape for AddSubShape, while FindComponent searches
for a referenced top-level shape. A product-local face color would affect all
uses and is not evidence of an occurrence-specific face style. This observation
does not claim STEP itself cannot encode that style.

The minimal independent probe is reproducible:

```sh
sh tools/occtref/build_probe_ap242_face_occurrence.sh \
 /tmp/plainsolid-occt-build/include/opencascade \
 /tmp/plainsolid-occt-build/mac64/clang/lib /tmp/plainsolid-ap242-face-probe
/tmp/plainsolid-ap242-face-probe
# {"componentFaceLabel":false,"instanceFaceColor":false}
```
