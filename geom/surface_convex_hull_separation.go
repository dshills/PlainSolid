package geom

import (
	"context"
	"math/big"
)

// SurfaceConvexHullSeparationOptions bounds an exact support search.
// Zero selects 32 iterations, 1M exact operations, and 16384 coefficient bits.
type SurfaceConvexHullSeparationOptions struct {
	MaxIterations, MaxOperations, MaxCoefficientBits int
}

// SurfaceConvexHullSeparation holds the strict certificate
// min(Direction·B)-max(Direction·A) >= Gap > 0. The unnormalized direction
// and gap are caller-owned rationals. An inconclusive search reports no contact.
type SurfaceConvexHullSeparation struct {
	Separated                   bool
	Direction                   [3]*big.Rat
	Gap                         *big.Rat
	Iterations, ExactOperations int
}

// SeparateSurfacePatchesConvexHull searches for a strict separating
// projection of the entire positive-weight control hulls. It uses full
// support scans and exact closest points on small simplices. It can prove
// separation missed by coordinate axes or the center normals. Shared hull
// points, hull overlap, and iteration exhaustion are inconclusive; even when
// hulls overlap the original surfaces may be disjoint.
func SeparateSurfacePatchesConvexHull(ctx context.Context, a, b *SurfacePatch,
	o SurfaceConvexHullSeparationOptions) (r SurfaceConvexHullSeparation, err error) {
	if ctx == nil || a == nil || b == nil || len(a.net) == 0 || len(b.net) == 0 {
		return r, ErrInvalid
	}
	if o.MaxIterations == 0 {
		o.MaxIterations = 32
	}
	if o.MaxIterations < 1 || o.MaxIterations > 1024 {
		return r, ErrInvalid
	}
	limits, err := patchOptions(SurfacePatchOptions{MaxOperations: o.MaxOperations, MaxCoefficientBits: o.MaxCoefficientBits})
	if err != nil {
		return r, err
	}
	budget := &polynomialBudget{maxOperations: limits.MaxOperations, maxBits: limits.MaxCoefficientBits}
	defer func() { r.ExactOperations = budget.operations }()
	if len(a.net)+len(b.net) > limits.MaxOperations/3 {
		return r, ErrConvergence
	}
	math := hullRatArithmetic{boundedRatArithmetic{b: budget}}
	c, err := math.controlPoints(ctx, a)
	if err != nil {
		return r, err
	}
	d, err := math.controlPoints(ctx, b)
	if err != nil {
		return r, err
	}
	simplex := []hullRat3{math.minus(d[0], c[0])}
	for r.Iterations < o.MaxIterations {
		if err := ctx.Err(); err != nil {
			return r, err
		}
		r.Iterations++
		near, dist2, active := math.closestSimplex(simplex)
		if math.err != nil {
			return r, math.err
		}
		if dist2.Sign() == 0 {
			return r, nil
		}
		// Find the minimum support of B-A along the nonzero nearest
		// vector. Positive support certifies all original controls.
		bestA, maxA, err := math.support(ctx, c, near, true)
		if err != nil {
			return r, err
		}
		bestB, minB, err := math.support(ctx, d, near, false)
		if err != nil {
			return r, err
		}
		gap := math.sub(minB, maxA)
		if math.err != nil {
			return r, math.err
		}
		if gap.Sign() > 0 {
			r.Separated, r.Direction, r.Gap = true, near, gap
			return r, nil
		}
		// The support value is strictly below |near|², so the
		// enlarged simplex will have a strictly nearer point.
		simplex = append(active, math.minus(bestB, bestA))
	}
	return r, math.err
}

type hullRat3 = [3]*big.Rat
type hullRatArithmetic struct{ boundedRatArithmetic }

func (a *hullRatArithmetic) dot(p, q hullRat3) *big.Rat {
	return a.add(a.add(a.mul(p[0], q[0]), a.mul(p[1], q[1])), a.mul(p[2], q[2]))
}

func (a *hullRatArithmetic) minus(p, q hullRat3) (r hullRat3) {
	for i := range r {
		r[i] = a.sub(p[i], q[i])
	}
	return r
}

func (a *hullRatArithmetic) controlPoints(ctx context.Context, p *SurfacePatch) ([]hullRat3, error) {
	out := make([]hullRat3, len(p.net))
	for i, h := range p.net {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		for j := range 3 {
			out[i][j] = a.quo(h[j], h[3])
		}
		if a.err != nil {
			return nil, a.err
		}
	}
	return out, nil
}

func (a *hullRatArithmetic) support(ctx context.Context, net []hullRat3, direction hullRat3, maximum bool) (hullRat3, *big.Rat, error) {
	best, projection := net[0], a.dot(net[0], direction)
	for _, point := range net[1:] {
		if err := ctx.Err(); err != nil {
			return hullRat3{}, nil, err
		}
		p := a.dot(point, direction)
		if a.err != nil {
			return hullRat3{}, nil, a.err
		}
		if cmp := p.Cmp(projection); (maximum && cmp > 0) || (!maximum && cmp < 0) {
			best, projection = point, p
		}
	}
	return best, projection, a.err
}

func (a *hullRatArithmetic) closestSimplex(simplex []hullRat3) (best hullRat3, dist2 *big.Rat, active []hullRat3) {
	for mask := 1; mask < 1<<len(simplex); mask++ {
		face := make([]hullRat3, 0, len(simplex))
		for i, p := range simplex {
			if mask&(1<<i) != 0 {
				face = append(face, p)
			}
		}
		p, ok := a.closestFace(face)
		if a.err != nil {
			return
		}
		if !ok {
			continue
		}
		s := a.dot(p, p)
		if dist2 == nil || s.Cmp(dist2) < 0 {
			best, dist2, active = p, s, face
		}
		if s.Sign() == 0 {
			break
		}
	}
	return
}

func (a *hullRatArithmetic) closestFace(face []hullRat3) (p hullRat3, ok bool) {
	if len(face) == 1 {
		return face[0], true
	}
	// Solve the normal equations for the affine span. Strictly positive
	// barycentric coefficients select an interior point of this face;
	// boundary and singular cases are covered by smaller faces.
	n := len(face) - 1
	var edges [3]hullRat3
	var m [3][4]*big.Rat
	for i := range n {
		edges[i] = a.minus(face[i+1], face[0])
	}
	zero, one := new(big.Rat), big.NewRat(1, 1)
	for i := range n {
		for j := range n {
			m[i][j] = a.dot(edges[i], edges[j])
		}
		m[i][n] = a.sub(zero, a.dot(edges[i], face[0]))
	}
	for i := range n {
		pivot := i
		for pivot < n && m[pivot][i].Sign() == 0 {
			pivot++
		}
		if a.err != nil || pivot == n {
			return p, false
		}
		m[i], m[pivot] = m[pivot], m[i]
		divisor := m[i][i]
		for j := i; j <= n; j++ {
			m[i][j] = a.quo(m[i][j], divisor)
		}
		for row := range n {
			if row == i || m[row][i].Sign() == 0 {
				continue
			}
			factor := m[row][i]
			for j := i; j <= n; j++ {
				m[row][j] = a.sub(m[row][j], a.mul(factor, m[i][j]))
			}
		}
	}
	remaining := one
	for i := range n {
		if a.err != nil || m[i][n].Sign() <= 0 {
			return p, false
		}
		remaining = a.sub(remaining, m[i][n])
	}
	if a.err != nil || remaining.Sign() <= 0 {
		return p, false
	}
	for i := range p {
		p[i] = a.mul(remaining, face[0][i])
		for j := range n {
			p[i] = a.add(p[i], a.mul(m[j][n], face[j+1][i]))
		}
	}
	return p, a.err == nil
}
