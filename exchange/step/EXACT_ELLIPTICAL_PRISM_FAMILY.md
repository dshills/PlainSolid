# Exact elliptical prism family pipeline

`ReadExactEllipticalPrismFamily` is an explicit material dispatcher, separate
from generic STEP, repair APIs and standalone surface readers. It parses the
input exactly once, bounded to 64 KiB, 118 entities, 4096 values and depth 16.
Numeric/frame inspection only chooses a candidate; the corresponding complete
owning graph proof and native exact material construction still run. Fixed
internal reference templates may be parsed separately; input bytes are never
reparsed per candidate. Existing typed entry points retain their restrictions.

The tagged result contains exactly one payload:

| Variant | Original typed domain |
| --- | --- |
| cardinal | ModelOnlyEllipticalPrism |
| pythagorean | PythagoreanEllipticalPrism |
| exact-control | ExactControlEllipticalPrism |
| tilted | TiltedEllipticalPrism |
| homothetic-annular | EllipticalAnnularPrism |
| nested-annular | NestedEllipticalAnnularPrism |
| orthogonal-annular | OrthogonalEllipticalAnnularPrism |

Common name, units, source graph and native solid are provided directly.
Original source body/shell/edge/face ancestry stays in the typed payload;
partitioned native topology is available from the immutable Solid. Native JSON
round-trips use the existing trimmed codec. This does not create a new native
container or allow stale recipe fields to override source geometry.

Lossless Pythagorean and rational-control capabilities overlap geometrically.
The dispatcher deterministically prefers the lossless Pythagorean proof. Only
its explicit ErrUnrepresentable permits trying the exact-control proof;
malformed, cancelled or unsupported geometry is never rescued by a broader
fallback. All other domains are selected by exact frame relationships and
semiaxis constraints. Source order and entity IDs do not decide the variant.

`WriteExactEllipticalPrismFamilySource` re-proves and emits a retained source
File atomically. It preserves source IDs, names and units, and writes no edits
to decoded fields. Use each typed definition writer for authored geometry.
Orphan entities, extra material graphs, unknown roots, rounded vendor pcurves,
invalid ownership and unsupported unit/frame records remain errors. No repair
API or tolerance promotion is reachable through family dispatch.

Tests cover all seven variants (six independently authored retained OCCT
fixtures plus the previously qualified exact cardinal template), ordinary
native reload, retained source roundtrip, unique tagging, resource limits and
malformed/ambiguous graph rejection. Previous fixture-specific live OCCT
qualification remains applicable: the dispatcher invokes the same proof and
material builders. Focused race, Go 1.23, vet and 100 bounded fuzz executions
also pass. Fuzz seeds include every registered variant and malformed input.
