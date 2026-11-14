package trimmed

import (
	"context"
	"math/big"

	"plainsolid/geom"
)

// AlgebraicGraphStockCoedge is one oriented edge of a retained original face
// loop. UV is expressed on the original stock chart. For an inherited stock
// perimeter edge, OriginalEdge is its initial stock ID and Cap is -1. For a
// cap attachment, Cap and Owner locate its proved cap coedge instead.
type AlgebraicGraphStockCoedge struct {
	UV           *geom.ExactComposedCurveCell
	Reversed     bool
	OriginalEdge int
	Cap          int
	Owner        AlgebraicGraphCapSeamOwner
	// PerimeterParameters are in the original model edge's increasing
	// normalized parameter. They are nil for cap attachments.
	PerimeterParameters [2]*big.Rat
	PerimeterReversed   bool
}

type AlgebraicGraphStockFaceLoop struct {
	OriginalFace int
	Reversed     bool
	Uses         []AlgebraicGraphStockCoedge
}

// OriginalStockLoops reconstructs the retained directed UV loops on the
// original stock faces. It includes unchanged faces and every shared cap
// contact; no constructed endpoint is rounded. Each incident face must be a
// certified rectangular stock chart. Separate directed cycles on the same
// chart are retained as distinct fragments. A nested stock face hole is
// outside this operation's domain.
func (c AlgebraicGraphCapPair) OriginalStockLoops(ctx context.Context, o RationalGraphShellSectionOptions, roots geom.ExactCurvePlaneEventOptions) ([]AlgebraicGraphStockFaceLoop, error) {
	b, e := newCapClipBudget(ctx, roots)
	if e != nil {
		return nil, e
	}
	return c.originalStockLoops(o, &b)
}

func (c AlgebraicGraphCapPair) originalStockLoops(o RationalGraphShellSectionOptions, b *capClipBudget) ([]AlgebraicGraphStockFaceLoop, error) {
	ctx := b.ctx
	if c.caps[0].recipe == nil || c.caps[1].recipe == nil || len(c.seams) == 0 {
		return nil, geom.ErrInvalid
	}
	sc := c.caps[0].recipe.stock
	buckets := make([][]AlgebraicGraphStockCoedge, len(sc.faces))
	for i, cap := range c.caps {
		for wi, wire := range cap.wires {
			for ui, use := range wire {
				if use.Seam >= 0 {
					continue
				}
				a, e := cap.OriginalStockAttachment(ctx, wi, ui, geom.SurfaceCurveAgreementOptions{SurfacePatchOptions: o.Spans.Faces.Patches}, b.limits())
				if e != nil {
					return nil, e
				}
				if e := b.charge(a.ExactOperations, a.Splits); e != nil {
					return nil, e
				}
				face := a.Source.Face
				buckets[face] = append(buckets[face], AlgebraicGraphStockCoedge{UV: a.UV, Reversed: a.StockReversed != sc.faces[face].Reversed,
					OriginalEdge: -1, Cap: i, Owner: AlgebraicGraphCapSeamOwner{wi, ui}})
			}
		}
	}
	var out []AlgebraicGraphStockFaceLoop
	for id, face := range sc.faces {
		if e := ctx.Err(); e != nil {
			return nil, e
		}
		row := buckets[id]
		patch, e := exactFaceSupportPatch(face, o.Spans.Faces.Patches)
		if e != nil {
			return nil, e
		}
		verts := slabFaceUV(face)
		var flags [2][4]int
		for v, uv := range verts {
			q, e := patch.EvaluateRational(ctx, uv[0], uv[1], geom.SurfacePatchEvaluationOptions{MaxOperations: b.work, MaxCoefficientBits: b.bits})
			if e != nil {
				return nil, e
			}
			if e := b.charge(q.ExactOperations, 0); e != nil {
				return nil, e
			}
			for i := range c.caps {
				p := c.caps[i].regions.plane
				flags[i][v] = new(big.Rat).Sub(p.dot(q.Point), p.offset).Sign() * p.orientation
				if flags[i][v] == 0 {
					return nil, geom.ErrConvergence
				}
			}
		}
		for i, original := range face.Outer {
			j := (i + 1) % len(verts)
			// The original rectangular proof shows a face section has only
			// its two known perimeter contacts. The exact original terminal
			// proves each cut parameter on this linear UV perimeter.
			lo, hi := new(big.Rat), big.NewRat(1, 1)
			kept := true
			for k, cap := range c.caps {
				a, z := flags[k][i], flags[k][j]
				if a == z {
					kept = kept && a < 0
					continue
				}
				var t *big.Rat
				for _, branch := range cap.recipe.branches {
					if branch.Face != id {
						continue
					}
					for _, end := range branch.Ends {
						if end.Edge == original.Edge {
							rng := sc.edges[original.Edge].Range
							t = new(big.Rat).Quo(new(big.Rat).Sub(end.Parameter, ratInset(rng.Min)), new(big.Rat).Sub(ratInset(rng.Max), ratInset(rng.Min)))
							if original.Reversed {
								t.Sub(big.NewRat(1, 1), t)
							}
						}
					}
				}
				if t == nil || t.Sign() <= 0 || t.Cmp(big.NewRat(1, 1)) >= 0 {
					return nil, geom.ErrConvergence
				}
				if a > 0 && lo.Cmp(t) < 0 {
					lo = t
				}
				if z > 0 && hi.Cmp(t) > 0 {
					hi = t
				}
			}
			if !kept || lo.Cmp(hi) >= 0 {
				continue
			}
			c := func(t *big.Rat) (p [4]*big.Rat) {
				for axis := 0; axis < 2; axis++ {
					d := new(big.Rat).Sub(verts[j][axis], verts[i][axis])
					p[axis] = new(big.Rat).Add(verts[i][axis], d.Mul(d, t))
				}
				p[2], p[3] = new(big.Rat), big.NewRat(1, 1)
				return p
			}
			lim := geom.SurfacePatchOptions{MaxOperations: b.work, MaxCoefficientBits: b.bits}
			uv, e := geom.NewExactHomogeneousBezier(ctx, [][4]*big.Rat{c(lo), c(hi)}, lim)
			if e != nil {
				return nil, e
			}
			if e := b.charge(uv.ExactOperations(), 0); e != nil {
				return nil, e
			}
			lim.MaxOperations = b.work
			q, used, e := geom.NewExactComposedBezierCellWithWork(ctx, uv, lim)
			if e != nil {
				return nil, e
			}
			if e := b.charge(used, 0); e != nil {
				return nil, e
			}
			parameters := [2]*big.Rat{lo, hi}
			if original.Reversed {
				parameters = [2]*big.Rat{new(big.Rat).Sub(big.NewRat(1, 1), hi), new(big.Rat).Sub(big.NewRat(1, 1), lo)}
			}
			row = append(row, AlgebraicGraphStockCoedge{UV: q, OriginalEdge: original.Edge, Cap: -1,
				PerimeterParameters: parameters, PerimeterReversed: original.Reversed})
		}
		if len(row) == 0 {
			continue
		}
		w, e := orderAlgebraicStockLoops(row, b)
		if e != nil {
			return nil, e
		}
		for _, loop := range w {
			out = append(out, AlgebraicGraphStockFaceLoop{id, face.Reversed, loop})
		}
	}
	return out, nil
}

func orderAlgebraicStockLoops(uses []AlgebraicGraphStockCoedge, b *capClipBudget) ([][]AlgebraicGraphStockCoedge, error) {
	type ends [2][2]*geom.ExactAlgebraicRoot
	points := make([]ends, len(uses))
	for i, u := range uses {
		for j := range points[i] {
			side := j
			if u.Reversed {
				side ^= 1
			}
			p, _ := u.UV.EndpointRoot(side)
			for k := range points[i][j] {
				q, work, e := u.UV.CoordinateRoot(b.ctx, p, k, b.limits())
				if e != nil {
					return nil, e
				}
				if e := b.charge(work, 0); e != nil {
					return nil, e
				}
				points[i][j][k] = q
			}
		}
	}
	equal := func(x, y [2]*geom.ExactAlgebraicRoot) (bool, error) {
		for i := range x {
			q, e := x[i].Compare(b.ctx, y[i], b.limits())
			if e != nil {
				return false, e
			}
			if e := b.charge(q.Operations, q.Subdivisions); e != nil {
				return false, e
			}
			if !q.Proven {
				return false, geom.ErrConvergence
			}
			if q.Order != 0 {
				return false, nil
			}
		}
		return true, nil
	}
	if len(uses) < 2 {
		return nil, geom.ErrConvergence
	}
	var out [][]AlgebraicGraphStockCoedge
	seen := make([]bool, len(uses))
	for start := range uses {
		if seen[start] {
			continue
		}
		var w []AlgebraicGraphStockCoedge
		for i := start; ; {
			if seen[i] {
				return nil, geom.ErrConvergence
			}
			seen[i] = true
			w = append(w, uses[i])
			next := -1
			for j := range uses {
				same, e := equal(points[i][1], points[j][0])
				if e != nil {
					return nil, e
				}
				if same {
					if next >= 0 {
						return nil, geom.ErrConvergence
					}
					next = j
				}
			}
			if next < 0 {
				return nil, geom.ErrConvergence
			}
			if next == start {
				if len(w) < 2 {
					return nil, geom.ErrConvergence
				}
				break
			}
			i = next
		}
		out = append(out, w)
	}
	// With two regular sections, each nonintersecting arrangement arc lies
	// on a graph crossing the original rectangle. A bounded complement
	// cannot form a stock hole: each excluded side reaches the rectangle
	// boundary. A single clipped graph, or the union of both excluded
	// sides, has the same property. Thus disjoint closed stock cycles are
	// distinct retained faces rather than inner loops of one face.
	return out, nil
}
