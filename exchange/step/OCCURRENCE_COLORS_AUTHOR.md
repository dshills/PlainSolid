# Creating a selected whole-body occurrence override

`SetOccurrenceSurfaceColor(ctx, source, nauoID, color, limits)` creates a standard
contextual override on an authenticated flat planar occurrence. It accepts either
an existing product-local colored assembly or the qualified contextual source.
Presence of a contextual assignment selects its specific strict reader; there
is no speculative decoder fallback.

Creation requires the actual NAUO, one local body on its child, an authenticated
whole-body product default, and no face-color overrides on that child. The author
never invents a base style, geometry, transform, product or occurrence. Nested
assemblies and invalid/ambiguous targets remain unsupported.

Ten fresh standard records describe the new context representation, SDR,
overriding styled item, contextual assignment and fill/color chain. The SDR uses
the occurrence's existing PDS, while the context representation uses the actual
CDSR/IDT target placement and the local body's representation context. The base
styled item and body remain the authenticated originals. The only changed old
record is the presentation's item list, with the new styled item appended.
Every other original entity, geometry and placement remains unchanged.

The generated file must pass `ReadOccurrenceColoredAssembly`. Regenerated native
boundary, product/default-style, placement and source-role claims must match the
original model plus precisely the authored new override. An error returns no
partial result and never mutates the supplied file. Source, allocation and
cancellation limits follow the existing contextual reader; ten-record expansion
and entity-ID overflow are checked before acceptance.

If the target already has an override, the same API uses the validated edit path,
including shared-fill copying and exact source no-op for an unchanged color.
Creating an explicit override equal to a product default is still authoring a
new override: it intentionally fixes that use's style independently of future
product-default edits. A repeated call then becomes a source no-op.

The returned source can be exported using `WriteOccurrenceColoredAssemblySource`
and persisted under the unchanged contextual-appearance native format. New
source is revalidated on both routes. No ordinary assembly writer changes scope.

## Qualification

Tests start from both genuine pinned OCCT AP242 fixtures: the product-local
palette assembly and the already-contextual assembly. Creating a style on the
first repeated use leaves the second use's default or existing override intact.
Native reload and independent XCAF STEP transfer preserve effective sRGB
`(0.25,0.5,0.75)` only on the selected use, with aggregate volume 54 and area 126.
Tests also check append-only presentation changes, exact unrelated record
preservation, source/native re-read, no-op, wrong targets, absent default,
face-style ambiguity, cancellation and bounded failures.

```sh
PLAINSOLID_OCCT_AP242_OCCURRENCE=/tmp/plainsolid-ap242-occurrence \
 go test -race ./exchange/step -run 'TestSetOccurrence|TestEditOccurrence|TestOccurrenceColor' -count=1
go test ./exchange/step -run '^$' -fuzz '^FuzzSetOccurrenceColor$' \
 -fuzztime=40x -parallel=1
```

The standalone oracle build and immutable OCCT pin are in
`OCCURRENCE_COLORS.md`. The author creates scoped appearance on an existing
proved graph; it is not a generic AP242 assembly or material author.
