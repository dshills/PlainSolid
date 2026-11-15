package trimmed

import (
	"context"
	"fmt"
	"math/big"

	"plainsolid/geom"
)

// CircularSweepOffsetOptions bounds original and final sweep construction and
// the exact interval boxes used to separate all nonadjacent moving supports.
type CircularSweepOffsetOptions struct {
	Sweep        ExactCircularLoopOptions
	MaxPairBoxes int
}

// CircularSweepSideOffset retains the original ring and segment ordering.
// Only lateral supports move; the original cap elevations remain fixed.
type CircularSweepSideOffset struct {
	Profile          ExactCircularSweep
	Solid            Solid
	CheckedPairBoxes int
}

type sweepOffsetPoint [2]*big.Rat
type sweepOffsetSegment struct {
	normal    [2]sweepOffsetPoint
	dr        *big.Rat
	controls  [][3]*big.Rat
	distance  *big.Rat
	ring      int
	part      int
	ringCount int
}

func sweepOffsetPointOf(p geom.Vec2) sweepOffsetPoint {
	return sweepOffsetPoint{ratInset(p.X), ratInset(p.Y)}
}
func sweepOffsetSub(a, b sweepOffsetPoint) sweepOffsetPoint {
	return sweepOffsetPoint{new(big.Rat).Sub(a[0], b[0]), new(big.Rat).Sub(a[1], b[1])}
}
func sweepOffsetDot(a, b sweepOffsetPoint) *big.Rat {
	return new(big.Rat).Add(new(big.Rat).Mul(a[0], b[0]), new(big.Rat).Mul(a[1], b[1]))
}
func sweepOffsetCross(a, b sweepOffsetPoint) *big.Rat {
	return new(big.Rat).Sub(new(big.Rat).Mul(a[0], b[1]), new(big.Rat).Mul(a[1], b[0]))
}
func sweepOffsetScale(a sweepOffsetPoint, b *big.Rat) sweepOffsetPoint {
	return sweepOffsetPoint{new(big.Rat).Mul(a[0], b), new(big.Rat).Mul(a[1], b)}
}

func sweepOffsetEndpoint(a, b sweepOffsetPoint, bothLines bool, d *big.Rat) (sweepOffsetPoint, error) {
	if a[0].Cmp(b[0]) == 0 && a[1].Cmp(b[1]) == 0 {
		return sweepOffsetScale(a, d), nil
	}
	det := sweepOffsetCross(a, b)
	if !bothLines || det.Sign() == 0 {
		return sweepOffsetPoint{}, fmt.Errorf("trimmed: offset join needs tangent curved supports or two nonparallel lines: %w", geom.ErrDomain)
	}
	m := new(big.Rat).Quo(d, det)
	return sweepOffsetScale(sweepOffsetPoint{new(big.Rat).Sub(b[1], a[1]), new(big.Rat).Sub(a[0], b[0])}, m), nil
}

func sweepOffsetExactRoot(x *big.Rat) (*big.Rat, error) {
	n, y := new(big.Int).Sqrt(x.Num()), new(big.Int).Sqrt(x.Denom())
	s := new(big.Rat).SetFrac(n, y)
	if s.Sign() == 0 || new(big.Rat).Mul(s, s).Cmp(x) != 0 {
		return nil, fmt.Errorf("trimmed: offset has no positive rational support length: %w", geom.ErrUnrepresentable)
	}
	return s, nil
}

func inspectSweepOffsetSegment(s ExactCircularLoopSegment, d *big.Rat) (sweepOffsetSegment, error) {
	var data sweepOffsetSegment
	var err error
	data.controls, err = exactCircularLoopControls(s, true)
	if err != nil {
		return data, err
	}
	p, q := sweepOffsetPointOf(s.Start), sweepOffsetPointOf(s.End)
	if s.Radius == 0 {
		v := sweepOffsetSub(q, p)
		length, err := sweepOffsetExactRoot(sweepOffsetDot(v, v))
		if err != nil {
			return data, err
		}
		n := sweepOffsetScale(sweepOffsetPoint{v[1], new(big.Rat).Neg(v[0])}, new(big.Rat).Inv(length))
		data.normal = [2]sweepOffsetPoint{n, n}
		data.dr = new(big.Rat)
	} else {
		c := sweepOffsetPointOf(s.Center)
		p, q = sweepOffsetSub(p, c), sweepOffsetSub(q, c)
		sign := big.NewRat(int64(sweepOffsetCross(p, q).Sign()), 1)
		r := ratInset(s.Radius)
		scale := new(big.Rat).Quo(sign, r)
		data.normal = [2]sweepOffsetPoint{sweepOffsetScale(p, scale), sweepOffsetScale(q, scale)}
		data.dr = new(big.Rat).Mul(sign, d)
	}
	return data, nil
}

// OffsetExactCircularSweepSide offsets finite model-Z lateral supports by a
// signed material-normal distance while leaving both caps at their original Z
// levels. Parallel lines and concentric arcs keep their original semantics.
// Line junctions use exact miters; a junction with a curve must be tangent.
// Every nonadjacent pair is proved separate for the entire motion from zero
// to distance, so topological changes are explicitly rejected.
func OffsetExactCircularSweepSide(ctx context.Context, source ExactCircularSweep, distance float64, options CircularSweepOffsetOptions) (CircularSweepSideOffset, error) {
	var zero CircularSweepSideOffset
	if ctx == nil || !finite(distance) || options.MaxPairBoxes < 0 || options.MaxPairBoxes > 1000000 {
		return zero, geom.ErrInvalid
	}
	if err := ctx.Err(); err != nil {
		return zero, err
	}
	stock, err := ExtrudeExactDirectedCircularLoops(ctx, source.Outer, source.Holes, source.Bottom, source.Top, options.Sweep)
	if err != nil {
		return zero, err
	}
	data := ExactCircularSweep{Bottom: source.Bottom, Top: source.Top}
	rings := append([][]ExactCircularLoopSegment{source.Outer}, source.Holes...)
	if distance == 0 {
		data.Outer = append([]ExactCircularLoopSegment(nil), source.Outer...)
		for _, ring := range source.Holes {
			data.Holes = append(data.Holes, append([]ExactCircularLoopSegment(nil), ring...))
		}
		return CircularSweepSideOffset{Profile: data, Solid: stock}, nil
	}
	d := ratInset(distance)
	var moving []sweepOffsetSegment
	for wire, ring := range rings {
		parts := make([]sweepOffsetSegment, len(ring))
		for i, s := range ring {
			parts[i], err = inspectSweepOffsetSegment(s, d)
			if err != nil {
				return zero, err
			}
			parts[i].ring, parts[i].part, parts[i].ringCount = wire, i, len(ring)
		}
		points := make([]geom.Vec2, len(ring))
		deltas := make([]sweepOffsetPoint, len(ring))
		for i, p := range ring {
			prev := (i + len(ring) - 1) % len(ring)
			v, err := sweepOffsetEndpoint(parts[prev].normal[1], parts[i].normal[0], ring[prev].Radius == 0 && p.Radius == 0, d)
			if err != nil {
				return zero, err
			}
			deltas[i] = v
			old := sweepOffsetPointOf(p.Start)
			x, e := geom.RepresentedExactScalar(new(big.Rat).Add(old[0], v[0]))
			if e != nil {
				return zero, e
			}
			y, e := geom.RepresentedExactScalar(new(big.Rat).Add(old[1], v[1]))
			if e != nil {
				return zero, e
			}
			points[i] = geom.Vec2{X: x, Y: y}
		}
		out := make([]ExactCircularLoopSegment, len(ring))
		for i, p := range ring {
			next := (i + 1) % len(ring)
			q := p
			q.Start, q.End = points[i], points[next]
			bound := new(big.Rat)
			if p.Radius == 0 {
				if sweepOffsetDot(sweepOffsetSub(sweepOffsetPointOf(q.End), sweepOffsetPointOf(q.Start)), sweepOffsetSub(sweepOffsetPointOf(p.End), sweepOffsetPointOf(p.Start))).Sign() <= 0 {
					return zero, fmt.Errorf("trimmed: offset collapses an original straight span: %w", geom.ErrDomain)
				}
				for _, v := range []sweepOffsetPoint{deltas[i], deltas[next]} {
					for _, x := range v {
						if a := new(big.Rat).Abs(x); a.Cmp(bound) > 0 {
							bound = a
						}
					}
				}
			} else {
				r := new(big.Rat).Add(ratInset(p.Radius), parts[i].dr)
				if r.Sign() <= 0 {
					return zero, fmt.Errorf("trimmed: offset collapses an original circular support: %w", geom.ErrDomain)
				}
				q.Radius, err = geom.RepresentedExactScalar(r)
				if err != nil {
					return zero, err
				}
				bound.Abs(d)
			}
			parts[i].distance = bound
			out[i] = q
		}
		moving = append(moving, parts...)
		if wire == 0 {
			data.Outer = out
		} else {
			data.Holes = append(data.Holes, out)
		}
	}
	boxes, err := sweepOffsetNonadjacentPairs(ctx, moving, options.MaxPairBoxes)
	if err != nil {
		return zero, err
	}
	solid, err := ExtrudeExactDirectedCircularLoops(ctx, data.Outer, data.Holes, data.Bottom, data.Top, options.Sweep)
	if err != nil {
		return zero, err
	}
	return CircularSweepSideOffset{Profile: data, Solid: solid, CheckedPairBoxes: boxes}, nil
}

type sweepOffsetHullBox [2]sweepOffsetPoint

func sweepOffsetHull(p [][3]*big.Rat) sweepOffsetHullBox {
	var out sweepOffsetHullBox
	for j := 0; j < 2; j++ {
		for _, a := range p {
			x := new(big.Rat).Quo(a[j], a[2])
			if out[0][j] == nil || x.Cmp(out[0][j]) < 0 {
				out[0][j] = x
			}
			if out[1][j] == nil || x.Cmp(out[1][j]) > 0 {
				out[1][j] = x
			}
		}
	}
	return out
}

func sweepOffsetHullWidth(p sweepOffsetHullBox) *big.Rat {
	a, b := new(big.Rat).Sub(p[1][0], p[0][0]), new(big.Rat).Sub(p[1][1], p[0][1])
	if a.Cmp(b) > 0 {
		return a
	}
	return b
}

func sweepOffsetSplit(p [][3]*big.Rat) [2][][3]*big.Rat {
	w := append([][3]*big.Rat(nil), p...)
	n := len(w)
	children := [2][][3]*big.Rat{make([][3]*big.Rat, n), make([][3]*big.Rat, n)}
	children[0][0], children[1][n-1] = w[0], w[n-1]
	half := big.NewRat(1, 2)
	for i := 1; i < n; i++ {
		for j := 0; j < n-i; j++ {
			for axis := 0; axis < 3; axis++ {
				w[j][axis] = new(big.Rat).Mul(new(big.Rat).Add(w[j][axis], w[j+1][axis]), half)
			}
		}
		children[0][i], children[1][n-1-i] = w[0], w[n-i-1]
	}
	return children
}

func sweepOffsetNonadjacentPairs(ctx context.Context, segments []sweepOffsetSegment, limit int) (int, error) {
	if limit == 0 {
		limit = 100000
	}
	checked := 0
	type cell struct {
		a, b  [][3]*big.Rat
		depth int
	}
	for i, a := range segments {
		for j, b := range segments[:i] {
			if a.ring == b.ring && (a.part-b.part == 1 || a.part-b.part == a.ringCount-1) {
				continue
			}
			d := new(big.Rat).Add(a.distance, b.distance)
			stack := []cell{{a.controls, b.controls, 0}}
			for len(stack) > 0 {
				if err := ctx.Err(); err != nil {
					return checked, err
				}
				if checked == limit {
					return checked, fmt.Errorf("trimmed: offset support separation budget: %w", geom.ErrConvergence)
				}
				checked++
				v := stack[len(stack)-1]
				stack = stack[:len(stack)-1]
				A, B := sweepOffsetHull(v.a), sweepOffsetHull(v.b)
				separate := false
				for axis := 0; axis < 2; axis++ {
					separate = separate || new(big.Rat).Sub(A[0][axis], B[1][axis]).Cmp(d) > 0 ||
						new(big.Rat).Sub(B[0][axis], A[1][axis]).Cmp(d) > 0
				}
				if separate {
					continue
				}
				if v.depth >= 48 {
					return checked, fmt.Errorf("trimmed: cannot prove offset path clearance for source segments %d and %d: %w", j, i, geom.ErrConvergence)
				}
				// Positive homogeneous weights keep all points within each
				// restricted control hull. The coordinate displacement bound
				// holds at every intermediate offset and every curve parameter.
				if sweepOffsetHullWidth(A).Cmp(sweepOffsetHullWidth(B)) >= 0 {
					k := sweepOffsetSplit(v.a)
					stack = append(stack, cell{k[0], v.b, v.depth + 1}, cell{k[1], v.b, v.depth + 1})
				} else {
					k := sweepOffsetSplit(v.b)
					stack = append(stack, cell{v.a, k[0], v.depth + 1}, cell{v.a, k[1], v.depth + 1})
				}
			}
		}
	}
	return checked, nil
}
