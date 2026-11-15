package plaincadbridge

import (
	"context"
	"math"
	"math/big"

	"plainsolid/brep/parametric"
	rigidplacement "plainsolid/brep/placement"
	"plainsolid/brep/trimmed"
	"plainsolid/exchange/step"
	"plainsolid/geom"
)

type rationalStepArcLoop struct {
	local      trimmed.Solid
	descriptor step.ExactCircularLoopExtrusion
	pose       rigidplacement.RationalRigid
}

func (b *Bridge) putRationalSTEPArcLoop(ctx context.Context, local trimmed.Solid, d step.ExactCircularLoopExtrusion, pose rigidplacement.RationalRigid) (Shape, error) {
	if d.LengthUnit != .001 || frameMap(d.Frame) != geom.Identity() || !convexStepLoop(d.Segments) {
		return Shape{}, unsupportedEdge("rational-world STEP import requires a convex product-local circular extrusion in millimetres")
	}
	if len(local.Topology().Shells) != 1 {
		return Shape{}, geom.ErrDomain
	}
	bounds, err := rationalStepArcBounds(d, pose)
	if err != nil {
		return Shape{}, err
	}
	p, err := local.MassProperties(ctx, parametric.IntegrationOptions{RelativeTolerance: 1e-10})
	if err != nil {
		return Shape{}, err
	}
	if !finite(p.Volume) || !finite(p.Area) || p.Volume <= 0 || p.Area <= 0 {
		return Shape{}, geom.ErrDomain
	}
	return b.publish(stored{
		rationalStepArc: &rationalStepArcLoop{local: local, descriptor: d, pose: pose},
		result: Shape{
			Bounds:             Bounds{coordinates(bounds.Min), coordinates(bounds.Max)},
			GeometryAssertions: Assertions{Valid: true, Volume: p.Volume, SurfaceArea: p.Area, SolidCount: 1},
		},
	})
}

func (p rationalStepArcLoop) transformed(ctx context.Context, m geom.Affine) (rigidplacement.RationalRigid, error) {
	var matrix [3][4]*big.Rat
	for i, row := range m {
		for j, q := range row {
			matrix[i][j] = new(big.Rat).SetFloat64(q)
		}
	}
	next, err := rigidplacement.NewRationalRigid(matrix, 512)
	if err != nil {
		return rigidplacement.RationalRigid{}, unsupportedEdge("rational-world STEP placement requires a represented exact proper rigid transform")
	}
	return p.pose.Then(ctx, next, 512)
}

func (b *Bridge) checkRationalSTEPArcOperation(q request) error {
	switch q.Op {
	case "importStep", "exportStep", "inspect", "clone", "dispose", "mesh", "stl", "displayEdges", "drawingGeometry", "transform":
		return nil
	}
	for _, handle := range []string{q.Handle, q.A, q.B} {
		if b.shapes[handle].rationalStepArc != nil {
			return unsupportedEdge("operation is outside the rational-world STEP occurrence domain")
		}
	}
	for _, handles := range [][]string{q.Handles, q.Tools} {
		for _, handle := range handles {
			if b.shapes[handle].rationalStepArc != nil {
				return unsupportedEdge("operation is outside the rational-world STEP occurrence domain")
			}
		}
	}
	return nil
}

// rationalStepArcBounds encloses each support extremum with outward binary64
// rounding. Exact cross signs select the extrema of each original minor arc.
// A directed square root bounds the length of the world axis projected in XY.
func rationalStepArcBounds(d step.ExactCircularLoopExtrusion, pose rigidplacement.RationalRigid) (geom.Box, error) {
	matrix, err := pose.Matrix()
	if err != nil {
		return geom.Box{}, err
	}
	var lower, upper vector
	for i, row := range matrix {
		axis := rationalPoint2{row[0], row[1]}
		var lo, hi *big.Rat
		add := func(q *big.Rat) {
			if lo == nil || q.Cmp(lo) < 0 {
				lo = q
			}
			if hi == nil || q.Cmp(hi) > 0 {
				hi = q
			}
		}
		squared := axis.dot(axis)
		sqrt := new(big.Float).SetPrec(256).SetMode(big.ToPositiveInf).SetRat(squared)
		sqrt.Sqrt(sqrt)
		lengthUpper, _ := sqrt.Rat(nil)
		if new(big.Rat).Mul(lengthUpper, lengthUpper).Cmp(squared) < 0 {
			return geom.Box{}, geom.ErrUnrepresentable
		}
		opposite := rationalPoint2{new(big.Rat).Neg(row[0]), new(big.Rat).Neg(row[1])}
		for _, segment := range d.Segments {
			start, end := rationalXY(segment.Start), rationalXY(segment.End)
			add(axis.dot(start))
			add(axis.dot(end))
			if segment.Radius == 0 || squared.Sign() == 0 {
				continue
			}
			center := rationalXY(segment.Center)
			u, v := start.sub(center), end.sub(center)
			offset := new(big.Rat).Mul(lengthUpper, new(big.Rat).SetFloat64(segment.Radius))
			if u.cross(axis).Sign() > 0 && axis.cross(v).Sign() > 0 {
				add(new(big.Rat).Add(axis.dot(center), offset))
			}
			if u.cross(opposite).Sign() > 0 && opposite.cross(v).Sign() > 0 {
				add(new(big.Rat).Sub(axis.dot(center), offset))
			}
		}
		if lo == nil {
			return geom.Box{}, geom.ErrDomain
		}
		axial := new(big.Rat).Mul(row[2], new(big.Rat).SetFloat64(d.Height))
		lo, hi = new(big.Rat).Add(lo, row[3]), new(big.Rat).Add(hi, row[3])
		if axial.Sign() < 0 {
			lo.Add(lo, axial)
		} else {
			hi.Add(hi, axial)
		}
		lower[i], upper[i] = outwardStepArcBound(lo, false), outwardStepArcBound(hi, true)
		if !finite(lower[i]) || !finite(upper[i]) {
			return geom.Box{}, geom.ErrUnrepresentable
		}
	}
	return geom.Box{Min: lower.vec(), Max: upper.vec()}, nil
}

func outwardStepArcBound(q *big.Rat, upper bool) float64 {
	v, _ := q.Float64()
	if !finite(v) {
		return v
	}
	comparison := new(big.Rat).SetFloat64(v).Cmp(q)
	if upper && comparison < 0 {
		return math.Nextafter(v, math.Inf(1))
	}
	if !upper && comparison > 0 {
		return math.Nextafter(v, math.Inf(-1))
	}
	return v
}
