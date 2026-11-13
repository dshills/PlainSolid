package intersect

import (
	"context"
	"fmt"
	"math"
)

// ClosedOptions controls a local closed branch trace. The search remains in
// the supplied patch domains. ClosureTolerance is measured in model units
// and must be at least the intersection tolerance and smaller than Step/2.
// MinStep bounds adaptive retries on poorly conditioned segments.
type ClosedOptions struct {
	Options
	ClosureTolerance, MinStep float64
}

// TraceClosed traces one locally seeded transverse branch until it returns
// near the seed with a consistent tangent. It accepts closure only after
// leaving a ball around the seed, then checks a bridge back to the initial
// sample. This is a numerical closed-loop check, not global branch proof.
// On error it returns the verified prefix. Domains must be chosen so the
// loop fits without crossing a parameter seam.
func TraceClosed(ctx context.Context, a, b Patch, seed Parameters, o ClosedOptions) ([]Sample, error) {
	if o.Steps < 4 || o.Steps > 100000 {
		return nil, fmt.Errorf("intersect: invalid closed trace step limit")
	}
	if !finite(o.ClosureTolerance) || !finite(o.MinStep) || o.ClosureTolerance < o.Tolerance || o.ClosureTolerance >= o.Step/2 ||
		o.MinStep < 0 || o.MinStep > o.Step {
		return nil, fmt.Errorf("intersect: invalid closed trace tolerances")
	}
	initial := o.Options
	initial.Steps = 1
	// Reuse Trace's option, surface and seed checks, retaining its first
	// corrected point even when the initial trial step needs reduction.
	r, e := Trace(ctx, a, b, seed, initial)
	if len(r) == 0 {
		return nil, e
	}
	if o.MaxNewton == 0 {
		o.MaxNewton = 16
	}
	if o.MinStep == 0 {
		o.MinStep = math.Max(o.Step/128, 8*o.Tolerance)
	}
	first := r[0]
	out := []Sample{first}
	left := false
	step := o.Step
	for len(out) <= o.Steps {
		if e := ctx.Err(); e != nil {
			return out, e
		}
		p := out[len(out)-1]
		fr, e := evaluate(a, b, p.Parameters)
		if e != nil {
			return out, e
		}
		var next Sample
		accepted := false
		for trial := step; trial >= o.MinStep; trial *= .5 {
			pred, e := predict(fr, p.Tangent, trial)
			if e != nil {
				return out, e
			}
			target := p.Point.Add(p.Tangent.Scale(trial))
			x, f, e := correct(ctx, a, b, shift(p.Parameters, pred, 1), target, p.Tangent, o.Options)
			if e != nil {
				if ctx.Err() != nil {
					return out, ctx.Err()
				}
				continue
			}
			y := f.p.Scale(.5).Add(f.q.Scale(.5))
			t := f.t
			if t.Dot(p.Tangent) < 0 {
				t = t.Scale(-1)
			}
			d := y.Sub(p.Point)
			if d.Dot(p.Tangent) <= o.Tolerance || d.Length() > 2*trial || t.Dot(p.Tangent) < .8 {
				continue
			}
			next = Sample{x, y, t, f.p.Sub(f.q).Length()}
			step = math.Min(o.Step, trial*1.5)
			accepted = true
			break
		}
		if !accepted {
			return out, ErrUnresolved
		}
		left = left || next.Point.Sub(first.Point).Length() > 2*o.Step
		if left && len(out) >= 4 && next.Tangent.Dot(first.Tangent) > .95 {
			// Locate the closest point to the seed on the latest chord. This
			// catches passing the seed between accepted steps.
			p0, p1 := p.Point, next.Point
			chord := p1.Sub(p0)
			l := chord.Length()
			if l > 0 {
				parameter := first.Point.Sub(p0).Dot(chord.Scale(1/l)) / l
				if parameter >= 0 && parameter <= 1 && p0.Add(chord.Scale(parameter)).Sub(first.Point).Length() <= o.ClosureTolerance {
					// Verify both parameter spaces return near their initial
					// values, rather than just meeting another nearby turn.
					if p.Point.Sub(first.Point).Dot(first.Tangent) < 0 && next.Point.Sub(first.Point).Dot(first.Tangent) >= 0 {
						if p.Point.Sub(first.Point).Length() <= 2*o.Step {
							bridge := []Sample{p, first}
							if _, _, e := FitBranch(a, b, bridge, o.ClosureTolerance); e == nil {
								return append(out, first), nil
							}
						}
					}
				}
			}
		}
		out = append(out, next)
	}
	return out, ErrUnresolved
}

// ContinueClosed reports the two paired parameter curves and validated samples
// for a traced loop. fitTolerance is the allowed between-sample residual.
func ContinueClosed(ctx context.Context, a, b Patch, seed Parameters, o ClosedOptions, fitTolerance float64) (BranchCurve, BranchCurve, []Sample, error) {
	r, e := TraceClosed(ctx, a, b, seed, o)
	if e != nil {
		return BranchCurve{}, BranchCurve{}, r, e
	}
	c, d, e := FitBranch(a, b, r, fitTolerance)
	return c, d, r, e
}
