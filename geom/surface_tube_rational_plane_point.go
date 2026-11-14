package geom

import (
	"context"
	"errors"
	"math/big"
)

// EvaluateRationalPlane recovers a rational original-coordinate point when the
// opposite tube support is planar, though its parameter map need not be affine.
// Source iso roots and target triangular inverses are exact. Every accepted
// tuple is independently evaluated on both supports, lies strictly inside all
// dependent tube bounds (except an immutable accepted anchor), and is therefore
// the unique tube point. Unsupported algebraic coordinates remain unresolved.
// This does not infer absence from failed rational recovery or inverse attempts.
func (t VerifiedSurfaceTube) EvaluateRationalPlane(ctx context.Context, free *big.Rat, o TubeAlgebraicGraphEndpointOptions) (r TubeAlgebraicGraphPointCertificate, err error) {
	if ctx == nil || free == nil || t.a == nil || t.b == nil || t.axis < 0 || t.axis > 3 || t.tangentSign == 0 {
		return r, ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return r, e
	}
	opts, e := patchOptions(o.SurfacePatchOptions)
	if e != nil {
		return r, e
	}
	if o.MaxSubdivisions == 0 {
		o.MaxSubdivisions = 8192
	}
	if o.MaxEvents == 0 {
		o.MaxEvents = 256
	}
	if o.MaxSubdivisions < 1 || o.MaxEvents < 1 || o.MaxEvents > 4096 || !finite(o.Tolerance) || o.Tolerance < 0 {
		return r, ErrInvalid
	}
	if len(t.a.net) > opts.MaxPoles || len(t.b.net) > opts.MaxPoles {
		return r, ErrConvergence
	}
	budget := &polynomialBudget{maxOperations: opts.MaxOperations, maxBits: opts.MaxCoefficientBits}
	steps := 0
	defer func() {
		used := budget.operations
		if err != nil {
			r = TubeAlgebraicGraphPointCertificate{}
		}
		r.ExactOperations = used
	}()
	next := func() (ExactCurvePlaneEventOptions, error) {
		if e := ctx.Err(); e != nil {
			return ExactCurvePlaneEventOptions{}, e
		}
		n := opts.MaxOperations - budget.operations
		s := o.MaxSubdivisions - steps
		if n < 1 || s < 1 {
			return ExactCurvePlaneEventOptions{}, ErrConvergence
		}
		return ExactCurvePlaneEventOptions{MaxOperations: n, MaxCoefficientBits: opts.MaxCoefficientBits, MaxSubdivisions: s}, nil
	}
	rational := func(q *big.Rat) (*ExactAlgebraicRoot, error) {
		child, e := next()
		if e != nil {
			return nil, e
		}
		root, n, e := NewExactRationalRoot(ctx, q, child)
		budget.operations += n
		return root, e
	}
	if e := budget.check(free); e != nil {
		return r, e
	}
	source, target, owner := t.a, t.b, 0
	if t.axis >= 2 {
		source, target, owner = t.b, t.a, 1
	}
	d := source.Domain()
	ranges := [2][2]float64{{d.UMin, d.UMax}, {d.VMin, d.VMax}}
	fixed := t.axis % 2
	if free.Cmp(new(big.Rat).SetFloat64(ranges[fixed][0])) < 0 || free.Cmp(new(big.Rat).SetFloat64(ranges[fixed][1])) > 0 {
		return r, ErrDomain
	}
	plane, e := tubePlanarEquation(ctx, target, budget)
	if e != nil {
		return r, e
	}
	origin, direction := [2]*big.Rat{new(big.Rat), new(big.Rat)}, [2]*big.Rat{new(big.Rat), new(big.Rat)}
	origin[fixed] = new(big.Rat).Set(free)
	direction[1-fixed] = big.NewRat(1, 1)
	lo, e := rational(new(big.Rat).SetFloat64(ranges[1-fixed][0]))
	if e != nil {
		return r, e
	}
	hi, e := rational(new(big.Rat).SetFloat64(ranges[1-fixed][1]))
	if e != nil {
		return r, e
	}
	child, e := next()
	if e != nil {
		return r, e
	}
	compose := o.SurfaceCurveAgreementOptions
	compose.SurfacePatchOptions = opts
	compose.MaxOperations = child.MaxOperations
	compose.MaxSubdivisions = child.MaxSubdivisions
	section, e := source.BuildAffineSection(ctx, origin, direction, lo, hi, compose)
	budget.operations += section.ExactOperations
	steps += section.Subdivisions
	if e != nil {
		return r, e
	}
	if !section.Complete || len(section.Cells) > o.MaxEvents {
		return r, ErrConvergence
	}
	events := 0
	for _, cell := range section.Cells {
		child, e = next()
		if e != nil {
			return r, e
		}
		contacts, e := cell.IntersectRawPlane(ctx, plane, child)
		budget.operations += contacts.Operations
		steps += contacts.Subdivisions
		if e != nil {
			return r, e
		}
		if !contacts.Complete || contacts.Coincident {
			return r, ErrConvergence
		}
		events += len(contacts.Events)
		if events > o.MaxEvents {
			return r, ErrConvergence
		}
		for _, event := range contacts.Events {
			if event.Multiplicity() != 1 {
				continue
			}
			child, e = next()
			if e != nil {
				return r, e
			}
			kind, e := event.Root().Classify(ctx, child)
			budget.operations += kind.Operations
			if e != nil {
				return r, e
			}
			if kind.Kind != ExactRootKindRational {
				continue
			}
			child, e = next()
			if e != nil {
				return r, e
			}
			model, e := event.EvaluateRationalWitness(ctx, kind.Rational, child)
			budget.operations += model.ExactOperations
			steps += model.Subdivisions
			if e != nil {
				return r, e
			}
			var inverse TriangularSurfaceInverse
			found := false
			for _, axes := range [][2]int{{0, 1}, {0, 2}, {1, 2}, {1, 0}, {2, 0}, {2, 1}} {
				for fixedTarget := 0; fixedTarget < 2; fixedTarget++ {
					child, e = next()
					if e != nil {
						return r, e
					}
					inv, e := InvertTriangularSurfacePointRational(ctx, target, model.Point, TriangularSurfaceInverseOptions{RationalSurfaceInverseOptions: RationalSurfaceInverseOptions{ProjectionAxes: axes, MaxOperations: child.MaxOperations, MaxCoefficientBits: child.MaxCoefficientBits}, FixedParameterAxis: fixedTarget, MaxRootSubdivisions: child.MaxSubdivisions})
					budget.operations += inv.ExactOperations
					steps += inv.RootSubdivisions
					if e != nil {
						if errors.Is(e, ErrConvergence) || errors.Is(e, ErrDegenerate) {
							continue
						}
						return r, e
					}
					if inv.Complete && inv.Found && inv.ModelPointValidated && inv.Parameters[0] != nil && inv.Parameters[1] != nil {
						inverse = inv
						found = true
						break
					}
				}
				if found {
					break
				}
			}
			if !found {
				continue
			}
			var tuple [4]*big.Rat
			tuple[2*owner+fixed] = free
			tuple[2*owner+1-fixed] = kind.Rational
			tuple[2*(1-owner)] = inverse.Parameters[0]
			tuple[2*(1-owner)+1] = inverse.Parameters[1]
			var roots [4]*ExactAlgebraicRoot
			for i, q := range tuple {
				roots[i], e = rational(q)
				if e != nil {
					return r, e
				}
			}
			anchor, e := t.matchesAnchorRoots(ctx, roots, budget, o.MaxSubdivisions-steps)
			if e != nil {
				return r, e
			}
			da, db := t.a.Domain(), t.b.Domain()
			boxes := [4][2]float64{{da.UMin, da.UMax}, {da.VMin, da.VMax}, {db.UMin, db.UMax}, {db.VMin, db.VMax}}
			inside := true
			var bounds [4][2]float64
			for i, q := range tuple {
				for side, bound := range boxes[i] {
					cmp := q.Cmp(new(big.Rat).SetFloat64(bound))
					if side == 0 && cmp < 0 || side == 1 && cmp > 0 || i != t.axis && cmp == 0 && !anchor {
						inside = false
					}
				}
				bounds[i] = [2]float64{max(boxes[i][0], rationalFloatBound(q, true)), min(boxes[i][1], rationalFloatBound(q, false))}
			}
			if !inside {
				continue
			}
			for i, p := range [2]*SurfacePatch{t.a, t.b} {
				child, e = next()
				if e != nil {
					return r, e
				}
				jet, e := p.EvaluateRational(ctx, tuple[2*i], tuple[2*i+1], SurfacePatchEvaluationOptions{MaxOperations: child.MaxOperations, MaxCoefficientBits: child.MaxCoefficientBits})
				budget.operations += jet.ExactOperations
				if e != nil {
					return r, e
				}
				for j, q := range jet.Point {
					if q.Cmp(model.Point[j]) != 0 {
						return r, ErrConvergence
					}
				}
			}
			r.Parameters = roots
			r.ModelEvent = event
			r.SourceIsoMultiplicity = 1
			r.SourceIsoTransverse = true
			r.ParameterBoundsA = UVBox{Min: Vec2{X: bounds[0][0], Y: bounds[1][0]}, Max: Vec2{X: bounds[0][1], Y: bounds[1][1]}}
			r.ParameterBoundsB = UVBox{Min: Vec2{X: bounds[2][0], Y: bounds[3][0]}, Max: Vec2{X: bounds[2][1], Y: bounds[3][1]}}
			r.PointBounds = Box{Min: Vec3{X: rationalFloatBound(model.Point[0], true), Y: rationalFloatBound(model.Point[1], true), Z: rationalFloatBound(model.Point[2], true)}, Max: Vec3{X: rationalFloatBound(model.Point[0], false), Y: rationalFloatBound(model.Point[1], false), Z: rationalFloatBound(model.Point[2], false)}}
			r.Proven = true
			return r, nil
		}
	}
	return r, ErrConvergence
}

// Exact corner proposals are accepted only after every original homogeneous
// coefficient satisfies the same nonzero equation; no affine UV map is assumed.
func tubePlanarEquation(ctx context.Context, p *SurfacePatch, b *polynomialBudget) (plane [4]*big.Rat, err error) {
	d := p.Domain()
	var q [3][3]*big.Rat
	for i, uv := range [3][2]float64{{d.UMin, d.VMin}, {d.UMax, d.VMin}, {d.UMin, d.VMax}} {
		left := b.maxOperations - b.operations
		if left < 1 {
			return plane, ErrConvergence
		}
		jet, e := p.EvaluateRational(ctx, new(big.Rat).SetFloat64(uv[0]), new(big.Rat).SetFloat64(uv[1]), SurfacePatchEvaluationOptions{MaxOperations: left, MaxCoefficientBits: b.maxBits})
		b.operations += jet.ExactOperations
		if e != nil {
			return plane, e
		}
		q[i] = jet.Point
	}
	var u, v [3]*big.Rat
	for i := range u {
		u[i] = new(big.Rat).Sub(q[1][i], q[0][i])
		v[i] = new(big.Rat).Sub(q[2][i], q[0][i])
	}
	plane[3] = new(big.Rat)
	nonzero := false
	for i := range u {
		j, k := (i+1)%3, (i+2)%3
		plane[i] = new(big.Rat).Sub(new(big.Rat).Mul(u[j], v[k]), new(big.Rat).Mul(u[k], v[j]))
		if e := b.check(plane[i]); e != nil {
			return plane, e
		}
		nonzero = nonzero || plane[i].Sign() != 0
		plane[3].Sub(plane[3], new(big.Rat).Mul(plane[i], q[0][i]))
	}
	if !nonzero {
		return plane, ErrDegenerate
	}
	if e := b.check(plane[3]); e != nil {
		return plane, e
	}
	for _, h := range p.net {
		if e := ctx.Err(); e != nil {
			return plane, e
		}
		sum := new(big.Rat)
		for i, c := range h {
			if e := b.check(c); e != nil {
				return plane, e
			}
			sum.Add(sum, new(big.Rat).Mul(plane[i], c))
			if e := b.check(sum); e != nil {
				return plane, e
			}
		}
		if sum.Sign() != 0 {
			return plane, ErrConvergence
		}
	}
	return plane, nil
}
