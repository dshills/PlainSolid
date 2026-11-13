package trimmed

import (
	"context"
	"math"

	"plainsolid/geom"
	"plainsolid/intersect"
)

// TubeTrimChordOptions bounds interval certification and support construction.
// Each bounded stage has its own budget; no partial certificate is returned.
type TubeTrimChordOptions struct {
	Interval geom.SurfaceTubeIntervalOptions
	Patch    geom.SurfacePatchOptions
}

// BoundTubeTrimChord certifies the maximum distance of an exact implicit
// intersection trim to a represented model chord. The interval is normalized
// to the TubeTrim's own traversal in [0,1]; either paired chart can be used.
// The complete interval enclosure and positive-weight support hull are proved
// before exact box-to-chord distance is computed. This is a conservative
// boundary-only bound; it does not prove face-interior error or embedding.
// Rounded chord endpoints are included in the bound. Unresolved interval
// refinement returns an error even if a coarser enclosure was available.
func BoundTubeTrimChord(ctx context.Context, trim intersect.TubeTrim, startParameter, endParameter float64, start, end geom.Vec3, o TubeTrimChordOptions) (SplineTrimChordBound, error) {
	empty := SplineTrimChordBound{}
	if ctx == nil || !start.IsFinite() || !end.IsFinite() {
		return empty, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return empty, e
	}
	uv, proved, e := trim.EncloseUV(ctx, startParameter, endParameter, o.Interval)
	if e != nil {
		return empty, e
	}
	if !proved {
		return empty, geom.ErrConvergence
	}
	patch, e := geom.NewSurfacePatch(trim.Support(), o.Patch)
	if e != nil {
		return empty, e
	}
	d := patch.Domain()
	// The certified branch lies on this support. Intersect outward-rounding
	// overshoot with its domain before restricting the exact support patch.
	u0, u1 := math.Max(d.UMin, uv.Min.X), math.Min(d.UMax, uv.Max.X)
	v0, v1 := math.Max(d.VMin, uv.Min.Y), math.Min(d.VMax, uv.Max.Y)
	widen := func(lo, hi, min, max float64) (float64, float64) {
		if lo == hi {
			lo = math.Max(min, math.Nextafter(lo, math.Inf(-1)))
			hi = math.Min(max, math.Nextafter(hi, math.Inf(1)))
		}
		return lo, hi
	}
	u0, u1 = widen(u0, u1, d.UMin, d.UMax)
	v0, v1 = widen(v0, v1, d.VMin, d.VMax)
	patch, e = patch.Trim(geom.ParameterRectangle{UMin: u0, UMax: u1, VMin: v0, VMax: v1})
	if e != nil {
		return empty, e
	}
	box, e := patch.Bounds()
	if e != nil {
		return empty, e
	}
	if e := ctx.Err(); e != nil {
		return empty, e
	}
	return SplineTrimChordBound{Bounds: box, SquaredDistance: exactChordBoxDistance(box, start, end)}, nil
}
