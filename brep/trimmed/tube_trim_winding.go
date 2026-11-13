package trimmed

import (
	"context"
	"errors"
	"slices"

	"plainsolid/geom"
	"plainsolid/intersect"
)

func trimTube(c PCurve) (intersect.TubeTrim, bool, bool) {
	reversed := false
	for {
		switch x := c.(type) {
		case reverseTrim:
			reversed = !reversed
			c = x.differentiableTrim
		case intersect.TubeTrim:
			return x, reversed, true
		default:
			return intersect.TubeTrim{}, false, false
		}
	}
}

func trimCertifiedSpline(c PCurve) (intersect.CertifiedTrim, bool, bool) {
	reversed := false
	for {
		switch x := c.(type) {
		case reverseTrim:
			reversed = !reversed
			c = x.differentiableTrim
		case intersect.CertifiedTrim:
			return x, reversed, !x.Data().OnPlane
		default:
			return intersect.CertifiedTrim{}, false, false
		}
	}
}

// Each accepted box contains the exact branch interval and the chosen
// endpoint representatives. Adjacent boxes contain the same representative
// at their join, so a continuous deformation to the chord chain avoids q.
func tubeTrimChordPath(ctx context.Context, c intersect.TubeTrim, reversed bool, q geom.UVBox, limit *int) ([]geom.Vec2, bool, error) {
	return boundedTrimChordPath(ctx, c, func(ctx context.Context, a, b float64) (geom.UVBox, bool, error) {
		return c.EncloseUV(ctx, a, b, geom.SurfaceTubeIntervalOptions{MaxIterations: 2})
	}, reversed, q, limit)
}

type trimIntervalEnclosure func(context.Context, float64, float64) (geom.UVBox, bool, error)

func boundedTrimChordPath(ctx context.Context, c PCurve, enclose trimIntervalEnclosure, reversed bool, q geom.UVBox, limit *int) ([]geom.Vec2, bool, error) {
	type interval struct{ a, b float64 }
	stack := []interval{{0, 1}}
	points := map[float64]geom.Vec2{}
	at := func(t float64) (geom.Vec2, error) {
		if p, ok := points[t]; ok {
			return p, nil
		}
		p, e := c.EvalUV(t)
		if e == nil {
			points[t] = p
		}
		return p, e
	}
	unresolved := func(e error) bool {
		return errors.Is(e, geom.ErrConvergence) || errors.Is(e, intersect.ErrUnresolved)
	}
	var path []geom.Vec2
	for len(stack) > 0 {
		if e := ctx.Err(); e != nil {
			return nil, false, e
		}
		if *limit <= 0 {
			return nil, false, nil
		}
		*limit = *limit - 1
		x := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		box, valid, e := enclose(ctx, x.a, x.b)
		if e != nil && !unresolved(e) {
			return nil, false, e
		}
		if valid {
			a, e := at(x.a)
			if e != nil {
				if unresolved(e) {
					return nil, false, nil
				}
				return nil, false, e
			}
			b, e := at(x.b)
			if e != nil {
				if unresolved(e) {
					return nil, false, nil
				}
				return nil, false, e
			}
			box.Min.X, box.Min.Y = min(box.Min.X, a.X, b.X), min(box.Min.Y, a.Y, b.Y)
			box.Max.X, box.Max.Y = max(box.Max.X, a.X, b.X), max(box.Max.Y, a.Y, b.Y)
			if box.Max.X < q.Min.X || box.Min.X > q.Max.X || box.Max.Y < q.Min.Y || box.Min.Y > q.Max.Y {
				path = append(path, a)
				continue
			}
		}
		mid := x.a/2 + x.b/2
		if mid <= x.a || mid >= x.b {
			return nil, false, nil
		}
		stack = append(stack, interval{mid, x.b}, interval{x.a, mid})
	}
	p, e := at(1)
	if e != nil {
		if unresolved(e) {
			return nil, false, nil
		}
		return nil, false, e
	}
	path = append(path, p)
	if reversed {
		slices.Reverse(path)
	}
	return path, true, nil
}
