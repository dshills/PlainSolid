package geom

import (
	"container/heap"
	"fmt"
	"math"
)

// CurveProjectionOptions bounds global branch-and-bound projection. Zero
// fields select DistanceTolerance=1e-7 and MaxSplits=4096. The error estimate
// is the best sampled distance minus a control-hull box/support-plane lower bound.
type CurveProjectionOptions struct {
	DistanceTolerance float64
	MaxSplits         int
}

// CurveProjection records a closest-point candidate and the remaining global
// distance gap. ErrorBound is a numerical control-hull estimate, not a formally
// outward-rounded certificate through all rational knot-insertion arithmetic.
// Multiple equal minima may exist; one deterministic representative is returned.
type CurveProjection struct {
	Parameter                        float64
	Point                            Vec3
	Distance, LowerBound, ErrorBound float64
	Splits                           int
}
type projectionNode struct {
	curve *BSpline
	bound float64
	start float64
}
type projectionQueue []projectionNode

func (q projectionQueue) Len() int { return len(q) }
func (q projectionQueue) Less(i, j int) bool {
	if q[i].bound == q[j].bound {
		return q[i].start < q[j].start
	}
	return q[i].bound < q[j].bound
}
func (q projectionQueue) Swap(i, j int) { q[i], q[j] = q[j], q[i] }
func (q *projectionQueue) Push(x any)   { *q = append(*q, x.(projectionNode)) }
func (q *projectionQueue) Pop() any {
	old := *q
	last := old[len(old)-1]
	old[len(old)-1] = projectionNode{}
	*q = old[:len(old)-1]
	return last
}
func hullDistance(p Vec3, curve *BSpline) (float64, error) {
	box, e := Bounds(curve.poles)
	if e != nil {
		return 0, e
	}
	delta := Vec3{math.Max(box.Min.X-p.X, math.Max(0, p.X-box.Max.X)), math.Max(box.Min.Y-p.Y, math.Max(0, p.Y-box.Max.Y)), math.Max(box.Min.Z-p.Z, math.Max(0, p.Z-box.Max.Z))}
	distance := delta.Length()
	if !finite(distance) {
		return 0, ErrInvalid
	}
	// A support-plane bound retains control-point correlations that an axis
	// aligned box loses. Near a stationary projection it converges quadratically
	// with the span width instead of forcing thousands of box-only splits.
	a, b := curve.Domain()
	midpoint, e := curve.Eval(a + (b-a)/2)
	if e != nil {
		return 0, e
	}
	normal, e := p.Sub(midpoint).Unit()
	if e == nil {
		support := math.Inf(1)
		for _, pole := range curve.poles {
			d := p.Sub(pole).Dot(normal)
			if !finite(d) {
				return 0, ErrInvalid
			}
			support = math.Min(support, d)
		}
		distance = math.Max(distance, support)
	} else if e != ErrDegenerate {
		return 0, e
	}
	return math.Max(0, math.Nextafter(distance, math.Inf(-1))), nil
}

// Project finds a globally bounded closest-point candidate on a clamped,
// continuous rational B-spline. Positive weights make each control hull a
// geometric enclosure. Knot subdivision refines those enclosures; work or
// floating-point parameter-resolution exhaustion returns ErrConvergence along
// with the best candidate and current error estimate. Unclamped curves and
// degree+1 interior knot multiplicities are explicitly rejected.
func (c *BSpline) Project(p Vec3, options CurveProjectionOptions) (CurveProjection, error) {
	result := CurveProjection{Distance: math.Inf(1)}
	if c == nil || len(c.poles) == 0 || !p.IsFinite() {
		return result, ErrInvalid
	}
	o := options
	if o.DistanceTolerance == 0 {
		o.DistanceTolerance = 1e-7
	}
	if o.MaxSplits == 0 {
		o.MaxSplits = 4096
	}
	if !finite(o.DistanceTolerance) || o.DistanceTolerance <= 0 || o.MaxSplits < 1 {
		return result, ErrInvalid
	}
	a, b := c.Domain()
	for i := 0; i <= c.degree; i++ {
		if c.knots[i] != a || c.knots[len(c.knots)-1-i] != b {
			return result, fmt.Errorf("%w: projection requires clamped knots", ErrInvalid)
		}
	}
	run := 0
	previous := a
	for _, k := range c.knots {
		if k == previous {
			run++
		} else {
			run = 1
			previous = k
		}
		if k > a && k < b && run > c.degree {
			return result, fmt.Errorf("%w: projection requires continuous knots", ErrInvalid)
		}
	}
	consider := func(curve *BSpline, t float64) error {
		q, e := curve.Eval(t)
		if e != nil {
			return e
		}
		d := p.Distance(q)
		if !finite(d) {
			return ErrInvalid
		}
		if d < result.Distance || (d == result.Distance && t < result.Parameter) {
			result.Distance, result.Parameter, result.Point = d, t, q
		}
		return nil
	}
	for _, t := range []float64{a, a + (b-a)/2, b} {
		if e := consider(c, t); e != nil {
			return result, e
		}
	}
	bound, e := hullDistance(p, c)
	if e != nil {
		return result, e
	}
	queue := projectionQueue{{c, bound, a}}
	heap.Init(&queue)
	for len(queue) > 0 {
		result.LowerBound = math.Min(result.Distance, queue[0].bound)
		result.ErrorBound = math.Max(0, result.Distance-result.LowerBound)
		if result.ErrorBound <= o.DistanceTolerance {
			return result, nil
		}
		if result.Splits >= o.MaxSplits {
			return result, ErrConvergence
		}
		node := heap.Pop(&queue).(projectionNode)
		left, right := node.curve.Domain()
		mid := left + (right-left)/2
		if mid == left || mid == right {
			return result, ErrConvergence
		}
		lc, rc, e := node.curve.Split(mid)
		if e != nil {
			return result, e
		}
		result.Splits++
		for _, child := range []*BSpline{lc, rc} {
			u, v := child.Domain()
			for _, t := range []float64{u, u + (v-u)/2, v} {
				if e = consider(child, t); e != nil {
					return result, e
				}
			}
			lower, e := hullDistance(p, child)
			if e != nil {
				return result, e
			}
			if lower <= result.Distance {
				heap.Push(&queue, projectionNode{child, lower, u})
			}
		}
	}
	result.LowerBound = result.Distance
	result.ErrorBound = 0
	return result, nil
}
