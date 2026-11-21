package trimmed

import (
	"plainsolid/geom"
)

func reverseUnionLoop(loop []Use) []Use {
	out := make([]Use, len(loop))
	for i, u := range loop {
		u.Reversed = !u.Reversed
		u.UVStart, u.UVEnd = u.UVEnd, u.UVStart
		arc := u.Curve.(unionArcUV)
		arc.From, arc.To = arc.To, arc.From
		arc.Start, arc.End = arc.End, arc.Start
		u.Curve = arc
		out[len(loop)-1-i] = u
	}
	return out
}

// unionArcUV keeps analytic ellipse support and derivative while sharing one
// floating endpoint representative at circle-circle intersections. Construction
// proves both representatives agree with the analytic rim within model tolerance.
// Interior evaluation is never displaced or replaced by a chord.
type unionArcUV struct {
	EllipseUV
	Start, End geom.Vec2
}

func (u unionArcUV) EvalUV(t float64) (geom.Vec2, error) {
	if t == 0 {
		return u.Start, nil
	}
	if t == 1 {
		return u.End, nil
	}
	return u.EllipseUV.EvalUV(t)
}
