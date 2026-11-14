# Explicit planar occurrence surface colors

`ReadOccurrenceColoredAssembly` is an additive reader for the genuine XCAF
contextual whole-body override graph in a flat planar assembly. Existing
`ReadColoredAssembly` remains product-local and rejects this source. No existing
writer or persistence format changes meaning.

For each `OVER_RIDING_STYLED_ITEM`, the reader proves:

* its target is the single local body of the selected child product;
* its parent is the authenticated product-local `STYLED_ITEM` on that body;
* the fill is one `SURFACE_STYLE_USAGE(.BOTH.)` with RGB or a standard named color;
* `PRESENTATION_STYLE_BY_CONTEXT` points to a single-placement shape representation
  in the body's original representation context;
* its `SHAPE_DEFINITION_REPRESENTATION` identifies the original occurrence PDS;
* that PDS's unique CDSR identifies the actual NAUO and IDT, whose target placement
  is exactly the contextual representation's placement record.

The existing planar assembly decoder then re-proves the product graph, original
geometry, units and transforms in a private view without the authenticated
contextual additions. Original source records remain separate and unchanged.
Unknown/detached styles, borrowed structural contexts, inconsistent targets,
face-specific overrides, nested occurrence ambiguity and conflicting duplicate
claims fail atomically. Existing base face colors on an overridden product are
excluded from this first subset. Other legacy product-local style rules remain
unchanged. At least one contextual claim is required.

The independently authored OCCT file emits two equivalent claims for one use.
Both original claim chains remain in `Overrides`; equal duplicates are allowed,
conflicting duplicates are rejected. Consumers apply their common color to that
NAUO only, retaining product-local defaults for other uses. All source IDs are
retained, including both style records, context representations, SDRs, PDS,
CDSR, transformation and target placement. The result is descriptive data;
mutating it is not an authenticated style edit.

`WriteOccurrenceColoredAssemblySource` takes the original `File`, re-proves the
complete supported graph and stages validation before writing its canonical
STEP encoding. It is source re-export, not generation from edited result fields.
An invalid source produces no output (apart from possible ordinary I/O failure).
The original AP242 schema and record IDs survive this route.

Limits are capped at 1 MiB, 4096 entities, 65536 values, depth 16, 16 products,
4096 geometry edge uses and 64 contextual claims. Caller limits may be lower.

## Independent qualification

`occt-ap242-occurrence-colors.step` is authored by XCAF on pinned OCCT
`4dbfdcc448c0b3e565488afcd626c84483172db9`, with only trailing line whitespace
removed. Two products have local red and green fills. The red box appears twice;
only its rotated use at translation `(0,20,0)` has a blue contextual override.
Ordinary XCAF transfer independently observes effective colors red/blue/green,
volume 54 and surface area 126. The untouched first use remains red. A separately
retargeted standard graph with both SDR and placement links changed proves blue
moves to the first occurrence rather than following a hardcoded record ordinal.
Both original variants and their source re-exports are checked live.

```sh
sh tools/occtref/build_author_ap242_occurrence.sh \
 /tmp/plainsolid-occt-build/include/opencascade \
 /tmp/plainsolid-occt-build/mac64/clang/lib /tmp/plainsolid-ap242-occurrence
/tmp/plainsolid-ap242-occurrence write /tmp/ap242-occurrence.step
PLAINSOLID_OCCT_AP242_OCCURRENCE=/tmp/plainsolid-ap242-occurrence \
 go test -race ./exchange/step -run TestOccurrenceColor -count=1
go test ./exchange/step -run '^$' -fuzz '^FuzzOccurrenceColoredAssembly$' \
 -fuzztime=40x -parallel=1
```

This is not general AP242 presentation, SHUO, hierarchy-path styling,
transparency, or per-face instance styling support.
