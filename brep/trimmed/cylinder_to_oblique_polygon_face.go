package trimmed

import (
	"context"
	"math"
	"math/big"
	"plainsolid/brep"
	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

// ObliqueCylinderToFaceProof certifies the ideal finite cap footprint and all
// generators forward. RimHeightError bounds the ideal-trig height difference
// caused by storing the constructor's three rim coefficients in float64; it
// excludes trig evaluation roundoff. TopologyTolerance is the independent
// numerical sewing/validation tolerance. No first-contact or Boolean is proved.
type ObliqueCylinderToFaceProof struct {
	TargetFace                                     int
	Coverage                                       geom.DiskPolygonComparison
	TopologyTolerance, RimHeightError, StockHeight float64
}

// CylinderToObliqueFace supports coordinate-axis sweep directions and a finite
// polygon cap whose raw plane survives the analytic constructor's normal
// normalization exactly. Axial projection certifies selected-face containment.
// Unsupported normalized planes or nonforward generators fail closed. This
// returns a numerically represented analytic cap with an explicit error bound,
// rather than promising exact represented endpoints for every generator.
func CylinderToObliqueFace(ctx context.Context, center, normal geom.Vec3, radius float64, target brep.Solid, faceIndex int, tolerance float64, o geom.DiskPolygonOptions) (Solid, ObliqueCylinderToFaceProof, error) {
	proof := ObliqueCylinderToFaceProof{TargetFace: faceIndex, TopologyTolerance: tolerance}
	if radius <= 0 || math.IsNaN(radius) || math.IsInf(radius, 0) || ctx == nil || !center.IsFinite() || !normal.IsFinite() || math.IsNaN(tolerance) || math.IsInf(tolerance, 0) || tolerance <= 0 {
		return Solid{}, proof, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return Solid{}, proof, e
	}
	axis := -1
	sign := 1.
	for i, q := range [3]float64{normal.X, normal.Y, normal.Z} {
		if q != 0 {
			if axis >= 0 {
				return Solid{}, proof, geom.ErrDomain
			}
			axis = i
			if q < 0 {
				sign = -1
			}
		}
	}
	if axis < 0 {
		return Solid{}, proof, geom.ErrInvalid
	}
	unit := geom.Vec3{}
	switch axis {
	case 0:
		unit.X = sign
	case 1:
		unit.Y = sign
	case 2:
		unit.Z = sign
	}
	faces, edges, vertices := target.Faces(), target.Edges(), target.Vertices()
	if faceIndex < 0 || faceIndex >= len(faces) {
		return Solid{}, proof, geom.ErrInvalid
	}
	face := faces[faceIndex]
	uses := append([][]brep.EdgeUse{face.Boundary}, face.Holes...)
	maxEdges := o.MaxEdges
	if maxEdges == 0 {
		maxEdges = 256
	}
	total := 0
	loops := make([][]geom.Vec3, len(uses))
	for l, wire := range uses {
		if len(wire) < 3 || len(wire) > maxEdges-total {
			return Solid{}, proof, geom.ErrInvalid
		}
		total += len(wire)
		for i, use := range wire {
			if use.Edge < 0 || use.Edge >= len(edges) {
				return Solid{}, proof, geom.ErrInvalid
			}
			edge := edges[use.Edge]
			a, b := edge.Start, edge.End
			if use.Reversed {
				a, b = b, a
			}
			next := wire[(i+1)%len(wire)]
			if next.Edge < 0 || next.Edge >= len(edges) {
				return Solid{}, proof, geom.ErrInvalid
			}
			n := edges[next.Edge].Start
			if next.Reversed {
				n = edges[next.Edge].End
			}
			if b != n || a < 0 || a >= len(vertices) {
				return Solid{}, proof, geom.ErrInvalid
			}
			loops[l] = append(loops[l], vertices[a])
		}
	}
	rat := func(x float64) *big.Rat { return new(big.Rat).SetFloat64(x) }
	vec := func(v geom.Vec3) [3]*big.Rat { return [3]*big.Rat{rat(v.X), rat(v.Y), rat(v.Z)} }
	origin := loops[0][0]
	p0 := vec(origin)
	var raw [3]*big.Rat
	for i := 1; i+1 < len(loops[0]); i++ {
		a, b := vec(loops[0][i]), vec(loops[0][i+1])
		for j := 0; j < 3; j++ {
			a[j].Sub(a[j], p0[j])
			b[j].Sub(b[j], p0[j])
		}
		nonzero := false
		for j := 0; j < 3; j++ {
			k, l := (j+1)%3, (j+2)%3
			raw[j] = new(big.Rat).Sub(new(big.Rat).Mul(a[k], b[l]), new(big.Rat).Mul(a[l], b[k]))
			nonzero = nonzero || raw[j].Sign() != 0
		}
		if nonzero {
			break
		}
	}
	if raw[axis] == nil || raw[axis].Sign() == 0 {
		return Solid{}, proof, geom.ErrDomain
	}
	if raw[axis].Sign()*int(sign) < 0 {
		for i := range raw {
			raw[i].Neg(raw[i])
		}
	}
	dot := func(a, b [3]*big.Rat) *big.Rat {
		q := new(big.Rat)
		for i := 0; i < 3; i++ {
			q.Add(q, new(big.Rat).Mul(a[i], b[i]))
		}
		return q
	}
	for _, loop := range loops {
		for _, v := range loop {
			d := vec(v)
			for i := range d {
				d[i].Sub(d[i], p0[i])
			}
			if dot(raw, d).Sign() != 0 {
				return Solid{}, proof, geom.ErrDomain
			}
		}
	}
	max := new(big.Rat)
	for _, q := range raw {
		if a := new(big.Rat).Abs(q); a.Cmp(max) > 0 {
			max = a
		}
	}
	var nf [3]float64
	for i, q := range raw {
		nf[i], _ = new(big.Rat).Quo(q, max).Float64()
	}
	plane, e := geom.NewPlane(origin, geom.Vec3{X: nf[0], Y: nf[1], Z: nf[2]})
	if e != nil {
		return Solid{}, proof, e
	}
	represented := vec(plane.Normal())
	for i := 0; i < 3; i++ {
		j, k := (i+1)%3, (i+2)%3
		if new(big.Rat).Sub(new(big.Rat).Mul(raw[j], represented[k]), new(big.Rat).Mul(raw[k], represented[j])).Sign() != 0 {
			return Solid{}, proof, geom.ErrUnrepresentable
		}
	}
	frame, e := geom.FrameFromNormal(center, unit)
	if e != nil {
		return Solid{}, proof, e
	}
	den := new(big.Rat).Mul(raw[axis], rat(sign))
	delta := vec(center)
	for i := range delta {
		delta[i].Sub(p0[i], delta[i])
	}
	c0 := new(big.Rat).Quo(dot(raw, delta), den)
	cx := new(big.Rat).Neg(new(big.Rat).Quo(new(big.Rat).Mul(rat(radius), dot(raw, vec(frame.XAxis()))), den))
	cy := new(big.Rat).Neg(new(big.Rat).Quo(new(big.Rat).Mul(rat(radius), dot(raw, vec(frame.YAxis()))), den))
	if c0.Sign() <= 0 || new(big.Rat).Mul(c0, c0).Cmp(new(big.Rat).Add(new(big.Rat).Mul(cx, cx), new(big.Rat).Mul(cy, cy))) <= 0 {
		return Solid{}, proof, geom.ErrDomain
	}
	projected := make([][]geom.Vec3, len(loops))
	for l, loop := range loops {
		for _, p := range loop {
			switch axis {
			case 0:
				p.X = center.X
			case 1:
				p.Y = center.Y
			case 2:
				p.Z = center.Z
			}
			projected[l] = append(projected[l], p)
		}
	}
	disk, e := geom.NewPlanarCircularProfile(center, unit, radius)
	if e != nil {
		return Solid{}, proof, e
	}
	proof.Coverage, e = geom.ClassifyDiskPlanarPolygon(ctx, disk, projected[0], projected[1:], o)
	if e != nil {
		return Solid{}, proof, e
	}
	if proof.Coverage.Relation != geom.DiskPolygonStrictlyCovered && proof.Coverage.Relation != geom.DiskPolygonCoveredTouching {
		return Solid{}, proof, brep.ErrTerminationUncovered
	}
	outward := func(q *big.Rat) float64 {
		f, _ := q.Float64()
		if !math.IsInf(f, 0) && rat(f).Cmp(q) < 0 {
			f = math.Nextafter(f, math.Inf(1))
		}
		return f
	}
	n := plane.Normal()
	z := n.Dot(frame.ZAxis())
	d := plane.SignedDistance(center)
	coefficients := [3]float64{-d / z, -radius * n.Dot(frame.XAxis()) / z, -radius * n.Dot(frame.YAxis()) / z}
	heightError := new(big.Rat)
	for i, q := range []*big.Rat{c0, cx, cy} {
		if math.IsInf(coefficients[i], 0) || math.IsNaN(coefficients[i]) {
			return Solid{}, proof, geom.ErrConvergence
		}
		heightError.Add(heightError, new(big.Rat).Abs(new(big.Rat).Sub(rat(coefficients[i]), q)))
	}
	proof.RimHeightError = outward(heightError)
	if proof.RimHeightError > tolerance {
		return Solid{}, proof, geom.ErrConvergence
	}
	upper := new(big.Rat).Add(c0, new(big.Rat).Add(new(big.Rat).Abs(cx), new(big.Rat).Abs(cy)))
	upper.Add(upper, new(big.Rat).Add(big.NewRat(1, 1), new(big.Rat).Mul(big.NewRat(4, 1), rat(tolerance))))
	h := outward(upper)
	if math.IsInf(h, 0) {
		return Solid{}, proof, geom.ErrConvergence
	}
	proof.StockHeight = h
	if e := ctx.Err(); e != nil {
		return Solid{}, proof, e
	}
	native, e := parametric.ClipCylinder(frame, radius, h, plane, tolerance)
	if e != nil {
		return Solid{}, proof, e
	}
	solid, e := FromParametric(native)
	return solid, proof, e
}
