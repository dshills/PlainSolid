package step

import (
	"fmt"
	"math"
	"strconv"
	"strings"

	"plainsolid/brep/trimmed"
	"plainsolid/geom"
)

type differentiatedUV interface {
	trimmed.PCurve
	DerivativeUV(float64) (geom.Vec2, error)
}

type unequalBoreSplines struct {
	model  []geom.Vec3
	owners [2][]geom.Vec3
}

// approximateUnequalBoreEdge is private to native unequal crossed-bore
// topology. Three cubic Hermite curves share the same parameter and knots.
func approximateUnequalBoreEdge(start, end geom.Vec3, adjacent []longEdgeOwner, surfaces []longEdgeSurface, tolerance float64) (*unequalBoreSplines, error) {
	first, second := surfaces[adjacent[0].face], surfaces[adjacent[1].face]
	a, b := min(first.radius, second.radius), max(first.radius, second.radius)
	if !(a > 0 && b > a) || first.frame.Origin() != second.frame.Origin() || first.frame.ZAxis().Dot(second.frame.ZAxis()) != 0 {
		return nil, geom.ErrDomain
	}
	small := 0
	if first.radius > second.radius {
		small = 1
	}
	// For q²=b²-a² the exact-arithmetic Hermite interpolation error
	// is bounded by max(|f''''|)*step^4/384. Compute the derivative
	// envelope using a/q to avoid large powers of the radii themselves.
	q := math.Sqrt((b - a) * (b + a))
	k := a / q
	k2 := k * k
	heightFourth := a * k * (4 + k2*(7+k2*(4.5+k2*15/16)))
	angleFourth := k * (1 + k2*(6.5+k2*(6.75+k2*15/8)))
	envelope := max(2*a+heightFourth, a+b*angleFourth)
	stepSize := math.Pow(384*(tolerance/16)/envelope, .25)
	segments := math.Ceil((math.Pi / 2) / stepSize)
	if !finiteNumber(segments) || segments < 1 || segments > 128 {
		return nil, geom.ErrDomain
	}
	// Reserve space in the exchange tolerance for floating evaluations,
	// endpoint representatives, and the model-to-owner residual.
	origin := first.frame.Origin()
	scale := max(math.Abs(origin.X), math.Abs(origin.Y), math.Abs(origin.Z), b, 1)
	if 256*0x1p-52*scale > tolerance/16 {
		return nil, geom.ErrUnrepresentable
	}

	n := int(segments)
	result := &unequalBoreSplines{}
	for index, owner := range adjacent {
		curve, ok := owner.use.Curve.(differentiatedUV)
		if !ok {
			return nil, geom.ErrDomain
		}
		support := surfaces[owner.face]
		surface, err := geom.NewCylinderSurface(support.frame, support.radius)
		if err != nil {
			return nil, err
		}
		uv := make([]geom.Vec3, 0, 3*n+1)
		var model []geom.Vec3
		if index == small {
			model = make([]geom.Vec3, 0, 3*n+1)
		}
		var prevUV, prevTangent, prevPoint, prevDirection geom.Vec3
		for j := range n + 1 {
			t := float64(j) / float64(n)
			sign := 1.
			if owner.use.Reversed {
				t = 1 - t
				sign = -1
			}
			p, err := curve.EvalUV(t)
			if err != nil {
				return nil, err
			}
			d, err := curve.DerivativeUV(t)
			if err != nil {
				return nil, err
			}
			position := geom.Vec3{X: p.X, Y: p.Y}
			tangent := geom.Vec3{X: d.X, Y: d.Y}.Scale(sign / (3 * float64(n)))
			if j == 0 {
				uv = append(uv, position)
			} else {
				uv = append(uv, prevUV.Add(prevTangent), position.Sub(tangent), position)
			}
			prevUV, prevTangent = position, tangent
			endpoint := j == 0 || j == n
			if index != small && !endpoint {
				continue
			}
			point, err := surface.Eval(p.X, p.Y)
			if err != nil {
				return nil, err
			}
			if endpoint {
				vertex := start
				if j == n {
					vertex = end
				}
				if point.Distance(vertex) > tolerance/16 {
					return nil, geom.ErrUnrepresentable
				}
				point = vertex
			}
			if index != small {
				continue
			}
			du, dv, err := surface.Partials(p.X, p.Y)
			if err != nil {
				return nil, err
			}
			direction := du.Scale(tangent.X).Add(dv.Scale(tangent.Y))
			if j == 0 {
				model = append(model, point)
			} else {
				model = append(model, prevPoint.Add(prevDirection), point.Sub(direction), point)
			}
			prevPoint, prevDirection = point, direction
		}
		for _, poles := range [][]geom.Vec3{uv, model} {
			for _, pole := range poles {
				if !pole.IsFinite() {
					return nil, geom.ErrUnrepresentable
				}
			}
		}
		result.owners[index] = uv
		if index == small {
			result.model = model
		}
	}
	return result, nil
}

func emitUnequalBoreSpline(emit func(string) int64, poles []geom.Vec3, point func(geom.Vec3) int64) int64 {
	ids := make([]int64, len(poles))
	for i, p := range poles {
		ids[i] = point(p)
	}
	segments := (len(poles) - 1) / 3
	multiplicities, knots := make([]string, segments+1), make([]string, segments+1)
	for i := range knots {
		multiplicities[i] = "3"
		knots[i] = strconv.Itoa(i) + "."
	}
	multiplicities[0], multiplicities[segments] = "4", "4"
	return emit(fmt.Sprintf("B_SPLINE_CURVE_WITH_KNOTS('',3,%s,.UNSPECIFIED.,.F.,.F.,(%s),(%s),.UNSPECIFIED.)", references(ids), strings.Join(multiplicities, ","), strings.Join(knots, ",")))
}
