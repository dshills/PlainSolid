# Original nonplanar spline cavity material

`ReadNonplanarSplineCavitySource` authenticates a bounded standard
`BREP_WITH_VOIDS` with one outward closed shell and exactly one inward
`ORIENTED_CLOSED_SHELL` occurrence. Each original shell has six polynomial
spline faces, twelve model edges and 24 bounded degree-one spline pcurves.
Both are genuinely nonplanar, have quadratic-or-lower carriers and unit `[0,1]`
parameter domains. This does not broaden existing single-shell readers.

The original shell/face/edge/pcurve/support graph is proved before material
promotion. The two closures may share only parametric context records, never
geometry, topology or model curves. Each isolated proof view preserves the
original geometry/role IDs, normalizes only the shell occurrence, and passes
the existing whole-domain model/pcurve and complete outward embedding proof.
The source's inner shell occurrence must be explicitly reversed. Unowned
geometry, extra roots, crossing ownership and unsupported records fail closed.

`trimmed.CombineSeparatedSplineShells` proves complete boundary separation and
exact witness classification. Acceptance requires inner-inside-outer, not the
reverse, and a retained inward inner shell. Mere nested bounding boxes or closed
incidence do not establish acceptance. Touching and unresolved cases return no
material. The original carrier faces remain unchanged; the native inner face
orientation is reversed. `History` maps each original shell's face ordering to
native face IDs. `SourceEdges` preserves original edge IDs without claiming
that those numeric IDs equal native edge indices. Body, shell, void occurrence,
face and pcurve provenance are exposed through copied observations.

Limits are 1 MiB source, 2048 entities, 32768 values, parser depth 16 and the
fixed twelve-face/twenty-four-edge topology. Caller face/use limits are applied
to the combined model, not independently reset for each shell. The vertex budget
conservatively counts all original Cartesian point records. Native separation,
embedding and ray algorithms retain their bounded defaults and context checks.

## Output and persistence

`WriteNonplanarSplineCavitySource` emits the original source after reproof.
`WriteNonplanarSplineCavity` authors standard generated `BREP_WITH_VOIDS` from
the retained original carriers and re-proves that complete output. The private
authoring path converts the generic writer's exactly unit-weight splines to
identical polynomial STEP entities and its unit-domain UV lines to exact
bounded two-pole splines. It does not normalize external rational geometry or
introduce an external LINE acceptance rule. Newly authored product contexts
use their supported base entities; original metadata is preserved only by the
source writer. These are distinct source-preserving and generated-output APIs.

The `plainsolid-step-nonplanar-spline-cavity-source` version 1 JSON envelope
retains original source, both shell identities, original owner data, inward
history, generated STEP and all three native solid claims (outer, inner,
material). Reload re-proves source geometry and strict containment, then compares
all claims. Stored native bytes are never trusted as a substitute for proof.
Ordinary native JSON reload also revalidates the two nested components and
inward depth. Forged orientation, source/native claims or topology fail atomically.

## Independent original and measurements

`tools/occtref/author_ap242_spline_roof_void.cxx` independently builds/sews two
original OCCT spline roof shells, reverses the inner shell occurrence and emits
an actual AP242 `BREP_WITH_VOIDS`. No Go STEP output is used for authoring.
Pinned OCCT revision: `4dbfdcc448c0b3e565488afcd626c84483172db9`.
The retained original has 624 entities and twelve faces.

Outer domain is `0<=x<=4, 0<=y<=3, 0<=z<=2+(x/4)^2`.
The inner domain is the same roof scaled by 1/4 and translated by `(1,1,1/2)`.
Independent exact volume is `441/16=27.5625`; centroid is
`(3797/1764,5299/3528,4205/3528)`. Area is
`17/16 * (12+56/3+15+3*sqrt(5)+12*asinh(1/2))`.
Pinned OCCT reports `61.783750781123864`. Source/native/source and generated
output both transfer as one valid solid, twelve faces, matching volume/area and
centroid. Native continuous edge agreement is exactly zero; point queries
confirm material, void and inner-boundary ownership.

```sh
sh tools/occtref/build_author_ap242_spline_roof_void.sh \
 /tmp/plainsolid-occt-build/include/opencascade \
 /tmp/plainsolid-occt-build/mac64/clang/lib \
 /tmp/plainsolid-ap242-spline-roof-void
PLAINSOLID_OCCT_SPLINE_ROOF_VOID=/tmp/plainsolid-ap242-spline-roof-void \
 go test ./exchange/step -run '^TestNonplanarSplineCavity' -count=1
go test -race ./exchange/step -run '^TestNonplanarSplineCavity' -count=1
go test ./exchange/step -run '^$' -fuzz '^FuzzNonplanarSplineCavitySource$' -fuzztime=40x -parallel=1
```

Negatives include touching floor, outside inner shell, inverted nesting,
outward void occurrence, shared cross-shell support, wrong inner pcurve owner,
changed trim and forged native geometry/history/source IDs. Harmless record
reorder, copied provenance, old-reader rejection, native byte stability,
cumulative source limits and canceled atomic failure are covered.
