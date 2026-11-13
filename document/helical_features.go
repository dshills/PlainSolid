package document

import (
	"context"

	"plainsolid/brep/trimmed"
	"plainsolid/csg/thread"
	"plainsolid/geom"
)

// HelicalGrooveRecipe persists the defining sketch and raw frame inputs.
// BoreRadius zero selects a shaft; a positive bore radius selects a nut.
// Start and End are physical turns in the frame, Lead is distance per turn.
type HelicalGrooveRecipe struct {
	Origin, Radial, Axis            geom.Vec3
	OuterRadius, BoreRadius, Height float64
	Lead, Start, End                float64
	Hand                            geom.HelixHandedness
	Starts                          int
	Profile                         []geom.Vec2
}

func (r HelicalGrooveRecipe) clone() HelicalGrooveRecipe {
	r.Profile = append([]geom.Vec2(nil), r.Profile...)
	return r
}

func (r HelicalGrooveRecipe) native() (trimmed.HelicalGrooveRecipe, error) {
	frame, e := geom.NewFrame(r.Origin, r.Radial, r.Axis)
	if e != nil {
		return trimmed.HelicalGrooveRecipe{}, e
	}
	return trimmed.HelicalGrooveRecipe{
		Frame: frame, OuterRadius: r.OuterRadius, BoreRadius: r.BoreRadius, Height: r.Height,
		Lead: r.Lead, Start: r.Start, End: r.End, Hand: r.Hand, Starts: r.Starts, Profile: r.Profile,
	}, nil
}

// HelicalPairPartRecipe retains both recipes to reprove nominal flank
// clearance whenever the chosen part is regenerated. Set SelectNut to use the
// nut result. Both nodes may refer to independently authored copies of this
// recipe; this feature does not synchronize edits between IDs.
type HelicalPairPartRecipe struct {
	Shaft, Nut HelicalGrooveRecipe
	SelectNut  bool
	Reliefs    thread.PairReliefs
}

func evaluateHelicalFeature(ctx context.Context, f Feature) (trimmed.Solid, error) {
	if f.Kind == HelicalGrooveFeature {
		if f.HelicalGroove == nil || f.HelicalPair != nil {
			return trimmed.Solid{}, geom.ErrInvalid
		}
		r, e := f.HelicalGroove.native()
		if e != nil {
			return trimmed.Solid{}, e
		}
		g, e := trimmed.BuildHelicalGrooveMaterial(ctx, r)
		return g.Solid(), e
	}
	r := f.HelicalPair
	if r == nil || f.HelicalGroove != nil {
		return trimmed.Solid{}, geom.ErrInvalid
	}
	a, e := r.Shaft.native()
	if e != nil {
		return trimmed.Solid{}, e
	}
	b, e := r.Nut.native()
	if e != nil {
		return trimmed.Solid{}, e
	}
	recipe := thread.PairRecipe{Shaft: a, Nut: b}
	if r.Reliefs != (thread.PairReliefs{}) {
		p, e := thread.BuildRelievedPair(ctx, recipe, r.Reliefs, geom.SurfacePatchEvaluationOptions{})
		if e != nil {
			return trimmed.Solid{}, e
		}
		if r.SelectNut {
			return p.Nut, nil
		}
		return p.Shaft, nil
	}
	p, e := thread.BuildPair(ctx, recipe, geom.SurfacePatchEvaluationOptions{})
	if e != nil {
		return trimmed.Solid{}, e
	}
	if r.SelectNut {
		return p.Nut.Solid(), nil
	}
	return p.Shaft.Solid(), nil
}
