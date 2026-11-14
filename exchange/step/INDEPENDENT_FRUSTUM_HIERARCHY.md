# Original rational frustum in a mixed STEP hierarchy

`ReadIndependentFrustumHierarchy`, `WriteIndependentFrustumHierarchy`, and
`WriteIndependentFrustumHierarchySource` add one exact original capped rational
frustum to the existing signed-coordinate planar/cylinder/torus hierarchy, with
an optional sphere. The public model is separate; existing hierarchy entrypoints
and native envelope tags remain unchanged and reject this family.

The frustum is authenticated by `ReadWrappedFrusta`: original rational support,
whole model/pcurve agreement, seam and rim incidence, planar caps, positive
endpoint radii and constructive material embedding. No `CONICAL_SURFACE` angle
is converted to a presumed exact slope. STEP finite-decimal radians generally
cannot encode an exact specified rational taper angle. Rational wrapped profiles
are a separate exact geometric family, including noncircular positive profiles.

Original local sources stay independently owned. Product and NAUO identities,
shared local assets, exact signed-coordinate transforms and complete expanded
paths are retained. All geometry must be reachable and exclusively owned.
Existing depth-eight, 64-expanded-use, 16-product and source/entity/material
limits apply. The new mode permits at most seven complex subtype records to
accommodate rational spline entities; old modes retain the four-record bound.
The bounded frustum proof has a fixed million-operation, 256-pole, 4096-bit cap;
cancellation is observed immediately before and after that legacy local proof.

The independent retained OCCT `wrapped-circular-frustum.step` is staged alongside
four independently qualified original local products. Both source and emitted
nine-occurrence mixed assembly pass pinned OCCT TransferRoots and adaptive
surface/volume integration. The reconstructed original local native solid also
survives strict native reload. The assembly source can be re-exported after full
reproof; the separate native envelope is described below. Rational-frame query modes
remain unchanged.

The old torus comparator integrates nonanalytic lateral surfaces at mid-height;
that is inadequate for tapered surfaces. The separate `step_frustum_hierarchy`
comparator uses OCCT adaptive surface and volume integration at 1e-12 instead.
Pin: OCCT 4dbfdcc448c0b3e565488afcd626c84483172db9.

    PLAINSOLID_OCCT_FRUSTUM_HIERARCHY=/tmp/plainsolid-frustum-hierarchy-oracle go test -race ./exchange/step -run '^TestIndependentFrustumHierarchy$' -count=1

## Source-bound native persistence

`WriteIndependentFrustumHierarchyJSON` and `ReadIndependentFrustumHierarchyJSON`
use the distinct version-one `plainsolid-step-independent-frustum-hierarchy`
tag and a separate claims tag. Each local native asset is stored once, alongside
source product/representation identities, original rim and corner IDs, original
support canonical evidence and expanded occurrence paths. The reader performs
one complete strict STEP reconstruction, regenerates all native bytes and
claims, and rejects every mismatch; stored native data never becomes an
independent geometry authority. Re-export retains source entity IDs and values,
with canonical STEP whitespace. Older envelope tags remain unchanged.

The envelope is capped at 16 MiB (caller may reduce it), 64 instances, eight path
edges, and source/material limits. Frustum evidence scalar size is capped at
4096 bits. Cancellation and failure produce no partial model. Native output is
staged before writing; ordinary I/O failure can still produce a short write.

Bounded mutation campaign: `go test ./exchange/step -run '^$' -fuzz
'^FuzzIndependentFrustumHierarchyNative$' -fuzztime=40x -parallel=2` completed
40 executions with valid native and malformed-envelope seeds. Input is capped
at 1 MiB in the fuzz target; parser and material quotas remain active.
