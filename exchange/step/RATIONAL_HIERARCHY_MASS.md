# Exact symbolic assembly mass

`MeasureRationalSphericalHierarchy` and `MeasureOCCTRationalSphericalHierarchy`
reprove their corresponding source graph, native local products, original IDs,
and exact occurrence frames before measuring. No geometry is baked or rounded.
They retain a deterministic local-product table and a separate occurrence table
with source body/face IDs and complete NAUO paths.

The sum counts material occurrences, **not their Boolean union**. Reused and
overlapping occurrences each contribute fully. Density is one in the common
source length unit: volume, first moments, and inertia have dimensions L³, L⁴,
and L⁵. Physical density/unit conversion is the caller's explicit responsibility.

Volume, first moments, raw product moments, and origin inertia are exact rational
polynomials in pi (degree at most two). Centroid and central inertia are exact
unreduced quotients, with the positive material volume as denominator. Polynomial
coefficients are private; accessors return copies. No decimal approximation of pi
is part of the result. This does not strengthen the source's analytic/angular
sphere or torus native chart claims to rational patch sewing.

Planar products integrate oriented tetrahedra from their existing planar boundary
triangulation, using exact represented vertices. Circular cylinders, spheres and
ring tori use analytic centered moments. Every proper rational placement applies
the exact first/second-moment transformation, followed by the parallel-axis
identity. Local products are integrated once; occurrences reuse those moments.

Defaults: 65,536 planar triangles and 4,096 rational coefficient bits; maxima
1,048,576 triangles and 8,192 bits. Source Limits separately bound complete source
reproof; existing hierarchy depth/product/occurrence limits still apply. All
errors return an empty result. Context cancellation is checked during source
proof, per product, triangle and occurrence. Triangle allocation uses the bounded
source's existing planar tessellator before the query triangle limit is checked.

Independent qualification uses OCCT 4dbfdcc448c0b3e565488afcd626c84483172db9,
`tools/occtref/step_rational_hierarchy_mass.cxx`, and genuine standard assembly
transfer of both retained rational hierarchy fixtures. Adaptive volume integration
checks volume, centroid, and all nine central/origin inertia components. Set
`PLAINSOLID_OCCT_RATIONAL_HIERARCHY_MASS` to the built helper to run the live check.
The ordinary test independently checks exact volume/first-moment coefficients
and analytic rotated-cylinder off-diagonal central inertia.
