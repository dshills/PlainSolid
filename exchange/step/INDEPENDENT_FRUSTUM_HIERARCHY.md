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

## Exact signed-pose world bounds

`BoundIndependentFrustumHierarchy` re-proves source, native local assets and
occurrence paths before returning exact rational AABBs for all five families.
For the tapered rational profile, positive weights keep each quadrant in its
control square, while the cardinal endpoints attain its axis extrema. The
common positive endpoint radius varies linearly with height, so its maximum is
attained at a cap. This proof applies to the accepted noncircular profiles too.
Signed-coordinate rigid occurrences preserve these extrema exactly. All axis
padding is zero; no rounded vertex baking or sampled extremum is used.

Returned leaf records identify the original product, body, source faces and
complete occurrence path. Their boxes describe whole bodies, not individual
trimmed faces. Rational coordinate accessors copy their values. Separate source,
point-evaluation and coefficient budgets fail atomically. Independent OCCT world
bounds match the translated and rotated nine-occurrence fixture after native
reload; a noncircular original-support regression uses the same exact extrema.

## Exact circular-frustum mass

`MeasureIndependentCircularFrustumHierarchy` returns symbolic unit-density
volume, first/second moments, centroid and inertia for every local product,
every original occurrence, and their sum. This is assembly occurrence mass,
not regularized Boolean-union mass. A noncircular frustum returns an error with
no partial products, even though the geometry and bounds codecs support it.

The complete source/native/pose proof must certify the original profile's exact
circularity. With t in [0,1], radius rho=r0+(r1-r0)t and height h, disk section
integrals are πrho² for area and πrho⁴/4 for each radial second moment. Exact
rational polynomial integration gives volume h∫πrho², axial first moment
h²∫tπrho² and axial second moment h³∫t²πrho². The original center/lower station
and retained signed occurrence matrix are then applied exactly, including all
parallel-axis products. No decimal half-angle is converted into a slope.

The shared post-authentication moment accumulator is reused by old queries;
their source acceptance and results are unchanged. This new mode uses exact
signed original face-loop integration for planar members with a fixed 65536
planar-proof-test cap, plus caller triangle/coefficient/source limits. All
symbolic coefficient accessors copy their rational values. The pinned adaptive
OCCT mass comparator verifies original emitted STEP after native reload,
including off-diagonal and centroidal inertia. The canonical local circular
frustum test has V=35π/3, Fz=−5π/12, Qxx=Qyy=31π/4, Qzz=65π/3.

## Exact circular-frustum point location

`LocateIndependentCircularFrustumHierarchyPoint` classifies each retained body
separately, including all four previous families. It does not classify a
regularized union or merge coincident occurrence identities. Noncircular
frusta fail atomically in this explicit circular query.

For source-local coordinates x,y measured from the certified axis and z measured
from the lower cap, h²(x²+y²) is compared exactly with
(r0*h+(r1-r0)*z)², together with 0≤z≤h. No square root, angular parameter,
float tolerance or rounded profile is used. The original wall and cap face IDs
are recovered from their authenticated supports/stations, not list order.
Boundary results preserve both faces at each rim, original rim/seam edge IDs,
and original vertex IDs where a seam meets a rim. The positive local-X ray
identifies the canonical physical shared seam; no arbitrary pcurve branch is
invented. `ProductCoordinates` copies original local coordinates;
`AnalyticCoordinates` copies axis-centered coordinates relative to the lower cap.

Independent OCCT source classification covers inside/outside, wall, caps,
rims/seam/endpoints, including an exact nonbinary 7/5 seam coordinate and a
rotated occurrence after native reload. Additional exact ±2^-120 radial tests
are deliberately not compared to OCCT's tolerance classifier. Original source,
coefficient, planar proof/ray budgets and cancellation remain enforced.

## Exact circular-frustum line clipping

`ClipIndependentCircularFrustumHierarchyLine` retains the caller's original
parameter t in origin+t*direction. It clips every retained body independently
and coalesces only exactly equal hit parameters, preserving every occurrence
owner. It does not return a regularized Boolean-union interval.

The circular taper predicate becomes a rational quadratic on the line, clipped
first to the exact finite axial slab. The bounded polynomial solver handles
quadratic, linear and constant cases; an identically zero polynomial denotes a
whole wall generator. Interior open intervals and coplanar-cap/wall boundary
spans remain distinct. Tangency retains its exact multiplicity. Boundary hits
and coincident spans identify authenticated original supports; exact polynomial
signs at algebraic roots attach the original rim/seam/vertex identities without
rounding an irrational station or guessing a periodic pcurve branch.

Root isolation, exact comparison and ownership sign proofs share cumulative
operation/subdivision limits. Initial planar point-proof work also counts
against the line's planar/ray quotas. Cancellation, unsupported noncircular
profiles, stale provenance or exhausted quotas return no partial result.

Pinned OCCT comparisons cover translated/rotated transverse and axial lines,
negative-leading and linear taper polynomials, and irrational stations. Exact
adversaries additionally cover tangent contacts, cap-coincident intervals,
complete/partially clipped generators, rim endpoint incidence and quota failure.
Coincident span semantics are tested analytically rather than inferred from a
finite list of numerical OCCT intersection points.

### Rebuilding the line comparator

The live frustum line tests require the all-solid comparator, not the older
`/tmp/plainsolid-rational-hierarchy-line` binary that selected only analytic
sphere/torus/cylinder faces. The originally qualifying executable was
`/tmp/plainsolid-rational-hierarchy-planar-line`. Rebuild from tracked source:

    sh tools/occtref/build_step_rational_hierarchy_line.sh \
      /tmp/plainsolid-occt-build/include/opencascade \
      /tmp/plainsolid-occt-build/mac64/clang/lib \
      /tmp/plainsolid-rational-hierarchy-all-line
    PLAINSOLID_OCCT_RATIONAL_HIERARCHY_LINE=/tmp/plainsolid-rational-hierarchy-all-line \
      go test ./exchange/step -run '^TestIndependentCircularFrustumHierarchyLine$' -count=1

The comparator intersects every transferred solid through OCCT's face
intersector; it does not filter by surface type. The rebuilt executable passes
the same retained frustum line tests. Numerical per-solid hit deduplication is
only oracle behavior, not an exact native acceptance rule.

## Exact incident outward normal rays

`BoundaryNormalsIndependentCircularFrustumHierarchy` uses the complete exact
point-location/source proof and returns one outward ray for each incident
original face, preserving original surface IDs and rim/seam/vertex contacts.
For the tapered wall it uses (h²x,h²y,−Δr*(r0*h+Δr*z)), one half of the
outward implicit gradient. Positive radii guarantee a nonzero wall gradient.
Cap rays are the negative/positive axial direction. Exact proper occurrences
rotate these rays without floating normalization. At a rim, cap and wall rays
remain separate; shared occurrences remain separate, with no union boundary or
averaged-corner normal claim. Noncircular profiles remain unsupported here.

Returned vectors and squared norms are copied exact rationals. Source, point
proof/coefficient and output-normal quotas apply atomically. The existing normal
query's bounded snapshot/reproof pattern is retained. Pinned OCCT independently
checks wall and both rim normal sets under translated and rotated occurrences,
after native reload; normalization to floats occurs only in that comparison.
