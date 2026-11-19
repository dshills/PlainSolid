package boolean

import (
	"math"

	"plainsolid/brep/trimmed"
	"plainsolid/geom"
)

// tolerances is the tolerance model of one call (section 6).
type tolerances struct {
	eps     float64 // working tolerance ε (T1)
	angular float64 // bound on a sine or cosine (T5)
	margin  float64 // MaxGrowth × ε: the largest node radius (T4) and the filter margin μ
	size    float64 // diagonal L of the box of both operands; zero when both are empty
}

// working derives ε from the options and the operands (T1). An empty
// operand contributes no box and no tolerance.
func working(o Options, a, b trimmed.Solid, empty [2]bool) (tolerances, error) {
	t := tolerances{eps: o.Linear, angular: o.Angular}
	var box geom.Box
	some := false
	for i, s := range [2]trimmed.Solid{a, b} {
		if empty[i] {
			continue
		}
		bounds, err := s.Bounds()
		if err != nil {
			return tolerances{}, fail(ErrUnsupported, "intake", "operand %s has no bounds: %v", operandName(i), err)
		}
		if !some {
			box, some = bounds, true
		} else if box, err = box.Union(bounds); err != nil {
			return tolerances{}, fail(ErrInvalid, "intake", "operand %s: %v", operandName(i), err)
		}
		t.eps = math.Max(t.eps, s.Tolerance())
	}
	if some {
		t.size = box.Size().Length()
		t.eps = math.Max(t.eps, o.Relative*t.size)
		if math.IsNaN(t.size) || math.IsInf(t.size, 0) || t.eps >= t.size/100 {
			return tolerances{}, fail(ErrInvalid, "intake", "working tolerance %g is not below a hundredth of the model size %g", t.eps, t.size)
		}
	}
	t.margin = o.MaxGrowth * t.eps
	return t, nil
}

// bounds is an axis-aligned range of space, used to filter pairs.
type bounds struct{ lo, hi geom.Vec3 }

// around is the bounds of a list of points, which must not be empty.
func around(points ...geom.Vec3) bounds {
	b := bounds{points[0], points[0]}
	for _, p := range points[1:] {
		b.lo = geom.Vec3{X: math.Min(b.lo.X, p.X), Y: math.Min(b.lo.Y, p.Y), Z: math.Min(b.lo.Z, p.Z)}
		b.hi = geom.Vec3{X: math.Max(b.hi.X, p.X), Y: math.Max(b.hi.Y, p.Y), Z: math.Max(b.hi.Z, p.Z)}
	}
	return b
}

// with is the smallest bounds that holds both.
func (b bounds) with(c bounds) bounds { return around(b.lo, b.hi, c.lo, c.hi) }

// grown widens the bounds by d on every side.
func (b bounds) grown(d float64) bounds {
	pad := geom.Vec3{X: d, Y: d, Z: d}
	return bounds{b.lo.Sub(pad), b.hi.Add(pad)}
}

// clear reports whether two bounds, each grown by d, do not overlap.
func (b bounds) clear(c bounds, d float64) bool {
	return b.lo.X-c.hi.X > 2*d || c.lo.X-b.hi.X > 2*d ||
		b.lo.Y-c.hi.Y > 2*d || c.lo.Y-b.hi.Y > 2*d ||
		b.lo.Z-c.hi.Z > 2*d || c.lo.Z-b.hi.Z > 2*d
}

// table is a zeroed slice of T with one entry for each key.
func table[T, K any](keys []K) []T { return make([]T, len(keys)) }

// filled is a table with every entry set to value.
func filled[T, K any](keys []K, value T) []T {
	out := table[T](keys)
	for i := range out {
		out[i] = value
	}
	return out
}
