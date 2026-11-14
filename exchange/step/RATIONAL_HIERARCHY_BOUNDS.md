# Certified rational hierarchy world bounds

`BoundRationalSphericalHierarchy` and `BoundOCCTRationalSphericalHierarchy`
compute per-expanded-leaf and aggregate world AABBs under the two separately
proved common-SI rational frame conventions. They re-prove the original source
and compare supplied native/pose/provenance claims once before querying. A
mutated cached pose, path or body does not become a new trusted assembly.

Each leaf retains original definition/product/body IDs, owned source face IDs,
and complete original NAUO path. The bound covers the **whole local material**;
source face IDs are ancestry, not a claim of separate tight face-trim boxes.
Bodies are never transformed into rounded native geometry or unioned together.
Aggregate is the AABB of all occurrences, with no overlap/disjointness claim.
Coordinates remain in the common source model unit (`LengthUnit`).

## Exact formulas

Planar extrema are minima/maxima of exact rational transforms of all original
native planar vertices. Polygonal material extrema occur at those vertices;
there is no sampling of curved geometry or approximate mesh bound.

For an exact world center c and unit axis n:

- sphere radius r: extent r on every axis;
- finite cylinder radius r, height h: extent
  `(h/2)*abs(n_i) + r*sqrt(1-n_i²)`, about the exact mid-height center;
- natural ring torus major radius R, minor radius r: extent
  `R*sqrt(1-n_i²) + r`.

Local frame origins, radii and heights are converted from their already proved
represented values to exact rationals before arithmetic. Centers and axes are
composed with exact occurrence matrices. Axis unit length is checked exactly.

Perfect rational square roots remain exact. Otherwise integer square root of
`floor(q * 2^(2*b))` gives adjacent dyadic endpoints enclosing sqrt(q), width
`2^-b`. Using the upper root gives an outward box. `MaximumAxisPadding()` bounds
the excess of either returned extreme beyond the true extremum: r times the
root interval width for a cylinder, R times for a torus, zero for planar and
sphere. Aggregate padding is the maximum leaf padding on each axis, which also
bounds the aggregate extremal excess. Min/Max/padding accessors return owned
copies, not mutable internal rationals.

No float sqrt or tolerance participates in acceptance or enclosure construction.
Precision refinement yields nested outward bounds. The 3:4:5 fixture has exact
aggregate bounds [-23,-20,-5]..[162/5,30,15] and zero padding. The noncommuting
OCCT-frame fixture requires irrational support extrema; at64 fraction bits each
side's error is bounded by its reported rational padding.

## Limits and behavior

Options default to64 fraction bits,65536 transformed points,192 square roots,
and2048 coefficient bits; caller Source limits independently bound full source
reproof. Fraction bits must be1..128, point budget1..1048576, root budget1..192,
and coefficient budget256..4096. Analytic center evaluation counts as one point;
each cylinder/torus axis counts one square-root request, including exact roots.
Stored coordinates/enclosures, point arithmetic terms and root scaling integers
are checked against the coefficient bound. Existing source/frame bounds also
limit fixed-size intermediate arithmetic. Budget exhaustion and cancellation
return an error with no partial leaves. No shape guarantee is weakened to meet
a query budget.

The retained seven-occurrence fixtures use14 point evaluations and12 root
requests. Tests independently square the reported circular-root support
intervals, verify padding, exact extrema, precision nesting, immutable accessors,
source ancestry, stale-query rejection and budget/cancellation behavior.

Pinned OCCT `4dbfdcc448c0b3e565488afcd626c84483172db9` original transfers match
both oblique aggregate bounds to comparator precision. OCCT's approximately
1e-7 bounding margin is used only for external numerical comparison, not as a
proof tolerance. Ordinary/race/Go1.23/vet, recent exchange regressions and a
fixed100-execution bounded query fuzz campaign qualify the milestone.
