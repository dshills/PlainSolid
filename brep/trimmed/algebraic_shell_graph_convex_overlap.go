package trimmed

import (
	"context"
	"math/big"
	"plainsolid/geom"
)

type AlgebraicGraphConvexOverlapOptions struct {
	Exact       geom.ExactCurvePlaneEventOptions
	MaxVertices int
}

// AlgebraicGraphConvexOverlap is exact original-domain exclusion evidence.
// Overlap is a closed convex polygon in ProjectionAxes order, not a material
// footprint. Volume lower is zero; its upper charges the complete overlap area at
// a conservative original-support depth. Window is only its bounding rectangle
// and is never promoted to material. Zero projected area is not a point-contact
// classification. All rational coordinates are owned by the result.
type AlgebraicGraphConvexOverlap struct {
	Complete, HasProjectedArea bool
	ProjectionAxes             [2]int
	ResidualAxis               int
	Original                   [2][][2]*big.Rat
	Overlap                    [][2]*big.Rat
	Window                     AlgebraicGraphProjectionWindow
	ProjectedArea, DepthUpper  *big.Rat
	Volume                     [2]*big.Rat
	ExactOperations            int
	// Placement maps the original native Boolean to the measured image.
	Placement geom.Affine
	// NativeFaces retains operand-ordered selected face IDs for each region.
	NativeFaces [][2]int
}

// BoundConvexWholeVolume supports strictly convex straight original trim
// polygons under arbitrary nonsingular affine graph projection. Exact rational
// clipping excludes everything outside their intersection without classifying
// anything inside it as retained material. MaxVertices defaults to 128 and caps
// each source/intermediate polygon (maximum 128). Exact work/bits are cumulative.
func (q *AlgebraicGraphMaterialQuery) BoundConvexWholeVolume(ctx context.Context, o AlgebraicGraphConvexOverlapOptions) (out AlgebraicGraphConvexOverlap, err error) {
	defer func() {
		if err != nil {
			out = AlgebraicGraphConvexOverlap{ExactOperations: out.ExactOperations}
		}
	}()
	if ctx == nil || q == nil || q.graphs[0] == nil || len(q.regions) == 0 {
		return out, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return out, e
	}
	o.Exact, err = graphMaterialLimits(o.Exact)
	if err != nil {
		return out, err
	}
	if o.MaxVertices == 0 {
		o.MaxVertices = 128
	}
	if o.MaxVertices < 3 || o.MaxVertices > 128 {
		return out, geom.ErrInvalid
	}
	var failure error
	calc := func(op byte, a, b *big.Rat) *big.Rat {
		r := new(big.Rat)
		if failure != nil {
			return r
		}
		if out.ExactOperations >= o.Exact.MaxOperations || !graphPolynomialBits(a, o.Exact.MaxCoefficientBits) || !graphPolynomialBits(b, o.Exact.MaxCoefficientBits) {
			failure = geom.ErrConvergence
			return r
		}
		out.ExactOperations++
		switch op {
		case '+':
			r.Add(a, b)
		case '-':
			r.Sub(a, b)
		case '*':
			r.Mul(a, b)
		case '/':
			if b.Sign() == 0 {
				failure = geom.ErrConvergence
				return r
			}
			r.Quo(a, b)
		}
		if !graphPolynomialBits(r, o.Exact.MaxCoefficientBits) {
			failure = geom.ErrConvergence
		}
		return r
	}
	cross := func(a, b, p [2]*big.Rat) *big.Rat {
		return calc('-', calc('*', calc('-', b[0], a[0]), calc('-', p[1], a[1])), calc('*', calc('-', b[1], a[1]), calc('-', p[0], a[0])))
	}
	signedArea := func(p [][2]*big.Rat) *big.Rat {
		a := new(big.Rat)
		for i, x := range p {
			y := p[(i+1)%len(p)]
			a = calc('+', a, calc('-', calc('*', x[0], y[1]), calc('*', x[1], y[0])))
		}
		return calc('/', a, big.NewRat(2, 1))
	}
	clone := func(p [][2]*big.Rat) [][2]*big.Rat {
		r := make([][2]*big.Rat, len(p))
		for i, x := range p {
			r[i] = [2]*big.Rat{new(big.Rat).Set(x[0]), new(big.Rat).Set(x[1])}
		}
		return r
	}
	for owner, g := range q.graphs {
		f := q.faces[owner]
		if len(f.Holes) != 0 || len(f.Outer) < 3 || len(f.Outer) > o.MaxVertices {
			return out, geom.ErrConvergence
		}
		inverse, e := g.InverseCoefficients()
		if e != nil {
			return out, e
		}
		det := calc('-', calc('*', inverse[0][0], inverse[1][1]), calc('*', inverse[0][1], inverse[1][0]))
		if failure != nil {
			return out, failure
		}
		if det.Sign() == 0 {
			return out, geom.ErrConvergence
		}
		for i, u := range f.Outer {
			if u.Curve != nil || u.UVEnd != f.Outer[(i+1)%len(f.Outer)].UVStart || !finite(u.UVStart.X) || !finite(u.UVStart.Y) {
				return out, geom.ErrConvergence
			}
			a, b := calc('-', ratInset(u.UVStart.X), inverse[0][2]), calc('-', ratInset(u.UVStart.Y), inverse[1][2])
			x := calc('/', calc('-', calc('*', a, inverse[1][1]), calc('*', inverse[0][1], b)), det)
			y := calc('/', calc('-', calc('*', inverse[0][0], b), calc('*', a, inverse[1][0])), det)
			out.Original[owner] = append(out.Original[owner], [2]*big.Rat{x, y})
		}
		polygon := out.Original[owner]
		a := signedArea(polygon)
		if failure != nil {
			return out, failure
		}
		if a.Sign() == 0 {
			return out, geom.ErrConvergence
		}
		if a.Sign() < 0 {
			for i, j := 1, len(polygon)-1; i < j; i, j = i+1, j-1 {
				polygon[i], polygon[j] = polygon[j], polygon[i]
			}
		}
		for i, a := range polygon {
			if e := ctx.Err(); e != nil {
				return out, e
			}
			b := polygon[(i+1)%len(polygon)]
			for j, p := range polygon {
				if j == i || j == (i+1)%len(polygon) {
					continue
				}
				if cross(a, b, p).Sign() <= 0 {
					return out, geom.ErrConvergence
				}
				if failure != nil {
					return out, failure
				}
			}
		}
	}
	polygon := clone(out.Original[0])
	equal := func(a, b [2]*big.Rat) bool { return a[0].Cmp(b[0]) == 0 && a[1].Cmp(b[1]) == 0 }
	for i, a := range out.Original[1] {
		if e := ctx.Err(); e != nil {
			return out, e
		}
		if len(polygon) == 0 {
			break
		}
		b := out.Original[1][(i+1)%len(out.Original[1])]
		var next [][2]*big.Rat
		appendPoint := func(p [2]*big.Rat) {
			if len(next) == 0 || !equal(next[len(next)-1], p) {
				next = append(next, p)
			}
		}
		for j, p := range polygon {
			z := polygon[(j+1)%len(polygon)]
			cp, cz := cross(a, b, p), cross(a, b, z)
			if failure != nil {
				return out, failure
			}
			insideP, insideZ := cp.Sign() >= 0, cz.Sign() >= 0
			if insideP {
				appendPoint(p)
			}
			if insideP != insideZ {
				t := calc('/', cp, calc('-', cp, cz))
				appendPoint([2]*big.Rat{calc('+', p[0], calc('*', t, calc('-', z[0], p[0]))), calc('+', p[1], calc('*', t, calc('-', z[1], p[1])))})
			}
			if len(next) > o.MaxVertices {
				return out, geom.ErrConvergence
			}
		}
		if len(next) > 1 && equal(next[0], next[len(next)-1]) {
			next = next[:len(next)-1]
		}
		polygon = next
		if failure != nil {
			return out, failure
		}
	}
	out.Overlap = clone(polygon)
	out.ProjectedArea = signedArea(polygon)
	if failure != nil {
		return out, failure
	}
	if out.ProjectedArea.Sign() < 0 {
		return out, geom.ErrConvergence
	}
	out.Placement = geom.Identity()
	for _, r := range q.regions {
		out.NativeFaces = append(out.NativeFaces, r.faces)
	}
	out.ProjectionAxes = q.axes
	out.ResidualAxis = q.residual
	out.Volume = [2]*big.Rat{new(big.Rat), new(big.Rat)}
	out.DepthUpper = new(big.Rat)
	if len(polygon) > 0 {
		out.Window = AlgebraicGraphProjectionWindow{Min: [2]*big.Rat{new(big.Rat).Set(polygon[0][0]), new(big.Rat).Set(polygon[0][1])}, Max: [2]*big.Rat{new(big.Rat).Set(polygon[0][0]), new(big.Rat).Set(polygon[0][1])}}
		for _, p := range polygon {
			for axis, x := range p {
				if x.Cmp(out.Window.Min[axis]) < 0 {
					out.Window.Min[axis].Set(x)
				}
				if x.Cmp(out.Window.Max[axis]) > 0 {
					out.Window.Max[axis].Set(x)
				}
			}
		}
	}
	if out.ProjectedArea.Sign() > 0 {
		var heights [2][2]*big.Rat
		for owner, p := range q.patches {
			b, e := p.Bounds()
			if e != nil {
				return out, e
			}
			lo := [3]float64{b.Min.X, b.Min.Y, b.Min.Z}
			hi := [3]float64{b.Max.X, b.Max.Y, b.Max.Z}
			if !finite(lo[q.residual]) || !finite(hi[q.residual]) {
				return out, geom.ErrConvergence
			}
			heights[owner] = [2]*big.Rat{ratInset(lo[q.residual]), ratInset(hi[q.residual])}
		}
		for _, region := range q.regions {
			d := calc('-', heights[region.upper][1], heights[region.upper^1][0])
			if d.Cmp(out.DepthUpper) > 0 {
				out.DepthUpper.Set(d)
			}
		}
		out.Volume[1] = calc('*', out.ProjectedArea, out.DepthUpper)
		out.HasProjectedArea = true
	}
	if failure != nil {
		return out, failure
	}
	out.Complete = true
	return out, nil
}
