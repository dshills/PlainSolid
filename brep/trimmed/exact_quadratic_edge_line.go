package trimmed

import (
	"context"
	"math/big"
	"sort"

	"plainsolid/geom"
)

// ExactQuadraticEdgePlaneHit retains the original normalized trim
// parameter bracket and exact local order on a composed model curve.
type ExactQuadraticEdgePlaneHit struct {
	Parameter         geom.ToroidLineRoot
	Crossing, Tangent bool // known only for roots interior to a single piece
	Pieces            []int
}

// ExactQuadraticEdgePlaneSection is an exhaustive plane section of the
// supported edge when Complete. Coplanar intervals retain their exact
// normalized parameter endpoints.
type ExactQuadraticEdgePlaneSection struct {
	Complete   bool
	Hits       []ExactQuadraticEdgePlaneHit
	Coincident []geom.ExactParameterInterval
}

// IntersectExactQuadraticTrimPlane intersects a composed exact quadratic
// trim against the plane through three represented points. No normalized
// normal or tessellation participates. Rational source knot cuts are
// supported; the geometry report of irrational cuts remains available
// from ModelCurveCells.
func (q ExactQuadraticUV) IntersectExactQuadraticTrimPlane(ctx context.Context, p *geom.SurfacePatch, plane [3]geom.Vec3,
	o geom.SplinePlaneOptions, c geom.SurfaceCurveCompositionOptions) (ExactQuadraticEdgePlaneSection, error) {
	var empty ExactQuadraticEdgePlaneSection
	if ctx == nil {
		return empty, geom.ErrInvalid
	}
	pieces, e := q.ModelCurvePieces(ctx, p, c)
	if e != nil {
		return empty, e
	}
	if !pieces.Complete || !pieces.Continuous {
		return empty, geom.ErrConvergence
	}
	if o.ParameterTolerance == 0 {
		o.ParameterTolerance = 1e-10
	}
	if o.MaxOperations == 0 {
		o.MaxOperations = 8000000
	}
	if o.MaxSubdivisions == 0 {
		o.MaxSubdivisions = 8192
	}
	r := ExactQuadraticEdgePlaneSection{}
	for i, piece := range pieces.Pieces {
		if o.MaxOperations < 1 || o.MaxSubdivisions < 1 {
			return empty, geom.ErrConvergence
		}
		section, e := piece.Curve.IntersectRawPlane(ctx, plane[0], plane[1], plane[2], o)
		if e != nil {
			return empty, e
		}
		if !section.Complete {
			return empty, geom.ErrConvergence
		}
		o.MaxOperations -= section.Operations
		o.MaxSubdivisions -= section.Subdivisions
		width := new(big.Rat).Sub(piece.End, piece.Start)
		parameter := func(x *big.Rat) *big.Rat { return new(big.Rat).Add(piece.Start, new(big.Rat).Mul(width, x)) }
		for _, x := range section.Hits {
			r.Hits = append(r.Hits, ExactQuadraticEdgePlaneHit{
				Parameter: geom.ToroidLineRoot{Min: parameter(x.Lower), Max: parameter(x.Upper), Multiplicity: x.Multiplicity},
				Crossing:  x.Crossing && x.Lower.Sign() > 0 && x.Upper.Cmp(big.NewRat(1, 1)) < 0,
				Tangent:   !x.Crossing && x.Lower.Sign() > 0 && x.Upper.Cmp(big.NewRat(1, 1)) < 0,
				Pieces:    []int{i},
			})
		}
		for _, x := range section.Coincident {
			l, h := parameter(ratInset(x.Start)), parameter(ratInset(x.End))
			if k := len(r.Coincident) - 1; k >= 0 && r.Coincident[k].Upper.Cmp(l) == 0 {
				r.Coincident[k].Upper = h
			} else {
				r.Coincident = append(r.Coincident, geom.ExactParameterInterval{Lower: l, Upper: h})
			}
		}
	}
	sort.SliceStable(r.Hits, func(i, j int) bool { return r.Hits[i].Parameter.Min.Cmp(r.Hits[j].Parameter.Min) < 0 })
	hits := r.Hits[:0]
	for _, h := range r.Hits {
		if len(hits) > 0 {
			last := &hits[len(hits)-1]
			a, b := last.Parameter, h.Parameter
			if a.Max.Cmp(b.Min) >= 0 {
				if a.Min.Cmp(a.Max) != 0 || b.Min.Cmp(b.Max) != 0 || a.Min.Cmp(b.Min) != 0 {
					return empty, geom.ErrConvergence
				}
				last.Parameter.Multiplicity = max(a.Multiplicity, b.Multiplicity)
				last.Pieces = append(last.Pieces, h.Pieces...)
				// Across a knot, local polynomial order does not by
				// itself prove global sign change.
				last.Crossing, last.Tangent = false, false
				continue
			}
		}
		hits = append(hits, h)
	}
	r.Hits, r.Complete = hits, true
	return r, nil
}
