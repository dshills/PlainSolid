# Source-bound contextual appearance persistence

`WriteOccurrenceColoredAssemblyJSON` and `ReadOccurrenceColoredAssemblyJSON`
use the distinct version-one tag `plainsolid-step-flat-contextual-appearance`.
They preserve the authenticated flat planar occurrence-color subset described
in `OCCURRENCE_COLORS.md`; no previous format or product-local API changes.

The envelope contains canonical original STEP, units, product definitions,
root and NAUO occurrence IDs/transforms, each original contextual style chain,
and deterministic local native boundary claims. A native boundary consists of
its original `brep.Solid` vertices, shared edges and oriented face loops, plus
source body/face IDs, name and product-local styles. It is not a tessellation or
a rounded replacement for the source model. Shared products are stored once;
occurrences continue referencing their original product definitions.

Reload invokes `ReadOccurrenceColoredAssembly` once, reconstructing all native
planar bodies and appearance from the original source. It then compares every
stored claim with freshly generated evidence. It never constructs geometry from
untrusted stored boundary claims. The writer performs the same reproof and
comparison against the supplied result before emitting any bytes. Altering a
local vertex, face use, source face ID, unit, placement, override color or source
association therefore cannot silently become an accepted edit.

Unknown fields, wrong versions/tags and trailing JSON values fail. The envelope
is capped at 8 MiB independently of the 1 MiB source limit; caller source limits
are honored. Native claims have additional cumulative body/vertex/edge/face
bounds. Cancellation is checked before/after input and around source proof;
the bounded legacy STEP decoder itself is not internally interruptible. Output
is staged through validation; an ordinary I/O failure may still partially write.

Tests verify canonical byte stability, original native boundary connectivity,
source ancestry, forged source/native claims, canceled and undersized reads,
invalid writer atomicity, short writes, old-tag rejection and bounded fuzzing.
Independent XCAF reads STEP re-exported after native reload with the same
red/blue/green occurrence colors, volume 54 and area 126 as the original source.

```sh
PLAINSOLID_OCCT_AP242_OCCURRENCE=/tmp/plainsolid-ap242-occurrence \
 go test -race ./exchange/step -run TestOccurrenceColorsNative -count=1
go test ./exchange/step -run '^$' -fuzz '^FuzzOccurrenceColorsNative$' \
 -fuzztime=40x -parallel=1
```

The oracle build and OCCT pin are documented in `OCCURRENCE_COLORS.md`. Native
reload retains original source re-export; this format does not authorize edited
appearance or broaden contextual styling to nested assemblies or face overrides.
