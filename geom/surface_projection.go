package geom

import (
	"container/heap"
	"fmt"
	"math"
)

// SplineSurfaceProjectionOptions bounds subdivision and queued control-net
// memory. Zero values choose distance tolerance 1e-6, 8192 splits, and 262144
// queued control points. The control-hull gap is a numerical error estimate.
type SplineSurfaceProjectionOptions struct {
	DistanceTolerance         float64
	MaxSplits, MaxQueuedPoles int
}

// SplineSurfaceProjection gives one closest-point candidate, not an enumeration
// of all equal minima. ErrorBound has the same numerical, noncertified meaning
// as CurveProjection.ErrorBound; it bounds the remaining distance gap estimate.
type SplineSurfaceProjection struct {
	U, V                             float64
	Point                            Vec3
	Distance, LowerBound, ErrorBound float64
	Splits                           int
}
type surfaceProjectionNode struct {
	surface *BSplineSurface
	bound   float64
	splitU  bool
	order   int
}
type surfaceProjectionQueue []surfaceProjectionNode

func (q surfaceProjectionQueue) Len() int { return len(q) }
func (q surfaceProjectionQueue) Less(i, j int) bool {
	if q[i].bound == q[j].bound {
		return q[i].order < q[j].order
	}
	return q[i].bound < q[j].bound
}
func (q surfaceProjectionQueue) Swap(i, j int) { q[i], q[j] = q[j], q[i] }
func (q *surfaceProjectionQueue) Push(x any)   { *q = append(*q, x.(surfaceProjectionNode)) }
func (q *surfaceProjectionQueue) Pop() any     { a := *q; n := len(a) - 1; v := a[n]; *q = a[:n]; return v }
func surfaceHullDistance(p Vec3, s *BSplineSurface) (float64, error) {
	box, e := Bounds(s.poles)
	if e != nil {
		return 0, e
	}
	delta := Vec3{math.Max(box.Min.X-p.X, math.Max(0, p.X-box.Max.X)), math.Max(box.Min.Y-p.Y, math.Max(0, p.Y-box.Max.Y)), math.Max(box.Min.Z-p.Z, math.Max(0, p.Z-box.Max.Z))}
	distance := delta.Length()
	if !finite(distance) {
		return 0, ErrInvalid
	}
	a, b, c, d := s.Domain()
	middle, e := s.Eval(a+(b-a)/2, c+(d-c)/2)
	if e != nil {
		return 0, e
	}
	normal, e := p.Sub(middle).Unit()
	if e == nil {
		support := math.Inf(1)
		for _, pole := range s.poles {
			v := p.Sub(pole).Dot(normal)
			if !finite(v) {
				return 0, ErrInvalid
			}
			support = math.Min(support, v)
		}
		distance = math.Max(distance, support)
	} else if e != ErrDegenerate {
		return 0, e
	}
	return math.Max(0, math.Nextafter(distance, math.Inf(-1))), nil
}
func projectionKnotsSupported(degree, count int, knots []float64) bool {
	a, b := knots[degree], knots[count]
	for i := 0; i <= degree; i++ {
		if knots[i] != a || knots[len(knots)-1-i] != b {
			return false
		}
	}
	run := 0
	previous := a
	for _, k := range knots {
		if k == previous {
			run++
		} else {
			run = 1
			previous = k
		}
		if k > a && k < b && run > degree {
			return false
		}
	}
	return true
}

// Project searches the full rectangular domain of a clamped continuous rational
// tensor surface using positive-weight control-hull subdivision. Trimming is
// not inferred. Work/memory exhaustion returns ErrConvergence plus a partial
// candidate. Unclamped or discontinuous knot directions are rejected.
func (s *BSplineSurface) Project(p Vec3, options SplineSurfaceProjectionOptions) (SplineSurfaceProjection, error) {
	result := SplineSurfaceProjection{Distance: math.Inf(1)}
	if s == nil || len(s.poles) == 0 || !p.IsFinite() {
		return result, ErrInvalid
	}
	o := options
	if o.DistanceTolerance == 0 {
		o.DistanceTolerance = 1e-6
	}
	if o.MaxSplits == 0 {
		o.MaxSplits = 8192
	}
	if o.MaxQueuedPoles == 0 {
		o.MaxQueuedPoles = 262144
	}
	if !finite(o.DistanceTolerance) || o.DistanceTolerance <= 0 || o.MaxSplits < 1 || o.MaxQueuedPoles < 4 {
		return result, ErrInvalid
	}
	if len(s.poles) > o.MaxQueuedPoles {
		return result, ErrConvergence
	}
	if !projectionKnotsSupported(s.degreeU, s.countU, s.knotsU) || !projectionKnotsSupported(s.degreeV, s.countV, s.knotsV) {
		return result, fmt.Errorf("%w: projection requires clamped continuous surface", ErrInvalid)
	}
	consider := func(surface *BSplineSurface) error {
		a, b, c, d := surface.Domain()
		for _, u := range []float64{a, a + (b-a)/2, b} {
			for _, v := range []float64{c, c + (d-c)/2, d} {
				q, e := surface.Eval(u, v)
				if e != nil {
					return e
				}
				distance := p.Distance(q)
				if !finite(distance) {
					return ErrInvalid
				}
				if distance < result.Distance {
					result.Distance, result.Point, result.U, result.V = distance, q, u, v
				}
			}
		}
		return nil
	}
	if e := consider(s); e != nil {
		return result, e
	}
	bound, e := surfaceHullDistance(p, s)
	if e != nil {
		return result, e
	}
	queue := surfaceProjectionQueue{{s, bound, true, 0}}
	heap.Init(&queue)
	queuedPoles := len(s.poles)
	order := 0
	for len(queue) > 0 {
		result.LowerBound = math.Min(result.Distance, queue[0].bound)
		result.ErrorBound = math.Max(0, result.Distance-result.LowerBound)
		if result.ErrorBound <= o.DistanceTolerance {
			return result, nil
		}
		if result.Splits >= o.MaxSplits {
			return result, ErrConvergence
		}
		node := heap.Pop(&queue).(surfaceProjectionNode)
		queuedPoles -= len(node.surface.poles)
		a, b, c, d := node.surface.Domain()
		var left, right *BSplineSurface
		if node.splitU {
			mid := a + (b-a)/2
			if mid == a || mid == b {
				return result, ErrConvergence
			}
			left, right, e = node.surface.SplitU(mid)
		} else {
			mid := c + (d-c)/2
			if mid == c || mid == d {
				return result, ErrConvergence
			}
			left, right, e = node.surface.SplitV(mid)
		}
		if e != nil {
			return result, e
		}
		result.Splits++
		for _, child := range []*BSplineSurface{left, right} {
			if e := consider(child); e != nil {
				return result, e
			}
			bound, e := surfaceHullDistance(p, child)
			if e != nil {
				return result, e
			}
			if bound <= result.Distance {
				if len(child.poles) > o.MaxQueuedPoles-queuedPoles {
					return result, ErrConvergence
				}
				queuedPoles += len(child.poles)
				order++
				heap.Push(&queue, surfaceProjectionNode{child, bound, !node.splitU, order})
			}
		}
	}
	result.LowerBound = result.Distance
	result.ErrorBound = 0
	return result, nil
}
