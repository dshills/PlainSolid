//go:build research

package trimmed

import (
	"context"
	"math/big"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

// RoofBoxPointQuery classifies the regularized result of two original roof
// boxes. Its source proof permits shared supports and does not construct a
// result shell. The prepared query is immutable and safe for concurrent use.
type RoofBoxPointQuery struct{ material *MixedRoofMaterialQuery }

// PrepareRoofBoxPointQuery authenticates both original six-face roof boxes.
func PrepareRoofBoxPointQuery(ctx context.Context, a, b Solid, op parametric.BooleanOp, proof MixedRoofQueryOptions) (*RoofBoxPointQuery, error) {
	q, err := prepareDirectRoofBoxMeasure(ctx, a, b, op, proof)
	if err != nil {
		return nil, err
	}
	return &RoofBoxPointQuery{material: q}, nil
}

// LocateRational checks all incident open XY sectors and the vertical layers
// they induce. Shared internal walls and zero-volume contacts are classified
// by nearby material rather than the source faces that meet at the point.
// If a necessary two-dimensional roof order remains unresolved, it returns
// ErrConvergence and no classification. It never picks a numeric epsilon.
func (r *RoofBoxPointQuery) LocateRational(ctx context.Context, p [3]*big.Rat, o geom.SurfacePatchEvaluationOptions) (MixedRoofMaterialPoint, error) {
	var zero MixedRoofMaterialPoint
	if r == nil || r.material == nil {
		return zero, geom.ErrInvalid
	}
	q := r.material
	o, err := q.evaluationOptions(ctx, o, p[:]...)
	if err != nil {
		return zero, err
	}
	out, active, err := q.locateOperandPoints(ctx, p, o)
	if err != nil {
		return zero, err
	}
	w := mixedRoofLineWork{o: MixedRoofLineOptions{SurfacePatchEvaluationOptions: o}, operations: out.ExactOperations}
	return q.regularizePoint(ctx, p, out, active, &w)
}

func (q *MixedRoofMaterialQuery) regularizePoint(ctx context.Context, p [3]*big.Rat, out MixedRoofMaterialPoint, active [2]int, w *mixedRoofLineWork) (MixedRoofMaterialPoint, error) {
	var zero MixedRoofMaterialPoint
	if err := ctx.Err(); err != nil {
		return zero, err
	}
	out.ExactOperations = w.operations
	if out.Operands[0] != parametric.Boundary || out.Operands[1] != parametric.Boundary {
		location, err := mixedRoofBooleanLocation(q.op, out.Operands)
		if err != nil {
			return zero, err
		}
		out.Location = location
		return out, nil
	}
	var certain, possible uint8
	for _, sx := range [...]int{-1, 1} {
		for _, sy := range [...]int{-1, 1} {
			if err := ctx.Err(); err != nil {
				return zero, err
			}
			dir := [2]int{sx, sy}
			keep := [2]bool{}
			for i, b := range q.box {
				keep[i] = roofIncidentSector(b.domain, p, dir)
			}
			var lower, upper [2]bool
			for i := range keep {
				lower[i] = keep[i] && active[i] <= 0
				upper[i] = keep[i] && active[i] >= 0
			}
			certain |= roofPointLayer(q.op, lower) | roofPointLayer(q.op, upper)
			if certain == 3 {
				out.Location, out.ExactOperations = parametric.Boundary, w.operations
				return out, nil
			}
			if !keep[0] || !keep[1] || active[0] == 0 || active[1] == 0 {
				continue
			}
			byOrder := [2]uint8{
				roofPointLayer(q.op, [2]bool{upper[0], lower[1]}),
				roofPointLayer(q.op, [2]bool{lower[0], upper[1]}),
			}
			if (byOrder[0]|byOrder[1])&^certain == 0 || active[0] == 1 && active[1] == 1 {
				continue
			}
			proved, may, err := q.roofIncidentOrder(p, dir, active, w)
			if err != nil {
				return zero, err
			}
			for i, layer := range byOrder {
				bit := uint8(1 << i)
				if proved&bit != 0 {
					certain |= layer
				}
				if may&bit != 0 {
					possible |= layer
				}
			}
			if certain == 3 {
				out.Location, out.ExactOperations = parametric.Boundary, w.operations
				return out, nil
			}
		}
	}
	if possible&^certain != 0 {
		return zero, geom.ErrConvergence
	}
	out.ExactOperations = w.operations
	if certain == 2 {
		out.Location = parametric.Inside
	}
	return out, nil
}

// Bit one represents exterior and bit two material. The four open XY
// quadrants and open Z layers are dense in a neighborhood away from the
// finite support cuts. Taking the closure of occupied open cells implements
// regularization, including points where a cutter merely touches the result.
func roofPointLayer(op parametric.BooleanOp, s [2]bool) uint8 {
	if op == parametric.Union && (s[0] || s[1]) || op == parametric.Intersection && s[0] && s[1] || op == parametric.Difference && s[0] && !s[1] {
		return 2
	}
	return 1
}

func roofIncidentSector(d geom.ParameterRectangle, p [3]*big.Rat, dir [2]int) bool {
	for i, ends := range [2][2]float64{{d.UMin, d.UMax}, {d.VMin, d.VMax}} {
		l, u := p[i].Cmp(ratInset(ends[0])), p[i].Cmp(ratInset(ends[1]))
		if l < 0 || u > 0 || l == 0 && dir[i] < 0 || u == 0 && dir[i] > 0 {
			return false
		}
	}
	return true
}

// Bits apply to strict A<B and A>B. Exact support values at the query point
// agree; the order is that of the first nonzero variation on each sector.
func (q *MixedRoofMaterialQuery) roofIncidentOrder(p [3]*big.Rat, dir, active [2]int, w *mixedRoofLineWork) (proved, possible uint8, err error) {
	var numerator, weight [2]roofPowerGrid
	var axes [2][2]*big.Rat
	for j := range axes {
		v, e := w.rat(new(big.Rat).Add(p[j], big.NewRat(int64(dir[j]), 1)))
		if e != nil {
			return 0, 0, e
		}
		axes[j] = [2]*big.Rat{p[j], v}
	}
	for i, box := range q.box {
		if active[i] > 0 {
			numerator[i], weight[i] = roofPowerGrid{{box.floor}}, roofPowerGrid{{big.NewRat(1, 1)}}
			continue
		}
		if len(box.roofSpans) > 0 {
			var chosen *mixedRoofBox
			for _, span := range box.roofSpans {
				if roofIncidentSector(span.domain, p, dir) {
					chosen = span
					break
				}
			}
			if chosen == nil {
				return 0, 0, geom.ErrConvergence
			}
			box = chosen
		}
		numerator[i], weight[i], err = box.roofOrderPower(axes, w)
		if err != nil {
			return 0, 0, err
		}
	}
	difference, err := roofHeightDifference(numerator, weight, w)
	if err != nil {
		return 0, 0, err
	}
	if difference[0][0].Sign() != 0 {
		return 0, 0, geom.ErrConvergence
	}
	return roofGermSigns(difference, w)
}

// A mixed coefficient is locally dominated when a same-sign candidate has
// componentwise smaller exponents. For existence, substitute x=t^a,y=t^b
// exactly: the first nonzero total coefficient after cancellation has a sign
// that persists arbitrarily close to t=0.
func roofGermSigns(p roofPowerGrid, w *mixedRoofLineWork) (proved, possible uint8, err error) {
	var terms [][3]int
	for i, row := range p {
		for j, c := range row {
			if c.Sign() != 0 {
				terms = append(terms, [3]int{i, j, c.Sign()})
			}
		}
	}
	signBit := func(sign int) uint8 {
		if sign < 0 {
			return 1
		}
		if sign > 0 {
			return 2
		}
		return 0
	}
	var minima uint8
	minY := len(p[0])
	for _, t := range terms {
		if t[1] < minY {
			minima |= signBit(t[2])
			minY = t[1]
		}
	}
	if minima != 3 {
		return minima, minima, nil
	}
	// Test all primitive-small slope regimes where the minimum weight can
	// switch, plus both strongly one-sided paths. Unknown signs stay possible.
	// The degree limit bounds this loop independently of coefficient size.
	n := max(len(p), len(p[0]))
	var order []int
	coefficients := make(map[int]*big.Rat, len(terms))
	for a := 1; a <= n && proved != 3; a++ {
		for b := 1; b <= n && proved != 3; b++ {
			clear(coefficients)
			order = order[:0]
			for _, t := range terms {
				degree := a*t[0] + b*t[1]
				if old := coefficients[degree]; old != nil {
					if _, err := w.rat(old.Add(old, p[t[0]][t[1]])); err != nil {
						return 0, 0, err
					}
				} else {
					c, err := w.rat(new(big.Rat).Set(p[t[0]][t[1]]))
					if err != nil {
						return 0, 0, err
					}
					coefficients[degree] = c
					order = append(order, degree)
				}
			}
			min, sign := (len(p)+len(p[0]))*n, 0
			for _, degree := range order {
				if c := coefficients[degree]; degree < min && c.Sign() != 0 {
					min, sign = degree, c.Sign()
				}
			}
			proved |= signBit(sign)
		}
	}
	return proved, 3, nil
}
