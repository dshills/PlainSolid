package trimmed

import (
	"math/big"

	"plainsolid/brep/edgecheck"
	"plainsolid/geom"
)

// splineGraphAffineWall uses a represented physical side vector and a
// vertical coordinate. Its horizontal parameter spans [0,1].
type splineGraphAffineWall struct {
	origin, along geom.Vec3
}

func newSplineGraphAffineWall(p, q geom.Vec3) (splineGraphAffineWall, error) {
	var out splineGraphAffineWall
	if !p.IsFinite() || !q.IsFinite() {
		return out, geom.ErrInvalid
	}
	x, e := geom.RepresentedExactScalar(new(big.Rat).Sub(ratInset(q.X), ratInset(p.X)))
	if e != nil {
		return out, e
	}
	y, e := geom.RepresentedExactScalar(new(big.Rat).Sub(ratInset(q.Y), ratInset(p.Y)))
	if e != nil {
		return out, e
	}
	if x == 0 && y == 0 {
		return out, geom.ErrDegenerate
	}
	return splineGraphAffineWall{origin: p, along: geom.Vec3{X: x, Y: y}}, nil
}

func (w splineGraphAffineWall) Eval(u, v float64) (geom.Vec3, error) {
	p := w.origin.Add(w.along.Scale(u)).Add(geom.Vec3{Z: v})
	if !p.IsFinite() {
		return geom.Vec3{}, geom.ErrInvalid
	}
	return p, nil
}

func (w splineGraphAffineWall) Partials(u, v float64) (geom.Vec3, geom.Vec3, error) {
	if !finite(u) || !finite(v) {
		return geom.Vec3{}, geom.Vec3{}, geom.ErrInvalid
	}
	return w.along, geom.Vec3{Z: 1}, nil
}

func (w splineGraphAffineWall) chart() edgecheck.Chart {
	return edgecheck.Chart{Origin: w.origin, U: w.along, V: geom.Vec3{Z: 1}}
}
