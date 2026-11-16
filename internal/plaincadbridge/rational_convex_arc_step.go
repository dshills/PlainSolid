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
	mmPerUnit  *big.Rat
	hole       *trimmed.ExtrusionCircle
}

func rationalSTEPFileUnit(lengthUnit float64) (stepFileUnit, *big.Rat) {
	switch lengthUnit {
	case .001:
		return stepMillimetre, big.NewRat(1, 1)
	case 1:
		return stepMetre, big.NewRat(1000, 1)
	case .0254:
		return stepInch, big.NewRat(127, 5)
	default:
		return "", nil
	}
}

func checkRationalSTEPUnit(lengthUnit float64, requested stepFileUnit) error {
	actual, _ := rationalSTEPFileUnit(lengthUnit)
	if requested == "" {
		requested = stepMillimetre
	}
	if actual != requested {
		return &failureError{"INVALID", "STEP import unit must match the retained source unit"}
	}
	return nil
}

func (b *Bridge) putRationalSTEPArcLoop(ctx context.Context, local trimmed.Solid, d step.ExactCircularLoopExtrusion, pose rigidplacement.RationalRigid) (Shape, error) {
	return b.putRationalSTEPArcLoopWithHole(ctx, local, d, pose, nil)
}

func (b *Bridge) putRationalSTEPArcLoopWithHole(ctx context.Context, local trimmed.Solid, d step.ExactCircularLoopExtrusion, pose rigidplacement.RationalRigid, hole *trimmed.ExtrusionCircle) (Shape, error) {
	unit, scale := rationalSTEPFileUnit(d.LengthUnit)
	if scale == nil || frameMap(d.Frame) != geom.Identity() || !convexStepLoop(d.Segments) {
		return Shape{}, unsupportedEdge("rational-world STEP import requires a convex product-local circular extrusion with a qualified source unit")
	}
	if len(local.Topology().Shells) != 1 {
		return Shape{}, geom.ErrDomain
	}
	bounds, err := rationalStepArcBounds(d, pose, scale)
	if err != nil {
		return Shape{}, err
	}
	var volume, area float64
	if hole == nil {
		p, err := local.MassProperties(ctx, parametric.IntegrationOptions{RelativeTolerance: 1e-10})
		if err != nil {
			return Shape{}, err
		}
		volume, area = p.Volume, p.Area
	} else {
		volume, area = measureStepLoopThroughHole(d, *hole)
	}
	if !finite(volume) || !finite(area) {
		return Shape{}, geom.ErrDomain
	}
	var explicitUnit stepFileUnit
	if unit != stepMillimetre {
		explicitUnit = unit
		squared := new(big.Rat).Mul(scale, scale)
		cubed := new(big.Rat).Mul(squared, scale)
		volume, _ = new(big.Rat).Mul(cubed, new(big.Rat).SetFloat64(volume)).Float64()
		area, _ = new(big.Rat).Mul(squared, new(big.Rat).SetFloat64(area)).Float64()
	}
	if !finite(volume) || !finite(area) || volume <= 0 || area <= 0 {
		return Shape{}, geom.ErrDomain
	}
	return b.publish(stored{
		rationalStepArc: &rationalStepArcLoop{local: local, descriptor: d, pose: pose, mmPerUnit: scale, hole: hole},
		result: Shape{
			Bounds:             Bounds{coordinates(bounds.Min), coordinates(bounds.Max)},
			GeometryAssertions: Assertions{Valid: true, Volume: volume, SurfaceArea: area, SolidCount: 1},
			StepUnits:          explicitUnit,
		},
	})
}

// The proved convex cap has a chord-polygon area plus one circular segment
// per arc. The complete axial bore subtracts two disks and adds its wall.
func measureStepLoopThroughHole(d step.ExactCircularLoopExtrusion, hole trimmed.ExtrusionCircle) (float64, float64) {
	origin := rationalXY(d.Segments[0].Start)
	polygon := new(big.Rat)
	arcArea, perimeter := 0., 0.
	for _, s := range d.Segments {
		start, end := rationalXY(s.Start), rationalXY(s.End)
		polygon.Add(polygon, start.sub(origin).cross(end.sub(origin)))
		if s.Radius == 0 {
			perimeter += math.Hypot(s.End.X-s.Start.X, s.End.Y-s.Start.Y)
			continue
		}
		center := rationalXY(s.Center)
		u, v := start.sub(center), end.sub(center)
		cross, _ := u.cross(v).Float64()
		dot, _ := u.dot(v).Float64()
		angle := math.Atan2(cross, dot)
		perimeter += s.Radius * angle
		arcArea += (s.Radius*s.Radius*angle - cross) / 2
	}
	chords, _ := polygon.Float64()
	cap := chords/2 + arcArea - math.Pi*hole.Radius*hole.Radius
	return cap * d.Height, 2*cap + (perimeter+2*math.Pi*hole.Radius)*d.Height
}

func (p rationalStepArcLoop) transformed(ctx context.Context, m geom.Affine) (rigidplacement.RationalRigid, error) {
	var matrix [3][4]*big.Rat
	for i, row := range m {
		for j, q := range row {
			matrix[i][j] = new(big.Rat).SetFloat64(q)
		}
		// Placement commands use world millimetres. Compose with the original
		// exact pose in its source length unit.
		matrix[i][3].Quo(matrix[i][3], p.mmPerUnit)
	}
	next, err := rigidplacement.NewRationalRigid(matrix, 512)
	if err != nil {
		return rigidplacement.RationalRigid{}, unsupportedEdge("rational-world STEP placement requires a represented exact proper rigid transform")
	}
	return p.pose.Then(ctx, next, 512)
}

func (b *Bridge) checkRetainedSTEPOperation(q request) error {
	switch q.Op {
	case "importStep", "exportStep", "inspect", "clone", "dispose", "mesh", "stl":
		return nil
	}
	arcReference := q.Op == "displayEdges" || q.Op == "drawingGeometry" || q.Op == "transform"
	handles := append([]string{q.Handle, q.A, q.B}, q.Handles...)
	handles = append(handles, q.Tools...)
	for _, handle := range handles {
		body := b.shapes[handle]
		if body.mappedChainSTEP != nil {
			return unsupportedEdge("operation is outside the ordered mapped STEP source domain")
		}
		if body.rationalStepArc != nil && !arcReference {
			return unsupportedEdge("operation is outside the rational-world STEP occurrence domain")
		}
	}
	return nil
}

// rationalStepArcBounds encloses each support extremum with outward binary64
// rounding. Exact cross signs select the extrema of each original minor arc.
// A directed square root bounds the length of the world axis projected in XY.
func rationalStepArcBounds(d step.ExactCircularLoopExtrusion, pose rigidplacement.RationalRigid, scale *big.Rat) (geom.Box, error) {
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
		lower[i] = outwardStepArcBound(lo.Mul(lo, scale), false)
		upper[i] = outwardStepArcBound(hi.Mul(hi, scale), true)
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
