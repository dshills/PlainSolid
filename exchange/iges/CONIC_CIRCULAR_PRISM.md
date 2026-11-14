# Circular conic tabulated material

`ReadConicCircularPrismMaterial` and `WriteConicCircularPrism` support a separate
canonical twenty-record IGES186 solid: two full type104 circular conics, a
model-only type122 tabulated lateral support, two type190 caps, and the explicit
502/504/508/510/514 topology. This is a distinct representation from the existing
type192 circular cylinder material. It is not general IGES solid import.

The centered source equation must have A=C>0, B=D=E=0, F<0 and exactly
representable positive radius sqrt(-F/A). Lower and upper Z and their positive
difference must be lossless. Original source units, record metadata, parameters
and geometry are retained. Old ellipse and type192 entry points are unchanged.

## Proof and normalization

For structural proof only, apply the explicitly invertible affine map
(x,y,z) -> (x,y/2,z). The conic coefficient C becomes 4C, giving a strict ellipse
accepted by the existing complete topology validator. The parser first proves
that every other coordinate-bearing record is invariant under this map:

- Type116 origins, type502 vertices, both type110 seam endpoints and the type122
  terminal have exactly zero Y.
- Type123 normal/reference directions have zero Y; canonical reproof further
  requires Z normals and X references.
- The conic has no cross or linear terms and both seam endpoints have zero Y.
- Type190 references these authenticated origins/directions. The image of its
  complete horizontal support is the same plane; no parameterized pcurve occurs.
- The type122 image is exactly translation of the transformed conic by the
  unchanged axial terminal displacement. Every graph and incidence-only record
  is preserved.

No other coordinate record or transform is admitted. Multiplying C by four must
be lossless, or the reader fails. The normalized proof establishes the original
solid through this positive-determinant bijection; it does not replace its native
geometry. Native promotion uses additive `ellipticprism.CirclePrism`, with the
actual circle's exact rational quarter controls, planar cap trims, continuous
shared edges and outward closed topology. Existing ellipse constructors remain
strictly noncircular.

Source-bound version1 native persistence re-runs this proof and compares native
geometry and source ancestry. Plain trimmed JSON also retains the exact circular
quarter geometry. Source limits are32KiB/512records/20entities/2048values, with
caller limits honored; native envelope cap is512KiB. No approximation or repair
is performed. The new API currently keeps canonical record/list order.

## Independent qualification

`tools/occtref/author_iges_conic_circular_prism.py` specializes the independently
authored original20-record analytic fixture to the exact equation
4x²+4y²-100=0 at Z=3 and7. The pinned OCCT standalone comparator imports this
original and Go-emitted IGES as a valid three-face solid. Both have area90π,
volume100π and centroid(0,0,5). Native integration and native reload agree.

Tests reject noncircular conics, changed noninvariant point/direction/vertex/
terminal/seam coordinates and resource exhaustion. Existing ellipse input is
rejected by the circular API and vice versa. The seeded fuzz target is
`FuzzConicCircularPrism`; the bounded qualification run uses100 executions.
