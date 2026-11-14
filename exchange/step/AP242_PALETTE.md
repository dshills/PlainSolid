# AP242 product-local named surface colors

The existing colored planar, spline and wrapped-product readers now recognize
`DRAUGHTING_PRE_DEFINED_COLOUR` in their authenticated surface-fill style chain.
Its eight case-sensitive names map to exact RGB corner values: black, white,
red, green, blue, yellow, magenta and cyan. Unknown names, other predefined-color
subtypes, unused style records and malformed targets are rejected. This adds no
transparency, texture, layer, occurrence override or arbitrary AP242 support.

All existing body/face ownership, representation-context and override checks
remain in force. A product-local style follows every occurrence of that shared
product; it is not a per-occurrence style. Writers continue emitting `COLOUR_RGB`.
Thus generated round trips preserve color semantics, names, local geometry and
placements, but do not preserve named-color lexical encoding or an AP242 header
(the existing assembly writer emits its existing AP214 envelope). The original
parsed source still contains its original named-color entities and IDs.

`occt-ap242-palette-assembly.step` is authored directly by pinned OCCT XCAF,
not converted from a Go-authored file. It contains two independent box products,
red and blue, three occurrences (including a rotated repeated red product), and
an actual AP242 schema declaration. Only line-end whitespace was normalized.
The independently transferred aggregate has volume 54 and surface area 126 in
source coordinate units; XCAF reads exactly one red and one blue local style
from both original input and generated output.

Reproduce against OCCT `4dbfdcc448c0b3e565488afcd626c84483172db9`:

```sh
sh tools/occtref/build_author_ap242_palette.sh \
 /tmp/plainsolid-occt-build/include/opencascade \
 /tmp/plainsolid-occt-build/mac64/clang/lib /tmp/plainsolid-ap242-palette
/tmp/plainsolid-ap242-palette write /tmp/ap242-palette.step
/tmp/plainsolid-ap242-palette read /tmp/ap242-palette.step
PLAINSOLID_OCCT_AP242_PALETTE=/tmp/plainsolid-ap242-palette \
 go test ./exchange/step -run 'TestAP242Palette|TestDraughtingPalette' -count=1
```

Negative tests retain rejection of `PRESENTATION_STYLE_BY_CONTEXT`, cross-product
style targeting, detached colors, unknown/case-altered names, and byte limits.
Preflight recognizes the entity structurally; it does not certify a style graph.
