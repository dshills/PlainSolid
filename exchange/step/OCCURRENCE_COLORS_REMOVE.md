# Removing a selected contextual override

`RemoveOccurrenceSurfaceColor(ctx, source, nauoID, limits)` removes every
original contextual claim for one authenticated flat NAUO. A known use without
an override is a validated source no-op; a product/body/unknown ID is rejected.
The input file is never modified. The result reports both `Removed` original
claims and the freshly proved `Appearance`.

Only the selected overriding items, contextual assignments, context shape
representations and their SDRs are removed. Their fill dependencies are removed
only when no retained record references them. Shared colors and complete fill
chains survive, as do other occurrence overrides and the original base styled
item. The occurrence's PDS, CDSR, transformation and actual geometry are never
removed. Presentation lists lose only the selected item references.

The complete result is decoded again, and native boundary, product defaults,
units, placements and remaining source-role claims are compared against the
original model after precisely deleting the selected override claims. A failed
proof returns no partial result.

When `Appearance.Overrides` is nonempty, the source still belongs to the strict
contextual reader/native format. When it is empty, the source belongs to
`ReadColoredAssembly`: the product-local default now supplies the appearance.
The existing contextual-only native tag deliberately rejects that empty state;
this milestone does not change its schema meaning or invent a default-only
native envelope.

`WriteFlatColoredAssemblySource` validates and exports either of these flat
source classes using structural syntax selection, never fallback after a failed
proof. It takes the authoritative `File`, not mutable result claims. Validation
is staged before output, subject to ordinary I/O failures. Source and cancellation
limits remain those of the bounded appearance lane.

## Independent qualification

Pinned XCAF reads the original AP242 source with the selected override removed
as red/red/green occurrences instead of red/blue/green. With a separately authored
yellow override on the other use, removal leaves yellow/red/green. Both preserve
volume 54 and area 126. An independently transferred source with a complete fill
chain shared with the product default also restores the default correctly.
Remaining contextual appearance survives native reload before STEP export.

Create-then-remove tests restore the exact original entity graph for both the
product-local and contextual fixtures. Further tests cover shared palette
preservation, no-op, wrong IDs, malformed ownership, byte/cancellation limits,
atomic invalid export, short writes and strict old native-tag rejection.

```sh
PLAINSOLID_OCCT_AP242_OCCURRENCE=/tmp/plainsolid-ap242-occurrence \
 go test -race ./exchange/step -run 'TestRemoveOccurrence|TestSetOccurrence|TestOccurrenceColor' -count=1
go test ./exchange/step -run '^$' -fuzz '^FuzzRemoveOccurrenceColor$' \
 -fuzztime=40x -parallel=1
```

The oracle build and immutable OCCT pin remain in `OCCURRENCE_COLORS.md`.
