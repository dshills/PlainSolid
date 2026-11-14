# Original OCCT depth-three SHUO appearance

`ReadOCCTDeepSHUOAssembly` is a separate explicitly scoped reader for the genuine
prefix-anchored SHUO convention emitted by the pinned OCCT writer. It admits
three levels of ordinary product uses, with a repeated middle product containing
a repeated group product and individually colored single-body planar leaves.
It is not an arbitrary recursive SHUO or style-inheritance parser.

## Independently observed source convention

The original `occt-ap242-deep-shuo-colors.step` fixture (830 entities) is authored
by `tools/occtref/author_ap242_deep_shuo.cxx` using `SetSHUO` with three original
component labels. OCCT commit is
`4dbfdcc448c0b3e565488afcd626c84483172db9`; only trailing whitespace is removed
from retained text. The source graph is:

- prefix SHUO801: root definition5 to group58, upper NAUO54, next NAUO85;
- terminal SHUO800: root definition5 to leaf89, upper SHUO801, next NAUO440;
- PDS803 and SDR802 anchor the style on **prefix801**, while context804 contains
  the terminal leaf's placement68 and the style targets body95.

This prefix anchor is confirmed in pinned `STEPCAFControl_Writer::writeSHUO`,
and is preserved, not rewritten into a claimed standard terminal-PDS encoding.
The reader name and native tag explicitly identify this observed convention.

## Bounded proof

Exactly two SHUO records and one contextual style are admitted. The terminal
must be the unique continuation of the prefix: no other SHUO, borrower,
competing continuation, cycle or terminal PDS is accepted. The prefix is owned
by exactly its terminal continuation and one PDS; that PDS has exactly one SDR.
All metadata attribute types and all references are checked.

The three ordinary NAUOs must connect four distinct product definitions. Prefix
root/related definitions must agree with the first and third definitions;
terminal root/related definitions must agree with the first and fourth.
The original terminal body, placement, geometry context, default predecessor,
fill style, next-use PDS/CDSR/IDT and owning face geometry are independently
checked through the existing contextual chain proof. A private proof view
retargets the style SDR to that already identified terminal-use PDS, solely to
reuse the chain verifier. The returned evidence and exported source retain the
original prefix-PDS and both SHUO IDs.

The entire product graph is independently checked: one root, at least two uses
of one geometry-free middle product, at least two middle uses of one
geometry-free group, and one-body planar group children with explicit whole-body
defaults. No other parent class, unreachable product, extra body, direct/shared
contextual style or face-specific override is admitted. Each expanded path
retains three NAUOs, four original product definitions and its body ID.

Only the fully matched three-link path receives the override. Each other use
retains its local product default. Original neutral-child/proper signed parent
frames are checked from source decimals; composed translations must remain
exactly represented (rational composition rejects rounding). The existing
planar geometry and unit contract remains unchanged.

The limit is 64 expanded leaves, 16 products/bodies, 4096 source entities and
1 MiB source, with existing tighter source/value/copy limits. This is a fixed
three-level grammar, not an unbounded recursive traversal. Cancellation is
observed around bounded source proof and during expansion/copy; it does not
interrupt the legacy parser.

## Persistence and independent validation

The result exposes copied `Paths()` and `Override()` and fresh re-proved local
geometry via `NativeAssembly`. The original source writer re-proves the source.
The version-one `plainsolid-step-occt-depth-three-shuo` native envelope preserves
source, local native boundaries, product/context/SHUO ancestry and full paths,
then compares every claim against one source reproof on reload. Stored native
geometry never substitutes for source validation. Envelope limit is 8 MiB;
prior formats and narrower readers remain strict and unchanged.

Pinned live transfer explicitly enables SHUO mode and observes color with the
original XCAF component-label sequence. Twelve depth-three leaf uses yield
assembly occurrence sums V216/A504. Only path[54,85,440], translated to
(105,26,7), is blue. Its same-outer sibling at (95,26,7) and other-outer
counterpart at (7,105,26) remain red. Separate valid source mutations retarget
middle85 to794 and outer54 to799; the live observer confirms only the respective
sibling/counterpart becomes blue. All 12 rotations, translations, colors and
component-color flags are compared, alongside BRepCheck and V/A, for retained
source, native reload/re-export, and freshly independently authored source.
These quantities are assembly sums, not union geometry.

```sh
sh tools/occtref/build_author_ap242_deep_shuo.sh \
 /tmp/plainsolid-occt-build/include/opencascade \
 /tmp/plainsolid-occt-build/mac64/clang/lib /tmp/plainsolid-ap242-deep-shuo
PLAINSOLID_OCCT_AP242_DEEP_SHUO=/tmp/plainsolid-ap242-deep-shuo \
 go test ./exchange/step -run '^TestOCCTDeepSHUO' -count=1
```

Mutation tests cover wrong related definitions, reversed links, recursion,
competing continuations, terminal anchors, wrong body/context placement,
forged native paths/topology, old tags, cancellation, byte/entity limits and
transactional failure. Source/native fuzz smoke tests use 40 executions each,
64 KiB/256 KiB input limits and 1024 source entities.
