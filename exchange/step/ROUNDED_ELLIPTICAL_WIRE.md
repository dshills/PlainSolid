# Explicit bounded reconstruction of OCCT's rounded full ellipse

`RepairRoundedEllipticalWire(reader, limits, EllipticalWireRepairOptions)` is
opt-in reconstruction, separate from `ReadEllipticalWire`. The strict reader
continues to reject the original decimal trimmed representation. No source
repair is performed implicitly by an existing reader or writer.

The accepted source is the retained original OCCT 29-record single-ellipse
product graph. It has forward `TRIMMED_CURVE` sense, master representation
`PARAMETER`, start exactly zero, and one parameter plus one Cartesian selector
at each end. The two selectors must have distinct original IDs and exactly
equal coordinates at center + major_radius × reference_direction. This equality
uses the original exact decimals and rational arithmetic, not trig evaluation
or rounded vector addition. The ellipse has the existing exact cardinal frame,
positive ordered radii and strict original millimeter/radian context.

Only the trim and its two exclusive selector records are removed. The surviving
26-record natural graph must pass the unchanged exact reader's full bijective
ownership proof. Any remaining reference to a removed record, extra geometry,
foreign owner or unsupported representation fails atomically. Original and new
graphs are independently owned; the old graph and removed IDs remain available.

## Continuous mathematical correspondence

Let E be the unchanged analytic ellipse, a its major radius, and θ the exact
authored decimal end parameter. The reported correspondence is explicitly

- authored numerical lift: C(s) = E(s θ),
- reconstructed natural loop: N(s) = E(s 2π), for 0 ≤ s ≤ 1.

Because the exact cardinal axes are orthonormal and 0 < b < a,
`||E'(u)|| = sqrt(a² sin²(u) + b² cos²(u)) ≤ a` everywhere. Therefore
`||C(s)-N(s)|| ≤ a |θ-2π|` continuously. The same bound controls the original
parameter endpoint's disagreement with its coincident +major Cartesian selector;
the start selector has exactly zero disagreement.

The implementation uses the existing rational Machin enclosure
π = 16 atan(1/5) − 4 atan(1/239), with 32 alternating terms and a rigorous
next-term remainder. It returns the exact source interval, lower/upper bounds
for 2π, a signed enclosure for θ−2π, and the rational physical upper bound
`a * max(abs(residual_lower), abs(residual_upper))`.

`MaxDistance` is required, positive and finite, and cannot exceed b/1,000,000
in model units. The complete upper bound must fit it. Thus multiple turns and
large angle changes cannot enter this gate. The fixed π enclosure can make an
extremely small requested tolerance inconclusive; such inputs fail rather than
weakening the bound. This is a bound on mathematical geometry, not on errors of
arbitrary binary64 evaluation calls.

This correspondence does **not** assert a vendor's periodic trim normalization
or tolerance policy is identical to the numerical lift. In particular, it does
not declare the source interval exactly one turn, recover an unspecified finite
trim, or prove exact source equivalence. The caller explicitly authorizes a new
full natural loop, replacing a small authored gap or overlap. No material or
face topology is inferred from this wire.

## Qualification

The original independently generated `occt-rounded-ellipse-wire.step` has
θ = 6.28318530718 and radii 4 and 2. Its positive angular residual lies strictly
between 4×10⁻¹³ and 5×10⁻¹³ radians. The physical upper bound is approximately
1.655×10⁻¹² model units; the successful test tolerance is 10⁻¹¹. The uncertainty
metadata is retained but never substitutes for the explicit caller tolerance.

Pinned OCCT 4dbfdcc448c0b3e565488afcd626c84483172db9 transfers both original
source and reconstructed natural source re-export as one valid edge. Adaptive
length is 19.376896441095354 and point/tangent values agree with the unchanged
native analytic support. An independent Simpson speed integral checks length;
no mesh, fitting or source-normalization helper supplies the reference.

Focused tests cover over- and under-turn bounds, exact selector positions,
incorrect radius/frame/owner, missing or duplicated selectors, reversed sense,
wrong trim preference, multiple turns, undersized/oversized/nonfinite tolerance,
resource limits, unchanged strict rejection, and independent source ownership.
The bounded fuzz campaign runs 100 inputs. Export the new graph explicitly via
`WriteEllipticalWireSource(w, result.Natural.Source, limits)`; the original graph remains available separately.
