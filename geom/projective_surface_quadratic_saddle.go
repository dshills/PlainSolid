package geom

import (
	"context"
	"math/big"
)

// ProjectiveQuadraticSaddleOptions bounds the construction of the original
// residual atlas and the exact certification of one cell. The limits for
// saddle operations and root subdivisions default to 2,000,000 and 8,192.
type ProjectiveQuadraticSaddleOptions struct {
	Atlas                              GeneralSurfaceHeightOptions
	MaxOperations, MaxRootSubdivisions int
}

// ProjectiveQuadraticSaddleRay ends on the certified cell boundary. Side
// entries are -1 at a lower side, +1 at an upper side, and 0 otherwise.
// At a corner both entries are nonzero.
type ProjectiveQuadraticSaddleRay struct {
	End  ProjectiveContactVertex
	Side [2]int
}

// ProjectiveQuadraticSaddle proves the entire contact set within one original
// common cell: two distinct straight lines in the local common clock meet
// at Center. Each Rays row contains opposite halves of one line, ordered
// by decreasing/increasing coordinate along that row's LineAxis.
// The model images can be curved and may continue into unclassified cells.
type ProjectiveQuadraticSaddle struct {
	Cell                                    ProjectiveSurfaceHeightCell
	Center                                  ProjectiveContactVertex
	LineAxis                                [2]int
	Rays                                    [2][2]ProjectiveQuadraticSaddleRay
	Proven                                  bool
	AtlasOperations, Operations, RootSplits int
}

// CertifyProjectiveSurfaceQuadraticSaddle selects a common knot cell in the
// reproducible order of BuildProjectiveSurfaceHeightDifference. It proves an
// interior zero saddle of the exact homogeneous residual and clips all four
// branches against the cell's exact sides. Nonquadratic, nonzero critical
// levels and saddles on cell boundaries leave the contact set unresolved.
func CertifyProjectiveSurfaceQuadraticSaddle(ctx context.Context, source, target *SurfacePatch, axes [2]int,
	cell int, o ProjectiveQuadraticSaddleOptions) (r ProjectiveQuadraticSaddle, err error) {
	if ctx == nil || cell < 0 {
		return r, ErrInvalid
	}
	limits, err := graphRootOptions(ExactCurvePlaneEventOptions{
		MaxOperations: o.MaxOperations, MaxCoefficientBits: o.Atlas.MaxCoefficientBits, MaxSubdivisions: o.MaxRootSubdivisions})
	if err != nil {
		return r, err
	}
	atlas, err := BuildProjectiveSurfaceHeightDifference(ctx, source, target, axes, o.Atlas)
	r.AtlasOperations = atlas.ExactOperations
	if err != nil {
		return r, err
	}
	if cell >= len(atlas.Cells) {
		return r, ErrDomain
	}
	r, err = certifyProjectiveQuadraticSaddleCell(ctx, atlas.Cells[cell], atlas.Overlap, limits)
	r.AtlasOperations = atlas.ExactOperations
	return r, err
}

func certifyProjectiveQuadraticSaddleCell(ctx context.Context, c ProjectiveSurfaceHeightCell, overlap ProjectiveSurfaceGraphOverlap,
	limits ExactCurvePlaneEventOptions) (r ProjectiveQuadraticSaddle, err error) {
	p := c.Difference
	b := &polynomialBudget{maxOperations: limits.MaxOperations, maxBits: limits.MaxCoefficientBits}
	defer func() { r.Operations = b.operations }()
	power, err := tensorPowerCoefficients(p.net, p.countU, p.countV, 2, b)
	if err != nil {
		return r, err
	}
	zero, one, two := new(big.Rat), big.NewRat(1, 1), big.NewRat(2, 1)
	var coefficient [3][3]*big.Rat
	for i := range coefficient {
		for j := range coefficient[i] {
			coefficient[i][j] = zero
		}
	}
	for k, q := range power {
		i, j := k/p.countV, k%p.countV
		if i+j <= 2 {
			coefficient[i][j] = q
		} else if q.Sign() != 0 {
			return r, ErrConvergence
		}
	}
	a := boundedRatArithmetic{b: b}
	du, dv := coefficient[1][0], coefficient[0][1]
	h00, h01, h11 := a.mul(two, coefficient[2][0]), coefficient[1][1], a.mul(two, coefficient[0][2])
	det := a.sub(a.mul(h00, h11), a.mul(h01, h01))
	if a.err != nil {
		return r, a.err
	}
	if det.Sign() >= 0 {
		return r, ErrConvergence
	}
	local := [2]*big.Rat{
		a.quo(a.sub(a.mul(h01, dv), a.mul(h11, du)), det),
		a.quo(a.sub(a.mul(h01, du), a.mul(h00, dv)), det),
	}
	// At a stationary point the exact value is constant + dot(linear,local)/2.
	value := a.add(coefficient[0][0], a.quo(a.add(a.mul(du, local[0]), a.mul(dv, local[1])), two))
	if a.err != nil {
		return r, a.err
	}
	if value.Sign() != 0 || local[0].Sign() <= 0 || local[0].Cmp(one) >= 0 ||
		local[1].Sign() <= 0 || local[1].Cmp(one) >= 0 {
		return r, ErrConvergence
	}
	intervals := [2]ExactParameterInterval{c.Common.U, c.Common.V}
	vertex := func(station [2]*ExactGraphLineRoot) (ProjectiveContactVertex, error) {
		var common [2]*ExactAlgebraicRoot
		for i, root := range station {
			d := intervals[i]
			width := a.sub(d.Upper, d.Lower)
			if a.err != nil {
				return ProjectiveContactVertex{}, a.err
			}
			mapped, err := affineMapRoot(root, d.Lower, width, b)
			if err != nil {
				return ProjectiveContactVertex{}, err
			}
			common[i] = &ExactAlgebraicRoot{root: mapped}
		}
		return projectiveContactVertex(common, overlap, b)
	}
	rational := func(q *big.Rat) *ExactGraphLineRoot {
		return &ExactGraphLineRoot{polynomial: rationalPolynomial{new(big.Rat).Neg(q), one}, bounds: [2]*big.Rat{q, q}}
	}
	center, err := vertex([2]*ExactGraphLineRoot{rational(local[0]), rational(local[1])})
	if err != nil {
		return r, err
	}
	var divider *big.Rat
	lineAxis := [2]int{0, 0}
	if h11.Sign() == 0 {
		lineAxis[0] = 1 // One factor is the vertical line through the center.
	} else {
		divider = a.quo(a.sub(zero, h01), h11) // Between the distinct slopes.
		if a.err != nil {
			return r, a.err
		}
	}
	var rays [2][2]ProjectiveQuadraticSaddleRay
	for fixedAxis := 0; fixedAxis < 2; fixedAxis++ {
		free := 1 - fixedAxis
		for side, x := range [2]*big.Rat{zero, one} {
			poly := make(rationalPolynomial, 3)
			for i := range poly {
				poly[i] = new(big.Rat)
				for j := 0; i+j <= 2; j++ {
					if side == 0 && j > 0 {
						continue
					}
					if fixedAxis == 0 {
						poly[i] = a.add(poly[i], coefficient[j][i])
					} else {
						poly[i] = a.add(poly[i], coefficient[i][j])
					}
				}
			}
			if a.err != nil {
				return r, a.err
			}
			poly = trimPolynomial(poly)
			if len(poly) == 0 || limits.MaxSubdivisions-r.RootSplits < 1 {
				return r, ErrConvergence
			}
			// A closed bracket for another root must exclude exact roots at
			// the rectangle corners and at prior bisection boundaries.
			steps, err := isolatePolynomialRoots(poly, zero, one, b, limits.MaxSubdivisions-r.RootSplits,
				func(lo, hi *big.Rat, order int) (bool, error) {
					if err := ctx.Err(); err != nil {
						return false, err
					}
					if order != 1 {
						return false, ErrConvergence
					}
					if len(poly) == 2 && lo.Cmp(hi) != 0 {
						// A linear original side root is exactly rational even
						// if the isolator accepted a larger unique-root bracket.
						q := a.quo(a.sub(zero, poly[0]), poly[1])
						if a.err != nil {
							return false, a.err
						}
						lo, hi = q, q
					}
					if lo.Cmp(hi) != 0 {
						for _, q := range [2]*big.Rat{lo, hi} {
							y, err := polynomialValue(poly, q, b)
							if err != nil || y.Sign() == 0 {
								return false, err
							}
						}
					}
					// The first axis already owns both copies of a corner.
					corner := 0
					if lo.Cmp(hi) == 0 {
						if lo.Sign() == 0 {
							corner = -1
						} else if lo.Cmp(one) == 0 {
							corner = 1
						}
					}
					if fixedAxis == 1 && corner != 0 {
						return true, nil
					}
					// Classify the endpoint inside the isolation callback so
					// the same subdivision budget also proves the ray order.
					var delta [2]exactInterval
					delta[fixedAxis] = exactInterval{a.sub(x, local[fixedAxis]), a.sub(x, local[fixedAxis])}
					delta[free] = exactInterval{a.sub(lo, local[free]), a.sub(hi, local[free])}
					if a.err != nil {
						return false, a.err
					}
					line := 0
					if divider == nil {
						switch {
						case delta[0].lo.Sign() == 0 && delta[0].hi.Sign() == 0:
						case delta[0].lo.Sign()*delta[0].hi.Sign() > 0:
							line = 1
						default:
							// Authenticate an exact vertical factor even if its
							// rational root is not hit by binary subdivision.
							atCenter, err := polynomialValue(poly, local[0], b)
							if err != nil {
								return false, err
							}
							if free != 0 || atCenter.Sign() != 0 {
								return false, nil
							}
							lo, hi = local[0], local[0]
						}
					} else {
						if delta[0].lo.Sign()*delta[0].hi.Sign() <= 0 {
							return false, nil
						}
						low, high := a.mul(divider, delta[0].lo), a.mul(divider, delta[0].hi)
						if divider.Sign() < 0 {
							low, high = high, low
						}
						offset := exactInterval{a.sub(delta[1].lo, high), a.sub(delta[1].hi, low)}
						if a.err != nil {
							return false, a.err
						}
						if offset.lo.Sign()*offset.hi.Sign() <= 0 {
							return false, nil
						}
						if delta[0].lo.Sign() == offset.lo.Sign() {
							line = 1
						}
					}
					along := delta[lineAxis[line]]
					if along.lo.Sign()*along.hi.Sign() <= 0 {
						return false, nil
					}
					direction := 0
					if along.lo.Sign() > 0 {
						direction = 1
					}
					if rays[line][direction].End.Common[0] != nil {
						return false, ErrConvergence
					}
					var station [2]*ExactGraphLineRoot
					station[fixedAxis] = rational(x)
					station[free] = &ExactGraphLineRoot{polynomial: poly, bounds: [2]*big.Rat{new(big.Rat).Set(lo), new(big.Rat).Set(hi)}}
					end, err := vertex(station)
					if err != nil {
						return false, err
					}
					var sides [2]int
					sides[fixedAxis], sides[free] = 2*side-1, corner
					rays[line][direction] = ProjectiveQuadraticSaddleRay{End: end, Side: sides}
					return true, nil
				})
			r.RootSplits += steps
			if err != nil {
				return r, err
			}
		}
	}
	for _, line := range rays {
		for _, ray := range line {
			if ray.End.Common[0] == nil {
				return r, ErrConvergence
			}
		}
	}
	if err := ctx.Err(); err != nil {
		return r, err
	}
	r.Cell, r.Center, r.LineAxis, r.Rays, r.Proven = c, center, lineAxis, rays, true
	return r, nil
}
