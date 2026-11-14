# Controlled contextual occurrence color edits

`EditOccurrenceSurfaceColor(ctx, source, nauoID, color, limits)` edits one
**existing** whole-body override in the authenticated flat planar graph. The
selector is the original `NEXT_ASSEMBLY_USAGE_OCCURRENCE` entity ID, not a product
ID, positional index or body ID. Unstyled uses are deliberately unsupported;
this is source-preserving editing, not a new arbitrary assembly/style author.

The function revalidates the original source, updates every equivalent duplicate
claim for the selected use, then revalidates the complete edited source. It
compares regenerated native geometry, source body/face roles, products, units,
occurrence names/poses and base styles, permitting only the requested contextual
color differences. The input `File` is never mutated. An unchanged color returns
a newly authenticated result with no source record changes.

Exclusive fill records retain their original IDs. If a selected fill record is
also referenced outside the selected override chains, that record and its
selected dependent chain are copied to fresh IDs before editing. Original shared
records remain unchanged. Thus editing an instance cannot accidentally recolor
its product default or another use. The original contextual assignment, styled
item, placement and NAUO IDs stay stable. All unrelated entity records remain
identical; tests compare them directly. New-ID overflow and caller entity limits
fail atomically.

Use the existing validated source exporter on the returned result's
`Assembly.Assembly.Source`, or persist it with
`WriteOccurrenceColoredAssemblyJSON`. Reload re-proves the edited source and its
native claims normally. No new native tag is needed: the existing version-one
format already represents this same authenticated graph.

Qualification uses the pinned AP242 XCAF source from `OCCURRENCE_COLORS.md`,
plus standard source variants sharing just a color or a complete fill chain with
the product default. Original/reloaded edited source transfers independently
through XCAF with only the selected occurrence changed to sRGB `(0.25,0.5,0.75)`;
the other occurrences stay red and green. Volume 54 and area 126 are unchanged.
The oracle now reports explicit sRGB components (rather than OCCT's internal
linear RGB), so non-primary color comparisons have the intended semantics.

```sh
sh tools/occtref/build_author_ap242_occurrence.sh \
 /tmp/plainsolid-occt-build/include/opencascade \
 /tmp/plainsolid-occt-build/mac64/clang/lib /tmp/plainsolid-ap242-occurrence
PLAINSOLID_OCCT_AP242_OCCURRENCE=/tmp/plainsolid-ap242-occurrence \
 go test -race ./exchange/step -run 'TestEditOccurrence|TestOccurrenceColor' -count=1
go test ./exchange/step -run '^$' -fuzz '^FuzzEditOccurrenceColor$' \
 -fuzztime=40x -parallel=1
```

The API preserves the original scoped reader's geometry and placement rules. It
does not add nested path styling, new overrides, per-face instance appearance,
material definitions, transparency or implicit color-space conversion in Go.
