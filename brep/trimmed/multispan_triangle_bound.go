package trimmed

import (
	"context"
	"math/big"
	"plainsolid/geom"
)

// MultiSpanTriangleBoundOptions bounds candidate knot cells, exact clipped
// triangle pieces and input rational coordinate bits. Defaults are1024,8192
// and16384; hard maxima are65536 cells,1M pieces and65536 input bits.
// Exact support restriction retains the patch's own construction limits.
type MultiSpanTriangleBoundOptions struct {
	MaxCells, MaxPieces, MaxInputBits int
	MaxDegree, MaxResidualOperations  int
}

// MultiSpanTrianglePiece retains its knot cell index and exact rational UV
// partition triangle. Model is the original display triangle's exact affine
// correspondence at those vertices, not separately rounded model samples.
type MultiSpanTrianglePiece struct {
	Cell            [2]int
	UV              [3][2]*big.Rat
	Model           [3][3]*big.Rat
	SquaredDistance *big.Rat
}

// MultiSpanTriangleDeviation certifies both directed distances through the
// original UV barycentric correspondence. Pieces partition its exact triangle
// at all intersecting knot cells; SquaredDistance is their maximum residual.
// The certificate does not prove surface or display triangle embedding.
type MultiSpanTriangleDeviation struct {
	ResidualOperations int
	SquaredDistance    *big.Rat
	BothDirections     bool
	Cells              int
	Pieces             []MultiSpanTrianglePiece
}

// BoundMultiSpanSplineTriangleDeviation is the represented-coordinate wrapper
// for BoundRationalMultiSpanSplineTriangleDeviation. Model rounding is included.
func BoundMultiSpanSplineTriangleDeviation(ctx context.Context, p *geom.SurfacePatch, uv [3]geom.Vec2, model [3]geom.Vec3, o MultiSpanTriangleBoundOptions) (MultiSpanTriangleDeviation, error) {
	var exact [3][2]*big.Rat
	for i, v := range uv {
		if !finite(v.X) || !finite(v.Y) {
			return MultiSpanTriangleDeviation{}, geom.ErrInvalid
		}
		exact[i] = [2]*big.Rat{ratInset(v.X), ratInset(v.Y)}
	}
	return BoundRationalMultiSpanSplineTriangleDeviation(ctx, p, exact, model, o)
}

// BoundRationalMultiSpanSplineTriangleDeviation clips against each candidate
// knot rectangle in exact rational arithmetic, restricts the spline exactly to
// that span, and applies its rational biquadratic Bernstein residual. Supports
// default to degree<=2; MaxDegree explicitly enables exact residuals through
// degree8 with an additional per-triangle arithmetic limit. C0 joins follow the
// SurfacePatch contract. Intersections and target interpolation are never
// rounded to binary64. All failures return no partial certificate.
func BoundRationalMultiSpanSplineTriangleDeviation(ctx context.Context, p *geom.SurfacePatch, uv [3][2]*big.Rat, model [3]geom.Vec3, o MultiSpanTriangleBoundOptions) (MultiSpanTriangleDeviation, error) {
	empty := MultiSpanTriangleDeviation{}
	if ctx == nil || p == nil {
		return empty, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return empty, e
	}
	if o.MaxDegree == 0 {
		o.MaxDegree = 2
	}
	if o.MaxResidualOperations == 0 {
		o.MaxResidualOperations = 2000000
	}
	if o.MaxDegree < 1 || o.MaxDegree > 8 || o.MaxResidualOperations < 1 || o.MaxResidualOperations > 100000000 {
		return empty, geom.ErrInvalid
	}
	if o.MaxCells == 0 {
		o.MaxCells = 1024
	}
	if o.MaxPieces == 0 {
		o.MaxPieces = 8192
	}
	if o.MaxInputBits == 0 {
		o.MaxInputBits = 16384
	}
	if o.MaxCells < 1 || o.MaxCells > 65536 || o.MaxPieces < 1 || o.MaxPieces > 1000000 || o.MaxInputBits < 1 || o.MaxInputBits > 65536 {
		return empty, geom.ErrInvalid
	}
	d := p.Domain()
	if !finite(d.UMin) || !finite(d.UMax) || !finite(d.VMin) || !finite(d.VMax) || d.UMin >= d.UMax || d.VMin >= d.VMax {
		return empty, geom.ErrInvalid
	}
	for i := range uv {
		for j := range uv[i] {
			q := uv[i][j]
			if q == nil {
				return empty, geom.ErrInvalid
			}
			if q.Num().BitLen() > o.MaxInputBits || q.Denom().BitLen() > o.MaxInputBits {
				return empty, geom.ErrConvergence
			}
			uv[i][j] = new(big.Rat).Set(q)
		}
		if uv[i][0].Cmp(ratInset(d.UMin)) < 0 || uv[i][0].Cmp(ratInset(d.UMax)) > 0 || uv[i][1].Cmp(ratInset(d.VMin)) < 0 || uv[i][1].Cmp(ratInset(d.VMax)) > 0 {
			return empty, geom.ErrDomain
		}
		if !model[i].IsFinite() {
			return empty, geom.ErrInvalid
		}
	}
	area := exactUVSignedArea(uv[0], uv[1], uv[2])
	if area.Sign() == 0 || newExactBoundTriangle(model).det.Sign() == 0 {
		return empty, geom.ErrDegenerate
	}
	minimum, maximum := uv[0], uv[0]
	for _, q := range uv[1:] {
		for k := 0; k < 2; k++ {
			if q[k].Cmp(minimum[k]) < 0 {
				minimum[k] = q[k]
			}
			if q[k].Cmp(maximum[k]) > 0 {
				maximum[k] = q[k]
			}
		}
	}
	us, vs := p.KnotBreaks()
	var ui, vi []int
	for i := 0; i+1 < len(us); i++ {
		if ratInset(us[i]).Cmp(maximum[0]) < 0 && ratInset(us[i+1]).Cmp(minimum[0]) > 0 {
			ui = append(ui, i)
		}
	}
	for i := 0; i+1 < len(vs); i++ {
		if ratInset(vs[i]).Cmp(maximum[1]) < 0 && ratInset(vs[i+1]).Cmp(minimum[1]) > 0 {
			vi = append(vi, i)
		}
	}
	if len(ui) == 0 || len(vi) == 0 || len(ui) > o.MaxCells/len(vi) {
		return empty, geom.ErrConvergence
	}
	targets := [3]exactBoundVector{exactBoundPoint(model[0]), exactBoundPoint(model[1]), exactBoundPoint(model[2])}
	target := func(q [2]*big.Rat) [3]*big.Rat {
		s := new(big.Rat).Quo(exactUVSignedArea(uv[0], q, uv[2]), area)
		t := new(big.Rat).Quo(exactUVSignedArea(uv[0], uv[1], q), area)
		var out [3]*big.Rat
		for k := range out {
			out[k] = new(big.Rat).Add(targets[0][k], new(big.Rat).Add(new(big.Rat).Mul(s, new(big.Rat).Sub(targets[1][k], targets[0][k])), new(big.Rat).Mul(t, new(big.Rat).Sub(targets[2][k], targets[0][k]))))
		}
		return out
	}
	result := MultiSpanTriangleDeviation{SquaredDistance: new(big.Rat), BothDirections: true}
	covered := new(big.Rat)
	for _, i := range ui {
		for _, j := range vi {
			if e := ctx.Err(); e != nil {
				return empty, e
			}
			result.Cells++
			polygon := [][2]*big.Rat{uv[0], uv[1], uv[2]}
			polygon = clipExactUVPolygon(polygon, 0, ratInset(us[i]), true)
			polygon = clipExactUVPolygon(polygon, 0, ratInset(us[i+1]), false)
			polygon = clipExactUVPolygon(polygon, 1, ratInset(vs[j]), true)
			polygon = clipExactUVPolygon(polygon, 1, ratInset(vs[j+1]), false)
			if len(polygon) < 3 {
				continue
			}
			cell, e := p.Trim(geom.ParameterRectangle{UMin: us[i], UMax: us[i+1], VMin: vs[j], VMax: vs[j+1]})
			if e != nil {
				return empty, e
			}
			cu, cv := cell.ControlShape()
			if cu > o.MaxDegree+1 || cv > o.MaxDegree+1 {
				return empty, geom.ErrConvergence
			}
			for k := 1; k+1 < len(polygon); k++ {
				triangle := [3][2]*big.Rat{polygon[0], polygon[k], polygon[k+1]}
				signed := exactUVSignedArea(triangle[0], triangle[1], triangle[2])
				if signed.Sign() == 0 {
					continue
				}
				if signed.Sign() != area.Sign() {
					return empty, geom.ErrConvergence
				}
				if len(result.Pieces) >= o.MaxPieces {
					return empty, geom.ErrConvergence
				}
				model := [3][3]*big.Rat{target(triangle[0]), target(triangle[1]), target(triangle[2])}
				bound, ok := rationalBiquadraticRationalCorrespondenceBound(cell, triangle, model)
				if !ok && o.MaxDegree > 2 {
					remaining := o.MaxResidualOperations - result.ResidualOperations
					if remaining < 1 {
						return empty, geom.ErrConvergence
					}
					certificate, e := BoundRationalBezierTriangleDeviation(ctx, cell, triangle, model, BezierTriangleBoundOptions{MaxDegree: o.MaxDegree, MaxOperations: remaining, MaxBits: o.MaxInputBits})
					if e != nil {
						return empty, e
					}
					result.ResidualOperations += certificate.Operations
					bound, ok = certificate.SquaredDistance, certificate.BothDirections
				}
				if !ok {
					return empty, geom.ErrConvergence
				}
				result.Pieces = append(result.Pieces, MultiSpanTrianglePiece{Cell: [2]int{i, j}, UV: triangle, Model: model, SquaredDistance: bound})
				covered.Add(covered, signed)
				if bound.Cmp(result.SquaredDistance) > 0 {
					result.SquaredDistance = new(big.Rat).Set(bound)
				}
			}
		}
	}
	if len(result.Pieces) == 0 || covered.Cmp(area) != 0 {
		return empty, geom.ErrConvergence
	}
	if e := ctx.Err(); e != nil {
		return empty, e
	}
	return result, nil
}

func exactUVSignedArea(a, b, c [2]*big.Rat) *big.Rat {
	return new(big.Rat).Sub(new(big.Rat).Mul(new(big.Rat).Sub(b[0], a[0]), new(big.Rat).Sub(c[1], a[1])), new(big.Rat).Mul(new(big.Rat).Sub(b[1], a[1]), new(big.Rat).Sub(c[0], a[0])))
}

func clipExactUVPolygon(polygon [][2]*big.Rat, axis int, bound *big.Rat, lower bool) [][2]*big.Rat {
	var out [][2]*big.Rat
	if len(polygon) == 0 {
		return out
	}
	appendPoint := func(q [2]*big.Rat) {
		if len(out) == 0 || !rationalGraphSameUV(out[len(out)-1], q) {
			out = append(out, q)
		}
	}
	for i, a := range polygon {
		b := polygon[(i+1)%len(polygon)]
		da := new(big.Rat).Sub(a[axis], bound)
		db := new(big.Rat).Sub(b[axis], bound)
		if !lower {
			da.Neg(da)
			db.Neg(db)
		}
		insideA, insideB := da.Sign() >= 0, db.Sign() >= 0
		if insideA != insideB {
			t := new(big.Rat).Quo(da, new(big.Rat).Sub(da, db))
			var q [2]*big.Rat
			for k := range q {
				q[k] = new(big.Rat).Add(a[k], new(big.Rat).Mul(t, new(big.Rat).Sub(b[k], a[k])))
			}
			appendPoint(q)
		}
		if insideB {
			appendPoint(b)
		}
	}
	if len(out) > 1 && rationalGraphSameUV(out[0], out[len(out)-1]) {
		out = out[:len(out)-1]
	}
	return out
}
