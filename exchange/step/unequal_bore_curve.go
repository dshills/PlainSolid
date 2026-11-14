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
	model   []geom.Vec3
	owners  [2][]geom.Vec3
	compact bool
	smooth  bool
	knots   []float64
}

// approximateUnequalBoreEdge is private to native unequal crossed-bore
// topology. Three cubic Hermite curves share the same parameter and knots.
func approximateUnequalBoreEdge(start, end geom.Vec3, adjacent []longEdgeOwner, surfaces []longEdgeSurface, tolerance float64) (*unequalBoreSplines, error) {
	return approximateUnequalBoreEdgeOptions(start, end, adjacent, surfaces, tolerance, false)
}

// Finite output may reuse checked control rounding and C1 Hermite join removal.
// The existing through representation and all fit/resource budgets stay unchanged.
func approximateUnequalBoreEdgeOptions(start, end geom.Vec3, adjacent []longEdgeOwner, surfaces []longEdgeSurface, tolerance float64, compactFinite bool) (*unequalBoreSplines, error) {
	first, second := surfaces[adjacent[0].face], surfaces[adjacent[1].face]
	a, b := min(first.radius, second.radius), max(first.radius, second.radius)
	if !(a > 0 && b >= a) || first.frame.ZAxis().Dot(second.frame.ZAxis()) != 0 {
		return nil, geom.ErrDomain
	}
	small := 0
	if first.radius > second.radius {
		small = 1
	}
	delta := first.frame.Origin().Sub(second.frame.Origin())
	if delta.Dot(first.frame.ZAxis()) != 0 || delta.Dot(second.frame.ZAxis()) != 0 {
		return nil, geom.ErrDomain
	}
	d := delta.Length()
	partial := a+d > b
	if d >= a+b || a+d == b {
		return nil, geom.ErrDomain
	}
	// The exact-arithmetic Hermite interpolation error
	// is bounded by max(|f''''|)*step^4/384. Compute the derivative
	// envelope using a/q to avoid large powers of the radii themselves.
	q := math.Sqrt((b - a) * (b + a))
	k := a / q
	k2 := k * k
	heightFourth := a * k * (4 + k2*(7+k2*(4.5+k2*15/16)))
	angleFourth := k * (1 + k2*(6.5+k2*(6.75+k2*15/8)))
	if d != 0 && !partial {
		// y=d+a*sin(u), h=sqrt(b²-y²). The offset increases the
		// derivative envelopes for b²-y² and lowers its minimum.
		heightFourth, angleFourth = coveredSkewDerivativeBounds(a, b, a+d)
	}
	envelope, arc := max(2*a+heightFourth, a+b*angleFourth), math.Pi/2
	if partial {
		envelope, arc = partialSkewFourthDerivative(a, b, d), math.Pi/4
	}
	stepSize := math.Pow(384*(tolerance/16)/envelope, .25)
	segments := math.Ceil(arc / stepSize)
	var knots []float64
	if !finiteNumber(segments) || segments < 1 || segments > 128 {
		var err error
		if partial {
			knots, err = partialSkewFitKnots(adjacent, a, b, d, tolerance)
		} else {
			knots, err = coveredSkewFitKnots(adjacent, a, b, d, tolerance)
		}
		if err != nil {
			return nil, err
		}
		segments = float64(len(knots) - 1)
	}
	// Reserve space in the exchange tolerance for floating evaluations,
	// endpoint representatives, and the model-to-owner residual.
	origin := first.frame.Origin()
	scale := max(math.Abs(origin.X), math.Abs(origin.Y), math.Abs(origin.Z), b, 1)
	if 256*0x1p-52*scale > tolerance/16 {
		return nil, geom.ErrUnrepresentable
	}

	n := int(segments)
	result := &unequalBoreSplines{compact: d != 0 || compactFinite, smooth: partial || compactFinite || len(knots) != 0, knots: knots}
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
			span := 1 / float64(n)
			if len(knots) != 0 {
				t = knots[j]
				if j > 0 {
					span = knots[j] - knots[j-1]
				}
			}
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
			incoming, outgoing := tangent, tangent
			if len(knots) != 0 {
				tangent = geom.Vec3{X: d.X, Y: d.Y}.Scale(sign / 3)
				incoming = tangent.Scale(span)
				if j < n {
					outgoing = tangent.Scale(knots[j+1] - knots[j])
				}
			}
			if j == 0 {
				uv = append(uv, position)
			} else {
				uv = append(uv, prevUV.Add(prevTangent), position.Sub(incoming), position)
			}
			prevUV, prevTangent = position, outgoing
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
			inDir := du.Scale(incoming.X).Add(dv.Scale(incoming.Y))
			outDir := du.Scale(outgoing.X).Add(dv.Scale(outgoing.Y))
			if j == 0 {
				model = append(model, point)
			} else {
				model = append(model, prevPoint.Add(prevDirection), point.Sub(inDir), point)
			}
			prevPoint, prevDirection = point, outDir
		}
		for i, poles := range [][]geom.Vec3{uv, model} {
			// A nonrational B-spline is a convex combination of its
			// controls. This physical-space budget applies to all t.
			poleBudget := tolerance / 32
			if i == 0 {
				poleBudget /= max(support.radius, 1)
			}
			for j, pole := range poles {
				if !pole.IsFinite() {
					return nil, geom.ErrUnrepresentable
				}
				if !result.compact || j == 0 || j == len(poles)-1 {
					continue
				}
				if rounded, ok := compactUnequalBoreControl(pole, poleBudget); ok {
					poles[j] = rounded
				}
			}
		}
		result.owners[index] = uv
		if index == small {
			result.model = model
		}
	}
	if result.smooth {
		model, modelOK := compactHermiteJoins(result.model, knots, tolerance/16)
		var owners [2][]geom.Vec3
		ok := modelOK
		for i, owner := range adjacent {
			poles, valid := compactHermiteJoins(result.owners[i], knots, tolerance/(16*max(surfaces[owner.face].radius, 1)))
			owners[i], ok = poles, ok && valid
		}
		result.smooth = ok
		if ok {
			result.model, result.owners = model, owners
		}
	}
	return result, nil
}

// The partial intersection uses sin(u)*sqrt(L*(2b-L*sin²u)) for
// one coordinate and its cosine analogue for the other. The derivatives
// of the radicand have bounds L², 2L², 4L² and 8L². The angle
// derivative has a single sin or cos factor divided by the same radical.
func partialSkewFourthDerivative(a, b, d float64) float64 {
	return partialSkewFourthDerivativeBounds(a, b, d, 1, 1)
}

func partialSkewFourthDerivativeBounds(a, b, d, xFactor, zFactor float64) float64 {
	length := a + b - d
	component := func(radius, factor float64) (cartesian, angle float64) {
		q := math.Sqrt(length * (2*radius - length*factor))
		k := length / q
		k2 := k * k
		g0 := math.Sqrt(2 * radius * length)
		g1 := q * k2 / 2
		g2 := q * k2 * (1 + k2/4)
		g3 := q * k2 * (2 + k2*(1.5+3*k2/8))
		g4 := q * k2 * (4 + k2*(7+k2*(9.0/2+15*k2/16)))
		return g0 + 4*g1 + 6*g2 + 4*g3 + g4, 2 * k * (1 + k2*(6.5+k2*(6.75+k2*15/8)))
	}
	x, zAngle := component(b, xFactor)
	z, xAngle := component(a, zFactor)
	return max(x+z+8*length, a*xAngle+x, b*zAngle+z)
}

// Partition only an edge that exceeds the uniform fit budget. Within each
// angular block the monotone squared sine/cosine bounds tighten the lower
// radicand and therefore the same Hermite fourth-derivative envelope.
func partialSkewFitKnots(adjacent []longEdgeOwner, a, b, d, tolerance float64) ([]float64, error) {
	var from, to float64
	for i, owner := range adjacent {
		arc, ok := owner.use.Curve.(interface{ PartialSkewAngularInterval() (float64, float64) })
		if !ok {
			return nil, geom.ErrDomain
		}
		x, y := arc.PartialSkewAngularInterval()
		if owner.use.Reversed {
			x, y = y, x
		}
		if i == 0 {
			from, to = x, y
		} else if from != x || to != y {
			return nil, geom.ErrDomain
		}
	}
	if min(from, to) < 0 || max(from, to) > math.Pi/2 || from == to {
		return nil, geom.ErrDomain
	}
	return boundedHermiteKnots(from, to, tolerance, 192, func(lo, hi float64) float64 {
		sin, cos := math.Sin(hi), math.Cos(lo)
		pad := 128 * 0x1p-52
		return partialSkewFourthDerivativeBounds(a, b, d, min(1, sin*sin+pad), min(1, cos*cos+pad))
	})
}

// Each covered-width edge stays within a single quadrant of the small
// cylinder. Its transverse coordinate is monotone there. A tighter maximum
// absolute ordinate gives a safe local lower bound for the larger cylinder's
// radical, while the original derivative numerators stay bounded by a.
func coveredSkewFitKnots(adjacent []longEdgeOwner, a, b, d, tolerance float64) ([]float64, error) {
	var from, to, offset float64
	for i, owner := range adjacent {
		arc, ok := owner.use.Curve.(interface {
			CoveredSkewAngularInterval() (float64, float64, float64)
		})
		if !ok {
			return nil, geom.ErrDomain
		}
		x, y, z := arc.CoveredSkewAngularInterval()
		if owner.use.Reversed {
			x, y = y, x
		}
		if i == 0 {
			from, to, offset = x, y, z
		} else if from != x || to != y || offset != z {
			return nil, geom.ErrDomain
		}
	}
	if math.Abs(offset) != d || min(from, to) < 0 || max(from, to) > 2*math.Pi || math.Abs(from-to) != math.Pi/2 {
		return nil, geom.ErrDomain
	}
	return boundedHermiteKnots(from, to, tolerance, 320, func(lo, hi float64) float64 {
		m := max(math.Abs(offset+a*math.Sin(lo)), math.Abs(offset+a*math.Sin(hi)))
		m = min(a+d, m+(a+d)*128*0x1p-52)
		h, angle := coveredSkewDerivativeBounds(a, b, m)
		return max(2*a+h, a+b*angle)
	})
}

func coveredSkewDerivativeBounds(a, b, maxOrdinate float64) (height, angle float64) {
	q := math.Sqrt((b - maxOrdinate) * (b + maxOrdinate))
	da, dq := a/q, maxOrdinate/q
	w1 := 2 * da * dq
	w2 := 2 * da * (da + dq)
	w3 := 2 * da * (3*da + dq)
	w4 := 2 * da * (7*da + dq)
	height = q * (w4/2 + w1*w3 + 3*w2*w2/4 + 9*w1*w1*w2/4 + 15*w1*w1*w1*w1/16)
	// theta'=±a*cos(u)/h. Bound derivatives of 1/h before
	// applying the product rule three more times.
	g1 := w1 / 2
	g2 := w2/2 + 3*w1*w1/4
	g3 := w3/2 + 9*w1*w2/4 + 15*w1*w1*w1/8
	angle = da * (1 + 3*g1 + 3*g2 + g3)
	return height, angle
}

func boundedHermiteKnots(from, to, tolerance float64, maximumSpans int, envelope func(float64, float64) float64) ([]float64, error) {
	const blocks = 16
	knots := []float64{0}
	for i := range blocks {
		start := from + (to-from)*float64(i)/blocks
		end := from + (to-from)*float64(i+1)/blocks
		lo, hi := min(start, end), max(start, end)
		step := math.Pow(384*(tolerance/16)/envelope(lo, hi), .25)
		n := math.Ceil(math.Abs(end-start) * (1 + 128*0x1p-52) / step)
		if !finiteNumber(n) || n < 1 || n > float64(maximumSpans+1-len(knots)) {
			return nil, geom.ErrDomain
		}
		for j := 1; j <= int(n); j++ {
			knots = append(knots, (float64(i)+float64(j)/n)/blocks)
		}
	}
	return knots, nil
}

// A cubic spline with double interior knots recovers the neighboring
// Bézier join from the adjacent controls weighted by their knot spans.
// Removing that join is safe only if its reconstructed displacement
// preserves the fit allowance for every point of the curve.
func compactHermiteJoins(poles []geom.Vec3, knots []float64, budget float64) ([]geom.Vec3, bool) {
	out := make([]geom.Vec3, 0, (len(poles)-1)/3*2+2)
	out = append(out, poles[0])
	for j := 3; j < len(poles)-1; j += 3 {
		mid := poles[j-1].Scale(.5).Add(poles[j+1].Scale(.5))
		if len(knots) != 0 {
			k := j / 3
			w := (knots[k] - knots[k-1]) / (knots[k+1] - knots[k-1])
			mid = poles[j-1].Scale(1 - w).Add(poles[j+1].Scale(w))
		}
		if mid.Distance(poles[j]) > budget {
			return nil, false
		}
		out = append(out, poles[j-2], poles[j-1])
	}
	last := len(poles) - 1
	return append(out, poles[last-2], poles[last-1], poles[last]), true
}

// Keep the endpoints byte-stable. Interior controls can use shorter decimal
// values when rounding fits in the explicit model or parameter-space budget.
func compactUnequalBoreControl(p geom.Vec3, budget float64) (geom.Vec3, bool) {
	round := func(x float64) float64 {
		value, _ := strconv.ParseFloat(strconv.FormatFloat(x, 'g', 11, 64), 64)
		return value
	}
	q := geom.Vec3{X: round(p.X), Y: round(p.Y), Z: round(p.Z)}
	return q, q.IsFinite() && q.Distance(p) <= budget
}

func compactUnequalBoreReal(x float64) string {
	// Use the shortest round-trippable text for the already checked control.
	s := strings.ToUpper(strconv.FormatFloat(x, 'g', -1, 64))
	offset := strings.IndexByte(s, 'E')
	if offset < 0 {
		offset = len(s)
	}
	if !strings.Contains(s[:offset], ".") {
		s = s[:offset] + "." + s[offset:]
	}
	return s
}

func emitUnequalBoreSpline(emit func(string) int64, poles []geom.Vec3, smooth bool, stations []float64, point func(geom.Vec3) int64) int64 {
	ids := make([]int64, len(poles))
	for i, p := range poles {
		ids[i] = point(p)
	}
	segments := (len(poles) - 1) / 3
	interior := "3"
	if smooth {
		segments, interior = (len(poles)-2)/2, "2"
	}
	multiplicities, knots := make([]string, segments+1), make([]string, segments+1)
	for i := range knots {
		multiplicities[i] = interior
		knots[i] = strconv.Itoa(i) + "."
		if len(stations) != 0 {
			knots[i] = real(stations[i])
		}
	}
	multiplicities[0], multiplicities[segments] = "4", "4"
	return emit(fmt.Sprintf("B_SPLINE_CURVE_WITH_KNOTS('',3,%s,.UNSPECIFIED.,.F.,.F.,(%s),(%s),.UNSPECIFIED.)", references(ids), strings.Join(multiplicities, ","), strings.Join(knots, ",")))
}
