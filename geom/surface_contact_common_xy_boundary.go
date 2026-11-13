package geom

import (
	"context"
	"errors"
	"math/big"
)

// commonXYBoundaryContactBounds is a sufficient exact confinement proof for
// equal chart rectangles. Projected injectivity forces equal UV at any contact.
// A weakly signed height numerator can then vanish only on its full zero
// Bernstein sides and zero corners. No floating near-zero test is used.
func commonXYBoundaryContactBounds(ctx context.Context, a, c *SurfacePatch, maxOperations, maxBits int) (bounds [4][2]float64, applied, excluded bool, operations int, err error) {
	if e := ctx.Err(); e != nil {
		return bounds, false, false, 0, e
	}
	if maxOperations < 1 {
		return bounds, false, false, 0, ErrConvergence
	}
	if a.Domain() != c.Domain() {
		return
	}
	b := &polynomialBudget{maxOperations: maxOperations, maxBits: maxBits}
	defer func() { operations = b.operations }()
	p, e := orderingDifference(ctx, a, c, 256, b)
	if e != nil {
		if errors.Is(e, ErrInvalid) {
			return bounds, false, false, 0, nil
		}
		return bounds, false, false, 0, e
	}
	if _, _, multi := patchSplitChoice(p); multi {
		return
	}
	positive, negative, nonzero := true, true, false
	for _, h := range p.net {
		sign := h[2].Sign()
		positive = positive && sign >= 0
		negative = negative && sign <= 0
		nonzero = nonzero || sign != 0
	}
	if !nonzero || (!positive && !negative) {
		return
	}
	// This local theorem requires injectivity; equality of XY control functions
	// alone does not rule out off-diagonal parameters on a self-overlapping chart.
	remaining := maxOperations - b.operations
	if remaining < 1 {
		return bounds, false, false, 0, ErrConvergence
	}
	graph, e := CertifySurfaceInjectivity(a, SurfaceInjectivityOptions{ProjectionAxes: [2]int{0, 1}, MaxCells: 256, MaxOperations: remaining, MaxCoefficientBits: maxBits})
	b.operations += graph.ExactOperations
	if errors.Is(e, ErrConvergence) && b.operations < maxOperations {
		// Failure to prove projected injectivity is only an unsupported
		// shortcut; preserve the caller's other confinement methods.
		return bounds, false, false, 0, nil
	}
	if e != nil {
		return bounds, false, false, 0, e
	}
	if !graph.ProvenInjective {
		return
	}
	d := p.Domain()
	found := false
	var uv [2][2]float64
	include := func(u, v float64) {
		if !found {
			uv = [2][2]float64{{u, u}, {v, v}}
			found = true
		} else {
			uv[0] = [2]float64{min(uv[0][0], u), max(uv[0][1], u)}
			uv[1] = [2]float64{min(uv[1][0], v), max(uv[1][1], v)}
		}
	}
	corners := []struct {
		index int
		u, v  float64
	}{{0, d.UMin, d.VMin}, {(p.countU - 1) * p.countV, d.UMax, d.VMin}, {len(p.net) - 1, d.UMax, d.VMax}, {p.countV - 1, d.UMin, d.VMax}}
	for _, corner := range corners {
		if p.net[corner.index][2].Sign() == 0 {
			include(corner.u, corner.v)
		}
	}
	for edge := SurfaceEdge(0); edge < 4; edge++ {
		zero := true
		for _, index := range p.edgeIndices(edge) {
			zero = zero && p.net[index][2].Sign() == 0
		}
		if !zero {
			continue
		}
		switch edge {
		case SurfaceEdgeUMin:
			include(d.UMin, d.VMin)
			include(d.UMin, d.VMax)
		case SurfaceEdgeUMax:
			include(d.UMax, d.VMin)
			include(d.UMax, d.VMax)
		case SurfaceEdgeVMin:
			include(d.UMin, d.VMin)
			include(d.UMax, d.VMin)
		case SurfaceEdgeVMax:
			include(d.UMin, d.VMax)
			include(d.UMax, d.VMax)
		}
	}
	if e := b.check(big.NewRat(0, 1)); e != nil {
		return bounds, false, false, 0, e
	}
	if !found {
		return bounds, true, true, 0, nil
	}
	return [4][2]float64{uv[0], uv[1], uv[0], uv[1]}, true, false, 0, nil
}
