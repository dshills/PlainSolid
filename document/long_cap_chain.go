package document

import (
	"context"

	"plainsolid/brep/trimmed"
	"plainsolid/csg/fillet"
	"plainsolid/geom"
)

// LongCapChainRecipe stores the bounded fillet, opposite chamfer, circular
// through bore, rectangular through-pocket and top boss recipe. Dimensions
// are in the authored local coordinates; use a node transform for placement.
type LongCapChainRecipe struct {
	Length, Width, Height, Radius, TopSetback, SideSetback float64
	Bore                                                   trimmed.ExtrusionCircle
	Pocket                                                 trimmed.ExtrusionRectangle
	Boss                                                   trimmed.ExtrusionBoss
}

func evaluateLongCapChain(ctx context.Context, f Feature) (trimmed.Solid, error) {
	p := f.LongCapChain
	if p == nil {
		return trimmed.Solid{}, geom.ErrInvalid
	}
	r, e := fillet.TreatLongCapEdgesWithFeatures(ctx, p.Length, p.Width, p.Height, p.Radius, p.TopSetback, p.SideSetback, p.Bore, p.Pocket, p.Boss, fillet.Options{Tolerance: f.Tolerance})
	return r.Solid, e
}
