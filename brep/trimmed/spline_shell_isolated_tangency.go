package trimmed

import (
	"context"
	"errors"
	"fmt"
	"math/big"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

// SplineShellTangencyOptions selects one original face pair and its affine
// world projection. The supplied pair is a candidate; all other original
// face pairs must be independently separated. Zero projection means XY.
type SplineShellTangencyOptions struct {
	Faces      [2]int
	Projection [2]int
	Shells     SplineShellBooleanOptions
	Contact    geom.GraphPointContactDiscoveryOptions
}

// SplineShellTangency retains the exactly proved original graph point and
// both original chart coordinates. StrictOrder is the sign of A minus B
// in the residual model coordinate everywhere away from the point.
type SplineShellTangency struct {
	Faces, Projection [2]int
	Point             [3]*big.Rat
	UV                [2][2]*big.Rat
	StrictOrder       int
	SeparatedPairs    int
	ExactOperations   int
	// Additional contains the remaining contacts on the same original face
	// pair, ordered by the first input's U and V. Every point is isolated.
	Additional []SplineShellTangencyPoint
}

// SplineShellTangencyPoint gives each isolated contact on the original
// supports. The XYZ and UV rationals are owned by the caller.
type SplineShellTangencyPoint struct {
	Point [3]*big.Rat
	UV    [2][2]*big.Rat
}

// CombineExternallyTangentSplineShells handles isolated definite or
// higher-order contacts on one graph pair of independently embedded rectangular
// spline shells. Opposite outward graph normals and strict graph order
// over the complete common projection prove no local material overlap at
// the contacts. With all other face pairs
// disjoint and original vertex witnesses outside either shell, common is
// empty and cut retains the stock. Union at isolated points is not a native
// manifold and returns ErrConvergence.
func CombineExternallyTangentSplineShells(ctx context.Context, a, b Solid, op parametric.BooleanOp,
	o SplineShellTangencyOptions) (Solid, SplineShellCombineHistory, SplineShellTangency, error) {
	var history SplineShellCombineHistory
	var empty SplineShellTangency
	if ctx == nil || op > parametric.Difference || o.Shells.MaxRays < 0 || o.Shells.MaxRays > 12 ||
		o.Contact.MaxOperations < 0 || o.Contact.MaxCoefficientBits != 0 && o.Contact.MaxCoefficientBits < 64 {
		return Solid{}, history, empty, geom.ErrInvalid
	}
	solids := [2]Solid{a, b}
	var all [2][]splineEmbeddingFace
	if op == parametric.Union {
		return Solid{}, history, empty, geom.ErrConvergence
	}
	for i, s := range solids {
		if o.Faces[i] < 0 || o.Faces[i] >= len(s.faces) {
			return Solid{}, history, empty, geom.ErrInvalid
		}
		if _, e := s.validateSplineRectangleEmbedding(ctx, o.Shells.Embedding, &all[i]); e != nil {
			return Solid{}, history, empty, e
		}
	}
	return combineTangentSplineShellPair(ctx, solids, all, op, o, false)
}

// separated is true only if the caller has already proved every unselected
// face pair disjoint on the exact original embedding patches.
func combineTangentSplineShellPair(ctx context.Context, solids [2]Solid, all [2][]splineEmbeddingFace,
	op parametric.BooleanOp, o SplineShellTangencyOptions, separated bool) (Solid, SplineShellCombineHistory, SplineShellTangency, error) {
	var history SplineShellCombineHistory
	var empty SplineShellTangency
	axes := o.Projection
	if axes == [2]int{} {
		axes = [2]int{0, 1}
	}
	if axes[0] < 0 || axes[0] > 2 || axes[1] < 0 || axes[1] > 2 || axes[0] == axes[1] {
		return Solid{}, history, empty, geom.ErrInvalid
	}
	if o.Shells.MaxRays == 0 {
		o.Shells.MaxRays = 12
	}
	if o.Shells.MaxRays < 0 || o.Shells.MaxRays > 12 {
		return Solid{}, history, empty, geom.ErrInvalid
	}
	if o.Contact.MaxOperations == 0 {
		o.Contact.MaxOperations = 8000000
	}
	if len(all[0])*len(all[1]) > 256 {
		return Solid{}, history, empty, geom.ErrConvergence
	}
	proof := SplineShellTangency{Faces: o.Faces, Projection: axes}
	limit := func() geom.SurfacePatchOptions {
		return geom.SurfacePatchOptions{MaxOperations: o.Contact.MaxOperations - proof.ExactOperations, MaxCoefficientBits: o.Contact.MaxCoefficientBits}
	}
	order := [3]int{axes[0], axes[1], 3 - axes[0] - axes[1]}
	var patches [2]*geom.SurfacePatch
	sign := [2]int{}
	for i := range patches {
		p := all[i][o.Faces[i]].patch
		if axes != [2]int{0, 1} {
			var m geom.Affine
			for j, k := range order {
				m[j][k] = 1
			}
			if limit().MaxOperations < 1 {
				return Solid{}, history, empty, geom.ErrConvergence
			}
			var used int
			var e error
			p, used, e = p.TransformedWithWorkLimit(m, limit().MaxOperations)
			proof.ExactOperations += used
			if e != nil {
				return Solid{}, history, empty, e
			}
		}
		if limit().MaxOperations < 1 {
			return Solid{}, history, empty, geom.ErrConvergence
		}
		g, used, e := geom.NewAffineProjectedGraphWithWork(ctx, p, [2]int{0, 1}, limit())
		proof.ExactOperations += used
		if e != nil {
			return Solid{}, history, empty, e
		}
		sign[i] = g.Orientation() * projectedAxisParity(axes)
		if solids[i].faces[o.Faces[i]].Reversed {
			sign[i] = -sign[i]
		}
		patches[i] = p
	}
	if sign[0] == sign[1] || limit().MaxOperations < 1 {
		return Solid{}, history, empty, geom.ErrConvergence
	}
	align := func() (*geom.AffineXYChartAlignment, error) {
		p := limit()
		if p.MaxOperations < 1 {
			return nil, geom.ErrConvergence
		}
		a, used, e := geom.AlignAffineXYChartsWithWork(ctx, patches[0], patches[1], p)
		proof.ExactOperations += used
		return a, e
	}
	alignment, e := align()
	if errors.Is(e, geom.ErrDomain) {
		// Discard only the part of either original chart with no
		// possible common projection. Analyze the full shared rectangle.
		p := limit()
		if p.MaxOperations < 1 {
			return Solid{}, history, empty, geom.ErrConvergence
		}
		overlap, e := geom.ClipAffineXYGraphOverlap(ctx, patches[0], patches[1], p)
		proof.ExactOperations += min(overlap.ExactOperations, p.MaxOperations)
		if e != nil {
			return Solid{}, history, empty, e
		}
		if overlap.Disjoint {
			return Solid{}, history, empty, geom.ErrConvergence
		}
		for i, d := range [2]geom.ParameterRectangle{overlap.A, overlap.B} {
			if patches[i].Domain() == d {
				continue
			}
			if limit().MaxOperations < 1 {
				return Solid{}, history, empty, geom.ErrConvergence
			}
			var used int
			patches[i], used, e = patches[i].TrimWithWorkLimit(d, limit().MaxOperations)
			proof.ExactOperations += used
			if e != nil {
				return Solid{}, history, empty, e
			}
		}
		alignment, e = align()
		if e != nil {
			return Solid{}, history, empty, e
		}
	} else if e != nil {
		return Solid{}, history, empty, e
	}
	o.Contact.MaxOperations = limit().MaxOperations
	if o.Contact.MaxOperations < 1 {
		return Solid{}, history, empty, geom.ErrConvergence
	}
	point, e := geom.DiscoverSplineGraphPointContact(ctx, patches[0], alignment.TargetOnSource(), o.Contact)
	proof.ExactOperations += point.ExactOperations
	var parameters [2]*big.Rat
	var xyz [3]*big.Rat
	var knots []splineKnotTangency
	side := 0
	if e == nil && point.Contact.Proven && point.Contact.BoundaryMask == 0 {
		parameters, xyz, side = point.Contact.Parameters, point.Contact.Point, point.Contact.Side
	} else if errors.Is(e, geom.ErrConvergence) || errors.Is(e, geom.ErrDomain) {
		// A strict coupled quartic can lose its Hessian at the contact.
		// Accept it only after recovering and proving the complete exact
		// numerator and retaining a strictly interior witness.
		p := o.Contact.GraphPointContactOptions
		p.MaxOperations = limit().MaxOperations
		if p.MaxOperations < 1 {
			return Solid{}, history, empty, geom.ErrConvergence
		}
		q, usedErr := geom.DiscoverCoupledQuarticGraphPointContact(ctx, patches[0], alignment.TargetOnSource(), p)
		proof.ExactOperations += q.ExactOperations
		if usedErr == nil && q.Proven {
			parameters, xyz, side = q.Parameters, q.Point, q.Side
		} else if errors.Is(usedErr, geom.ErrConvergence) || errors.Is(usedErr, geom.ErrDomain) {
			// A separable exact height numerator may vanish at different
			// even orders in the two directions or at a common knot. The
			// univariate root proofs still establish strict global order.
			p.MaxOperations = limit().MaxOperations
			if p.MaxOperations < 1 {
				return Solid{}, history, empty, geom.ErrConvergence
			}
			sep, err := geom.DiscoverSeparableSplineGraphPointContact(ctx, patches[0], alignment.TargetOnSource(), geom.SeparableGraphDiscoveryOptions{
				SeparableGraphContactOptions: geom.SeparableGraphContactOptions{GraphPointContactOptions: p},
				MaxCandidates:                o.Contact.MaxCandidates,
			})
			proof.ExactOperations += sep.ExactOperations
			if err == nil && sep.Contact.Proven {
				parameters, xyz, side = sep.Contact.Parameters, sep.Contact.Point, sep.Contact.Side
			} else if errors.Is(err, geom.ErrConvergence) || errors.Is(err, geom.ErrDomain) {
				p.MaxOperations = limit().MaxOperations
				if p.MaxOperations < 1 {
					return Solid{}, history, empty, geom.ErrConvergence
				}
				var used int
				knots, side, used, e = isolatedSplineKnotTangencies(ctx, patches[0], alignment.TargetOnSource(), p)
				proof.ExactOperations += used
				if e != nil {
					return Solid{}, history, empty, e
				}
				parameters, xyz = knots[0].uv, knots[0].xyz
			} else if err != nil {
				return Solid{}, history, empty, err
			} else {
				return Solid{}, history, empty, geom.ErrConvergence
			}
		} else if usedErr != nil {
			return Solid{}, history, empty, usedErr
		} else {
			return Solid{}, history, empty, geom.ErrConvergence
		}
		box := patches[0].Domain()
		for i, b := range [2][2]float64{{box.UMin, box.UMax}, {box.VMin, box.VMax}} {
			if parameters[i].Cmp(ratInset(b[0])) <= 0 || parameters[i].Cmp(ratInset(b[1])) >= 0 {
				return Solid{}, history, empty, geom.ErrConvergence
			}
		}
		e = nil
	}
	if e != nil {
		return Solid{}, history, empty, e
	}
	if side != -sign[0] {
		return Solid{}, history, empty, geom.ErrConvergence
	}
	for i, f := range all[0] {
		for j, g := range all[1] {
			if i == o.Faces[0] && j == o.Faces[1] {
				continue
			}
			if !separated {
				if e := proveSplinePatchPairDisjoint(ctx, f.patch, g.patch, o.Shells.Separation); e != nil {
					return Solid{}, history, empty, fmt.Errorf("trimmed: tangent shell pair %d,%d: %w", i, j, e)
				}
			}
			proof.SeparatedPairs++
		}
	}
	// At an interior contact the source vertex is always disjoint from
	// the other boundary. Each connected shell remains outside it.
	for i, s := range solids {
		j := i ^ 1
		inside, e := classifySeparatedSplineWitness(ctx, solids[j], splineShellPatchBounds(all[j]), s.vertices[0], o.Shells)
		if e != nil || inside {
			if e == nil {
				e = geom.ErrConvergence
			}
			return Solid{}, history, empty, e
		}
	}
	mapping, e := alignment.MapCoefficients()
	if e != nil {
		return Solid{}, history, empty, e
	}
	tangentPoint := func(parameters [2]*big.Rat, xyz [3]*big.Rat) (p SplineShellTangencyPoint) {
		for i, axis := range order {
			p.Point[axis] = new(big.Rat).Set(xyz[i])
		}
		p.UV[0] = parameters
		for i, row := range mapping {
			p.UV[1][i] = new(big.Rat).Set(row[2])
			for j, q := range parameters {
				p.UV[1][i].Add(p.UV[1][i], new(big.Rat).Mul(row[j], q))
			}
		}
		return p
	}
	p := tangentPoint(parameters, xyz)
	proof.Point, proof.UV = p.Point, p.UV
	if len(knots) > 1 {
		for _, q := range knots[1:] {
			proof.Additional = append(proof.Additional, tangentPoint(q.uv, q.xyz))
		}
	}
	proof.StrictOrder = side
	native, history := regularizedTouchResult(solids[0], solids[1], op)
	history.Method = "isolated-graph-tangency"
	history.EdgesA, history.EdgesB = make([][]int, len(solids[0].edges)), make([][]int, len(solids[1].edges))
	if op == parametric.Difference {
		for i := range history.EdgesA {
			history.EdgesA[i] = []int{i}
		}
	}
	return native, history, proof, nil
}
