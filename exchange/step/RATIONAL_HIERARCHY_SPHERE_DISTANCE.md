# Exact algebraic distance to sphere occurrences

`DistanceRationalHierarchySpheres`, `DistanceOCCTRationalHierarchySpheres`, and
`AffinePCurveHierarchy.SphereDistance` add a separate per-occurrence sphere
query. They re-prove the original complete source and native claims, preserve
source product/body/face IDs and occurrence paths, and leave other distance
and import APIs unchanged. Planar, cylinder and torus leaves are explicitly
unsupported here, with nil distances. There is no partial assembly minimum.

## Algebraic proof

Let the exactly transformed sphere center be C, radius r>0, query point P, and
`s=|P-C|²`. The exact boundary squared distance is
`(sqrt(s)-r)²`. For s>0 away from the boundary it is the unique root in
`[0,s+r²]` of

```
t² - 2(s+r²)t + (s-r²)² = 0.
```

The other root is strictly above the two roots’ midpoint s+r². Both
coefficients and the complete root isolation are rational and bounded; no
floating root is selected. On the sphere, squared distance is exactly zero.
At the center it is exactly r². Exact comparison of s with r² also determines
inside/boundary/outside. Material squared distance is zero inside and on the
sphere, and the boundary squared distance outside.

For P != C, the unique nearest boundary point is represented exactly as
`C + lambda*(P-C)`, where lambda is the unique positive root of
`s*lambda²-r²=0`. Isolation uses `[0,1+r²/s]`, whose upper bound strictly exceeds
the positive root. The representation is complete even when the point has
irrational coordinates. `RadialScale` returns the immutable algebraic root;
center, direction and radius getters return owned rationals.

`Enclose` provides outward rational coordinate intervals with a caller-requested
maximum width. It refines lambda and applies the exact affine interval map,
reversing endpoints for negative direction components. It does not convert an
irrational point into a claimed exact binary64 point.

For P=C the nearest boundary set is the **entire sphere**. `Nonunique=true`,
center/radius identify the set, and no radial scale or arbitrarily chosen
nearest point is returned. Requesting a unique point enclosure returns
`geom.ErrDomain`. A boundary query, by contrast, has a unique nearest point
coincident with the input and scale exactly one. Repeated material occurrences
retain separate identities; this is not a Boolean union or global tie ledger.

Only original face ownership is reported for the sphere. No periodic seam or
pole edge/vertex role is inferred from a nearest point. The native analytic
sphere's trigonometric evaluation is not claimed to be exact rational sewing.

## Limits and independent evidence

Root work and subdivisions are cumulative over supported occurrences. The
query reserves a fixed bound of 128 operations per occurrence for its small
exact affine/coefficient calculation; isolation and rational-root construction
charge their reported actual work. All intermediate coefficients obey the
selected bit limit. Source reproof has its existing separate bounded limits.
Point enclosure is a separate bounded operation with a 32-operation reservation
for its affine interval calculation. Failures/cancellation return no partial
query or coordinate enclosure.

Tests cover rational exterior/interior distances, exact zero on the sphere,
center nonuniqueness, irrational interior/exterior squared distances and
nearest coordinates, both rational occurrence-frame conventions, native reload,
source tamper, budgets and immutable observations. Exact polynomial sign tests
verify the retained roots; known rational values are compared algebraically.
Original pinned OCCT boundary/material distances and nearest points agree with
the rational enclosures. Finite planar hole/reentrant distance tests remain
unchanged and are run alongside the new sphere lane.

```
PLAINSOLID_OCCT_RATIONAL_HIERARCHY_DISTANCE=/tmp/plainsolid-rational-hierarchy-distance \
  go test -race ./exchange/step -run 'TestHierarchySphereDistance|TestHierarchyPlanarDistance'
```

The comparator is built by `tools/occtref/build_step_rational_hierarchy_distance.sh`
against OCCT pin `4dbfdcc448c0b3e565488afcd626c84483172db9`. Center nonuniqueness
is proved directly and not replaced by the comparator's finite witness list.
