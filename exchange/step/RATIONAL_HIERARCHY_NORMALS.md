# Exact outward boundary normal rays

`BoundaryNormalsRationalHierarchy`, `BoundaryNormalsOCCTRationalHierarchy` and
`AffinePCurveHierarchy.BoundaryNormals` add source-aware normals to the existing
exact general point classification. They do not expand imported geometry.

Each material occurrence returns its exact point classification and every
incident original face normal. At an edge or corner the normals are **not**
averaged. At a split-cylinder seam, two distinct source faces can carry equal
normal rays; both are retained. Duplicate uses of one periodic face do not
invent additional face owners. Original product/body/face/support IDs and full
occurrence paths are preserved. Planar edge/vertex contacts retain the existing
exact source contact IDs; no analytic edge/vertex IDs are guessed from normals.

## Exact directions and orientation

Vectors are outward rational normal rays, not necessarily unit vectors.
`Vector` and `SquaredNorm` return independent rational copies. This preserves
exact directions without rounding a square-root normalization.

- Planar normals use the original support Z axis and ADVANCED_FACE sense, after
  exact finite-loop ownership and native/source embedded material reproof.
  Nonconvex reentrant faces and hole walls retain material orientation, including
  normals directed into an aperture. All incident finite faces are included.
- Finite-cylinder caps use the proved lower/upper source face roles; side normals
  are radial. Exact closed-quadrant predicates select the independently proved
  half/quarter cell roles. On seams both adjacent source faces are returned.
- A natural sphere uses its outward radial vector, including the poles. Its
  sole source face owns the vector; pole chart singularity does not erase the
  regular material normal.
- A ring torus uses one quarter of the exact outward quartic gradient:
  `(x*(s-2R²), y*(s-2R²), z*s)`, where
  `s=x²+y²+z²+R²-r²`. The authenticated `R>r>0` family is regular. Inner torus
  normals point toward the hole. No approximate trigonometric evaluation is
  promoted to an exact rational patch claim.

The source-local analytic frame and original proper rational occurrence
rotation transport these vectors exactly. Translation has no effect on a
normal. Every returned squared norm is proved positive. Unsupported sealed
cavity/multiple-shell source families remain outside the hierarchy reader;
this query does not infer them from normal direction.

An interior or exterior occurrence returns no normals. Shared or overlapping
assembly occurrences are considered independently, not Boolean-unioned.

## Limits, snapshots and independent qualification

The query first owns and re-proves a source snapshot, then calls the unchanged
exact point locator on that snapshot. These are two bounded source-proof passes;
point finite-face/ray counters describe the locator, not the parse passes.
Point coefficient/source/proof/ray budgets retain their existing meanings.
`MaxNormals` bounds all emitted face normals across occurrences. Vector and
quartic arithmetic obey the selected coefficient limit. Failure or cancellation
returns no partial result. Caller-visible evidence never aliases source state.

The independently built OCCT comparator transfers the original STEP, projects
the world point onto each original face support, checks its finite face domain,
and evaluates the oriented geometric normal. It tests periodic UV equivalents
so a face ending at `2*pi` is not lost when projection chooses `0`. A regular
sphere radial fallback handles a parametrically singular pole. Its tolerances
apply only to independent comparison, not the Go proof.

Tests include original reentrant and affine-pcurve Boolean-cut products in both
rational hierarchy conventions; native reload; convex vertices, reentrant
edges and aperture walls; cylinder cap/seam simultaneity; sphere poles; outer,
inner and top torus points; and a rational non-seam cylinder point `(6/5,8/5,1)`.
All incident outward directions are compared against pinned OCCT, retaining
multiplicity across distinct faces. Adversaries include forged ancestry,
coefficient/proof/normal limits, cancellation and returned-rational mutation.

```
sh tools/occtref/build_step_rational_hierarchy_normals.sh \
  /path/to/occt/include /path/to/occt/lib /tmp/plainsolid-rational-hierarchy-normals
PLAINSOLID_OCCT_RATIONAL_HIERARCHY_NORMALS=/tmp/plainsolid-rational-hierarchy-normals \
  go test -race ./exchange/step -run 'TestHierarchyBoundaryNormals|TestHierarchyNormalsReject|TestHierarchyRationalNonseamNormal'
```

Qualified OCCT source pin: `4dbfdcc448c0b3e565488afcd626c84483172db9`.
