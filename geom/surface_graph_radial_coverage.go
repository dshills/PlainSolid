package geom

import (
	"context"
	"math/big"
)

// RadialTubeCoverageOptions bounds this proof independently from the immutable
// coefficient and uniform tube certificates supplied to it. MaxOperations is
// cumulative; MaxSubdivisions applies per exact root comparison/isolation.
// Defaults:256 tubes,64M operations,4096 coefficient bits,1M neighbor tests,
// 8192 root subdivisions. Hard caps:512,512M,16384,2M,1048576 respectively.
type RadialTubeCoverageOptions struct {
	MaxTubes, MaxOperations, MaxCoefficientBits, MaxNeighborTests, MaxSubdivisions int
}

// RadialTubeCoverage certifies coverage, not a nonredundant branch ordering or
// native topology. Original tube occurrences and both original UV endpoints
// are retained; overlapping/redundant arcs are permitted.
type RadialTubeCoverage struct {
	source            *SplineGraphRadialContours
	tubes             []VerifiedSurfaceTube
	radius            []int
	neighbors         [][2]int
	endpoints         [][2]TubeAlgebraicGraphEndpointCertificate
	operations, tests int
}

func (c *RadialTubeCoverage) TubeCount() int {
	if c == nil {
		return 0
	}
	return len(c.tubes)
}
func (c *RadialTubeCoverage) RadiusOwners() []int {
	if c == nil {
		return nil
	}
	return append([]int(nil), c.radius...)
}
func (c *RadialTubeCoverage) EndpointNeighbors() [][2]int {
	if c == nil {
		return nil
	}
	return append([][2]int(nil), c.neighbors...)
}
func (c *RadialTubeCoverage) ExactOperations() int {
	if c == nil {
		return 0
	}
	return c.operations
}
func (c *RadialTubeCoverage) NeighborTests() int {
	if c == nil {
		return 0
	}
	return c.tests
}

// Endpoint uses lower/upper ORIGINAL free parameter order, not any caller's
// oriented traversal. Exact roots/events are immutable; outward boxes are only
// display aids and are not consumed as incidence evidence.
func (c *RadialTubeCoverage) Endpoint(tube, end int) (TubeAlgebraicGraphEndpointCertificate, error) {
	if c == nil || tube < 0 || tube >= len(c.endpoints) || end < 0 || end > 1 {
		return TubeAlgebraicGraphEndpointCertificate{}, ErrInvalid
	}
	return c.endpoints[tube][end], nil
}

// CertifyTubeCoverage proves a complete cover of EVERY circle of this exact
// radial certificate. Initial scope is its original identity-XY charts and
// unanchored regular uniform tubes, with unchanged original UV coordinates.
// Each tube support is independently compared with the corresponding original
// surface restricted to the identical exact UV rectangle. Each endpoint's XY
// circle equation is evaluated exactly at its immutable algebraic model root.
// All four endpoint UV roots must lie STRICTLY inside a same-radius neighbor's
// original domains. Uniform target uniqueness then proves actual membership;
// overlapping boxes, rounded endpoints and caller labels never prove incidence.
//
// Proof: a tube's continuous contact arc belongs to one of the disjoint circles
// of the complete coefficient certificate, hence to the circle identified by
// its endpoint. Identity XY and a nonzero free tangent make each tube interior
// relatively open in that circle. Strict endpoint witnesses make the finite
// closed union relatively open too. A nonempty open-and-closed subset of the
// connected circle is the whole circle. Every certified radius must be present.
// Missing arcs/radii, endpoint-only joins, changed supports and exhausted work
// fail atomically. This sufficient proof does not infer native branch topology.
func (r *SplineGraphRadialContours) CertifyTubeCoverage(ctx context.Context, tubes []VerifiedSurfaceTube, o RadialTubeCoverageOptions) (*RadialTubeCoverage, error) {
	if ctx == nil || r == nil || r.supports[0] == nil || r.supports[1] == nil || len(r.radii) == 0 || len(tubes) == 0 {
		return nil, ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return nil, e
	}
	if o.MaxTubes == 0 {
		o.MaxTubes = 256
	}
	if o.MaxOperations == 0 {
		o.MaxOperations = 64000000
	}
	if o.MaxCoefficientBits == 0 {
		o.MaxCoefficientBits = 4096
	}
	if o.MaxNeighborTests == 0 {
		o.MaxNeighborTests = 1000000
	}
	if o.MaxSubdivisions == 0 {
		o.MaxSubdivisions = 8192
	}
	if o.MaxTubes < 1 || o.MaxTubes > 512 || o.MaxOperations < 1 || o.MaxOperations > 512000000 || o.MaxCoefficientBits < 64 || o.MaxCoefficientBits > 16384 || o.MaxNeighborTests < 1 || o.MaxNeighborTests > 2000000 || o.MaxSubdivisions < 1 || o.MaxSubdivisions > 1048576 {
		return nil, ErrInvalid
	}
	if len(tubes) > o.MaxTubes {
		return nil, ErrConvergence
	}
	b := &polynomialBudget{maxOperations: o.MaxOperations, maxBits: o.MaxCoefficientBits}
	remaining := func() (int, error) {
		n := b.maxOperations - b.operations
		if n < 1 {
			return 0, ErrConvergence
		}
		return n, nil
	}
	out := &RadialTubeCoverage{source: r, tubes: append([]VerifiedSurfaceTube(nil), tubes...), radius: make([]int, len(tubes)), neighbors: make([][2]int, len(tubes)), endpoints: make([][2]TubeAlgebraicGraphEndpointCertificate, len(tubes))}
	present := make([]bool, len(r.radii))
	for i, t := range tubes {
		if e := ctx.Err(); e != nil {
			return nil, e
		}
		if t.a == nil || t.b == nil || t.axis < 0 || t.axis > 3 || t.tangentSign == 0 || t.anchor != nil {
			return nil, ErrConvergence
		}
		out.neighbors[i] = [2]int{-1, -1}
		for owner, p := range []*SurfacePatch{t.a, t.b} {
			d := p.Domain()
			domain := [2][2]*big.Rat{{new(big.Rat).SetFloat64(d.UMin), new(big.Rat).SetFloat64(d.UMax)}, {new(big.Rat).SetFloat64(d.VMin), new(big.Rat).SetFloat64(d.VMax)}}
			domains := [2][2][2]*big.Rat{domain, domain}
			n, e := remaining()
			if e != nil {
				return nil, e
			}
			same, e := compareExactSurfaceRationalDomains(ctx, r.supports[owner], p, ExactSurfaceComparisonOptions{MaxCells: 64, MaxOperations: n, MaxCoefficientBits: o.MaxCoefficientBits}, &domains)
			b.operations += same.ExactOperations
			if e != nil {
				return nil, e
			}
			if !same.Complete || !same.Equal {
				return nil, ErrDomain
			}
		}
		selected := -1
		for end := 0; end < 2; end++ {
			n, e := remaining()
			if e != nil {
				return nil, e
			}
			endpoint, e := t.DiscoverAlgebraicGraphEndpoint(ctx, end == 1, TubeAlgebraicGraphEndpointOptions{SurfaceCurveAgreementOptions: SurfaceCurveAgreementOptions{SurfacePatchOptions: SurfacePatchOptions{MaxOperations: n, MaxCoefficientBits: o.MaxCoefficientBits}, MaxSubdivisions: o.MaxSubdivisions}})
			b.operations += endpoint.ExactOperations
			if e != nil {
				return nil, e
			}
			if !endpoint.Proven || endpoint.ModelEvent == nil || endpoint.ModelEvent.cell == nil || endpoint.ModelEvent.root == nil {
				return nil, ErrConvergence
			}
			for axis := 0; axis < 2; axis++ {
				if endpoint.Parameters[axis] == nil || endpoint.Parameters[axis+2] == nil {
					return nil, ErrConvergence
				}
				same, e := compareRootBudget(ctx, endpoint.Parameters[axis].root, endpoint.Parameters[axis+2].root, b, o.MaxSubdivisions)
				if e != nil {
					return nil, e
				}
				if !same.Proven || same.Order != 0 {
					return nil, ErrConvergence
				}
			}
			owner := -1
			for k, radius := range r.radii {
				sign, e := radialCoverageEndpointSign(ctx, endpoint.ModelEvent, r.center, radius, b, o.MaxSubdivisions)
				if e != nil {
					return nil, e
				}
				if sign == 0 {
					if owner >= 0 {
						return nil, ErrConvergence
					}
					owner = k
				}
			}
			if owner < 0 || end == 1 && owner != selected {
				return nil, ErrConvergence
			}
			selected = owner
			out.endpoints[i][end] = endpoint
		}
		out.radius[i] = selected
		present[selected] = true
	}
	for _, exists := range present {
		if !exists {
			return nil, ErrConvergence
		}
	}
	for i := range tubes {
		for end := 0; end < 2; end++ {
			for j, t := range tubes {
				if i == j || out.radius[i] != out.radius[j] {
					continue
				}
				out.tests++
				if out.tests > o.MaxNeighborTests {
					return nil, ErrConvergence
				}
				if e := ctx.Err(); e != nil {
					return nil, e
				}
				inside, e := radialCoverageStrictEndpoint(ctx, out.endpoints[i][end].Parameters, t, b, o.MaxSubdivisions)
				if e != nil {
					return nil, e
				}
				if inside {
					out.neighbors[i][end] = j
					break
				}
			}
			if out.neighbors[i][end] < 0 {
				return nil, ErrConvergence
			}
		}
	}
	if b.operations > b.maxOperations {
		return nil, ErrConvergence
	}
	if e := ctx.Err(); e != nil {
		return nil, e
	}
	out.operations = b.operations
	return out, nil
}

func radialCoverageEndpointSign(ctx context.Context, e *ExactComposedCurveEvent, center [2]*big.Rat, radius *big.Rat, b *polynomialBudget, subdivisions int) (int, error) {
	var residual rationalPolynomial
	for axis := 0; axis < 2; axis++ {
		p, err := subtractScaledPolynomial(e.cell.power[axis], e.cell.power[3], center[axis], b)
		if err != nil {
			return 0, err
		}
		square, err := polynomialProduct(p, p, b)
		if err != nil {
			return 0, err
		}
		residual, err = subtractScaledPolynomial(residual, square, big.NewRat(-1, 1), b)
		if err != nil {
			return 0, err
		}
	}
	w2, err := polynomialProduct(e.cell.power[3], e.cell.power[3], b)
	if err != nil {
		return 0, err
	}
	radius2 := new(big.Rat).Mul(radius, radius)
	if err = b.check(radius2); err != nil {
		return 0, err
	}
	residual, err = subtractScaledPolynomial(residual, w2, radius2, b)
	if err != nil {
		return 0, err
	}
	sign, _, err := composedPolynomialRootSign(ctx, e.root, trimPolynomial(residual), b, subdivisions)
	return sign, err
}
func radialCoverageStrictEndpoint(ctx context.Context, roots [4]*ExactAlgebraicRoot, t VerifiedSurfaceTube, b *polynomialBudget, subdivisions int) (bool, error) {
	for owner, p := range []*SurfacePatch{t.a, t.b} {
		d := p.Domain()
		ranges := [2][2]float64{{d.UMin, d.UMax}, {d.VMin, d.VMax}}
		for axis, interval := range ranges {
			root := roots[2*owner+axis]
			if root == nil || root.root == nil {
				return false, ErrInvalid
			}
			for side, value := range interval {
				q := new(big.Rat).SetFloat64(value)
				if e := b.check(q); e != nil {
					return false, e
				}
				bound := &ExactGraphLineRoot{polynomial: rationalPolynomial{new(big.Rat).Neg(q), big.NewRat(1, 1)}, bounds: [2]*big.Rat{q, q}}
				order, e := compareRootBudget(ctx, root.root, bound, b, subdivisions)
				if e != nil {
					return false, e
				}
				if !order.Proven {
					return false, ErrConvergence
				}
				if side == 0 && order.Order <= 0 || side == 1 && order.Order >= 0 {
					return false, nil
				}
			}
		}
	}
	return true, nil
}
