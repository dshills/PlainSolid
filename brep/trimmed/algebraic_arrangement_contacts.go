//go:build research

package trimmed

import (
	"context"
	"math/big"
	"plainsolid/geom"
)

type AlgebraicArrangementContactOptions struct {
	Seams        AlgebraicSeamOptions
	Roots        geom.ExactCurvePlaneEventOptions
	MaxCellPairs int
}

type AlgebraicArrangementEdgeContact struct {
	geom.PairedComposedCurveEvent
	Cells [][2]int
}

type AlgebraicArrangementCoincidence struct {
	Boundary [2]geom.PairedComposedCurveEvent
	Cells    [2]int
}

type AlgebraicArrangementEdgeContacts struct {
	Edges                                      [2]int
	ProjectionAxis                             int
	Hits                                       []AlgebraicArrangementEdgeContact
	Coincident                                 []AlgebraicArrangementCoincidence
	TestedPairs, ExactOperations, Subdivisions int
	Complete                                   bool
}

// IntersectAlgebraicArrangementEdges proves contacts between selected original
// or generated edges of independent arrangements. It keeps both original
// parameter roots, including reversed traversal and multi-cell duplicates.
// The selected coordinate must be proved affine in both cell parameters.
// Axis -1 selects bounded exact projective quadratic/conic pairing instead.
// Coincident pieces stay separately attributed; they do not imply a weld.
func IntersectAlgebraicArrangementEdges(ctx context.Context, a AlgebraicFaceArrangement, edgeA int,
	b AlgebraicFaceArrangement, edgeB, axis int, o AlgebraicArrangementContactOptions) (AlgebraicArrangementEdgeContacts, error) {
	out := AlgebraicArrangementEdgeContacts{Edges: [2]int{edgeA, edgeB}, ProjectionAxis: axis}
	if ctx == nil || axis < -1 || axis > 2 || o.MaxCellPairs < 0 || o.MaxCellPairs > 65536 {
		return out, geom.ErrInvalid
	}
	if o.MaxCellPairs == 0 {
		o.MaxCellPairs = 256
	}
	if o.Roots.MaxOperations == 0 {
		o.Roots.MaxOperations = 8000000
	}
	if o.Roots.MaxSubdivisions == 0 {
		o.Roots.MaxSubdivisions = 8192
	}
	if o.Roots.MaxOperations < 1 || o.Roots.MaxSubdivisions < 1 {
		return out, geom.ErrInvalid
	}
	x, e := a.ModelCurveCells(ctx, edgeA, o.Seams)
	if e != nil {
		return out, e
	}
	y, e := b.ModelCurveCells(ctx, edgeB, o.Seams)
	if e != nil {
		return out, e
	}
	if !x.Complete || !y.Complete || len(x.Cells) == 0 || len(y.Cells) == 0 || len(x.Cells) > o.MaxCellPairs/len(y.Cells) {
		return out, geom.ErrConvergence
	}
	limits := func() (geom.ExactCurvePlaneEventOptions, error) {
		q := o.Roots
		q.MaxOperations -= out.ExactOperations
		q.MaxSubdivisions -= out.Subdivisions
		if q.MaxOperations < 1 || q.MaxSubdivisions < 1 {
			return q, geom.ErrConvergence
		}
		return q, nil
	}
	compare := func(x, y *geom.ExactComposedCurveEvent) (int, error) {
		q, e := limits()
		if e != nil {
			return 0, e
		}
		r, e := x.CompareParameter(ctx, y, q)
		out.ExactOperations += r.Operations
		out.Subdivisions += r.Subdivisions
		if e == nil && !r.Proven {
			e = geom.ErrConvergence
		}
		return r.Order, e
	}
	insert := func(h geom.PairedComposedCurveEvent, cells [2]int) error {
		pos := len(out.Hits)
		for pos > 0 {
			prior := &out.Hits[pos-1]
			order, e := compare(prior.A, h.A)
			if e != nil {
				return e
			}
			if order < 0 {
				break
			}
			if order == 0 {
				same, e := compare(prior.B, h.B)
				if e != nil {
					return e
				}
				if same == 0 {
					prior.Cells = append(prior.Cells, cells)
					return nil
				}
				if axis != -1 {
					return geom.ErrConvergence
				}
				if same < 0 {
					break
				}
			}
			pos--
		}
		out.Hits = append(out.Hits, AlgebraicArrangementEdgeContact{})
		copy(out.Hits[pos+1:], out.Hits[pos:])
		out.Hits[pos] = AlgebraicArrangementEdgeContact{h, [][2]int{cells}}
		return nil
	}
	var offset, scale *big.Rat
	if axis == -1 {
		for _, cells := range [][]*geom.ExactComposedCurveCell{x.Cells, y.Cells} {
			for i, c := range cells {
				q, e := limits()
				if e != nil {
					return out, e
				}
				r, ops, e := c.ReduceHomogeneous(ctx, q)
				out.ExactOperations += ops
				if e != nil {
					return out, e
				}
				cells[i] = r
			}
		}
	}
	for i, u := range x.Cells {
		for j, v := range y.Cells {
			if e := ctx.Err(); e != nil {
				return out, e
			}
			q, e := limits()
			if e != nil {
				return out, e
			}
			if axis == -1 {
				out.TestedPairs++
				r, e := geom.IntersectComposedQuadraticConic(ctx, u, v, q)
				out.ExactOperations += r.Operations
				if e != nil && !r.CoincidentSupport {
					return out, e
				}
				cells := [2]int{i, j}
				if r.CoincidentSupport {
					q, e = limits()
					if e != nil {
						return out, e
					}
					z, e := geom.IntersectCoincidentComposedConics(ctx, u, v, q)
					out.ExactOperations += z.Operations
					if e != nil {
						return out, e
					}
					if !z.Complete {
						return out, geom.ErrConvergence
					}
					if z.Coincident {
						out.Coincident = append(out.Coincident, AlgebraicArrangementCoincidence{z.Boundary, cells})
					}
					if z.PointOnly {
						if e := insert(z.Boundary[0], cells); e != nil {
							return out, e
						}
					}
				} else {
					if !r.Complete {
						return out, geom.ErrConvergence
					}
					for _, h := range r.Hits {
						if e := insert(h, cells); e != nil {
							return out, e
						}
					}
				}
				continue
			}
			r, e := geom.IntersectComposedCurvesAffineProjection(ctx, u, v, axis, q)
			out.TestedPairs++
			out.ExactOperations += r.Operations
			out.Subdivisions += r.Subdivisions
			if e != nil {
				return out, e
			}
			if !r.Complete {
				return out, geom.ErrConvergence
			}
			if scale == nil {
				offset, scale = r.Offset, r.Scale
			} else if offset.Cmp(r.Offset) != 0 || scale.Cmp(r.Scale) != 0 {
				return out, geom.ErrConvergence
			}
			cells := [2]int{i, j}
			if r.Coincident {
				out.Coincident = append(out.Coincident, AlgebraicArrangementCoincidence{r.CoincidentBoundary, cells})
			}
			for _, h := range r.Hits {
				if e := insert(h, cells); e != nil {
					return out, e
				}
			}
		}
	}
	keep := out.Hits[:0]
	for _, h := range out.Hits {
		covered := false
		for _, span := range out.Coincident {
			lo, e := compare(h.A, span.Boundary[0].A)
			if e != nil {
				return out, e
			}
			if lo < 0 {
				continue
			}
			hi, e := compare(h.A, span.Boundary[1].A)
			if e != nil {
				return out, e
			}
			if hi <= 0 {
				// Nonlinear curves can retrace a parameter's model image
				// on another B arc. Only suppress the same paired root.
				if axis == -1 {
					b0, b1 := span.Boundary[0].B, span.Boundary[1].B
					order, e := compare(b0, b1)
					if e != nil {
						return out, e
					}
					if order > 0 {
						b0, b1 = b1, b0
					}
					l, e := compare(h.B, b0)
					if e != nil {
						return out, e
					}
					u, e := compare(h.B, b1)
					if e != nil {
						return out, e
					}
					if l < 0 || u > 0 {
						continue
					}
				}
				covered = true
				break
			}
		}
		if !covered {
			keep = append(keep, h)
		}
	}
	out.Hits = keep
	out.Complete = true
	return out, nil
}
