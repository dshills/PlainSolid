package parametric

import (
	"context"
	"math/big"

	"plainsolid/geom"
)

// SphereCylinderCavity retains the outer analytic sphere and an independently
// closed, inward-oriented cylinder shell. Face indices are stable ancestry.
// VolumePiCoefficient is caller-owned exact ideal volume divided by π.
type SphereCylinderCavity struct {
	Solid                                Solid
	SphereFace                           int
	CylinderWall, LowerFloor, UpperFloor int
	VolumePiCoefficient                  *big.Rat
}

// SphereEnclosedCylinderCavity subtracts a strictly enclosed Z-directed finite
// cylinder from an axis-aligned sphere centered at center. offset and axial
// lower/upper are relative to center. Exact rational worst-cap inequalities
// prove clearance greater than eight sewing tolerances. Placement additions
// and cylinder height must be losslessly representable; native analytic charts
// otherwise retain their usual floating evaluation/sewing semantics.
func SphereEnclosedCylinderCavity(ctx context.Context, center geom.Vec3, R, r float64, offset geom.Vec2, lower, upper, tolerance float64) (SphereCylinderCavity, error) {
	fail := func(e error) (SphereCylinderCavity, error) { return SphereCylinderCavity{}, e }
	if ctx == nil || !center.IsFinite() || !finite(tolerance) || tolerance <= 0 {
		return fail(geom.ErrInvalid)
	}
	if e := ctx.Err(); e != nil {
		return fail(e)
	}
	if _, e := geom.NewOffAxisSphereFiniteCylinderCommon(R, r, offset, lower, upper); e != nil {
		return fail(e)
	}
	q := func(x float64) *big.Rat { return new(big.Rat).SetFloat64(x) }
	sq := func(x *big.Rat) *big.Rat { return new(big.Rat).Mul(x, x) }
	safeR := new(big.Rat).Sub(q(R), new(big.Rat).Mul(big.NewRat(8, 1), q(tolerance)))
	if safeR.Sign() <= 0 {
		return fail(geom.ErrDomain)
	}
	d2 := new(big.Rat).Add(sq(q(offset.X)), sq(q(offset.Y)))
	z2 := sq(q(lower))
	if z2.Cmp(sq(q(upper))) < 0 {
		z2 = sq(q(upper))
	}
	a := new(big.Rat).Sub(new(big.Rat).Sub(new(big.Rat).Sub(sq(safeR), z2), d2), sq(q(r)))
	rhs := new(big.Rat).Mul(big.NewRat(4, 1), new(big.Rat).Mul(d2, sq(q(r))))
	if a.Sign() <= 0 || sq(a).Cmp(rhs) <= 0 {
		return fail(geom.ErrDomain)
	}
	heightRat := new(big.Rat).Sub(q(upper), q(lower))
	height, e := geom.RepresentedExactScalar(heightRat)
	if e != nil {
		return fail(e)
	}
	world := geom.Vec3{}
	coords := []*float64{&world.X, &world.Y, &world.Z}
	for i, xy := range [][2]float64{{center.X, offset.X}, {center.Y, offset.Y}, {center.Z, lower}} {
		v, e := geom.RepresentedExactScalar(new(big.Rat).Add(q(xy[0]), q(xy[1])))
		if e != nil {
			return fail(e)
		}
		*coords[i] = v
	}
	frame, e := geom.NewFrame(world, geom.Vec3{X: 1}, geom.Vec3{Z: 1})
	if e != nil {
		return fail(e)
	}
	outer, e := Sphere(center, R)
	if e != nil {
		return fail(e)
	}
	inner, e := Cylinder(frame, r, height)
	if e != nil {
		return fail(e)
	}
	vertices, edges, faces := outer.Vertices(), outer.Edges(), outer.Faces()
	nv, ne, nf := len(vertices), len(edges), len(faces)
	vertices = append(vertices, inner.Vertices()...)
	for _, edge := range inner.Edges() {
		edge.Start += nv
		edge.End += nv
		edges = append(edges, edge)
	}
	for _, face := range inner.Faces() {
		face.Reversed = !face.Reversed
		for i := range face.Boundary {
			face.Boundary[i].Edge += ne
		}
		faces = append(faces, face)
	}
	solid, e := NewSolid(vertices, edges, faces, tolerance)
	if e != nil {
		return fail(e)
	}
	if e = ctx.Err(); e != nil {
		return fail(e)
	}
	volume := new(big.Rat).Sub(new(big.Rat).Mul(big.NewRat(4, 3), new(big.Rat).Mul(sq(q(R)), q(R))), new(big.Rat).Mul(sq(q(r)), heightRat))
	return SphereCylinderCavity{Solid: solid, SphereFace: 0, CylinderWall: nf, LowerFloor: nf + 1, UpperFloor: nf + 2, VolumePiCoefficient: volume}, nil
}
