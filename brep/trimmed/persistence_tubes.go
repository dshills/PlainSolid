package trimmed

import (
	"context"
	"encoding/json"
	"fmt"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
	"plainsolid/intersect"
)

type nativeTubeUse struct {
	Recipe, Segment int
	Second          bool
}

type nativeTubePatch struct {
	Surface  json.RawMessage
	Min, Max geom.Vec2
}

type nativeTubeRecipe struct {
	A, B           nativeTubePatch
	Cells          []intersect.TubeRecipeCell
	FreeAxis       int
	Options        intersect.TubeCurveOptions
	Oriented       []intersect.TubeDirection     `json:",omitempty"`
	Closed         bool                          `json:",omitempty"`
	ExactEndpoints []intersect.TubeExactEndpoint `json:",omitempty"`
}

func encodeTubeRecipe(data intersect.TubeRecipeData) (nativeTubeRecipe, error) {
	encode := func(p intersect.Patch) (nativeTubePatch, error) {
		s, e := parametric.EncodeSurface(p.Surface)
		return nativeTubePatch{s, p.Min, p.Max}, e
	}
	a, e := encode(data.A)
	if e != nil {
		return nativeTubeRecipe{}, e
	}
	b, e := encode(data.B)
	if e != nil {
		return nativeTubeRecipe{}, e
	}
	return nativeTubeRecipe{A: a, B: b, Cells: data.Cells, FreeAxis: data.FreeAxis, Options: data.Options,
		Oriented: data.Oriented, Closed: data.Closed, ExactEndpoints: data.ExactEndpoints}, nil
}

// Proof arithmetic and reconstruction work are capped independently of the
// file's byte limit. In-memory chains can use larger budgets.
func validateTubeRecipes(tubes []nativeTubeRecipe, maxControls int) error {
	if len(tubes) > 32 {
		return fmt.Errorf("trimmed: tube recipe count limit")
	}
	cells := 0
	for _, t := range tubes {
		cells += len(t.Cells)
		p, v, j := t.Options.Patches, t.Options.Evaluation, t.Options.Junction
		inRange := func(n, hi int) bool { return n >= 0 && n <= hi }
		if len(t.Cells) == 0 || cells > 128 || !inRange(t.FreeAxis, 3) || len(t.ExactEndpoints) > 2 ||
			len(t.Oriented) != 0 && len(t.Oriented) != len(t.Cells) ||
			t.Closed && (len(t.Cells) < 2 || len(t.Oriented) == 0) ||
			!inRange(p.MaxPoles, min(maxControls, 4096)) || !inRange(p.MaxOperations, 1000000) || !inRange(p.MaxCoefficientBits, 16384) ||
			!inRange(v.MaxOperations, 1000000) || !inRange(v.MaxCoefficientBits, 16384) || !inRange(v.MaxIterations, 24) ||
			!inRange(j.MaxOperations, 1000000) || !inRange(j.MaxCoefficientBits, 16384) {
			return fmt.Errorf("trimmed: tube recipe exceeds work limit or has invalid inputs")
		}
		for _, direction := range t.Oriented {
			if !inRange(direction.FreeAxis, 3) {
				return fmt.Errorf("trimmed: invalid oriented tube axis")
			}
		}
	}
	return nil
}

func (t nativeTubeRecipe) rebuild(ctx context.Context, maxControls int) ([]intersect.TubeSegment, error) {
	return t.rebuildWithExactSupport(ctx, maxControls, false)
}

func (t nativeTubeRecipe) rebuildWithExactSupport(ctx context.Context, maxControls int, allowExact bool) ([]intersect.TubeSegment, error) {
	decode := func(p nativeTubePatch) (intersect.Patch, error) {
		s, e := parametric.DecodeSurface(p.Surface, maxControls)
		if e != nil {
			return intersect.Patch{}, e
		}
		_, spline := s.(*geom.BSplineSurface)
		base, _ := parametric.SurfaceMapping(s)
		if !spline && !(allowExact && supportsExactSplinePatch(base)) {
			return intersect.Patch{}, fmt.Errorf("trimmed: tube recipe requires two splines")
		}
		return intersect.Patch{Surface: s, Min: p.Min, Max: p.Max}, nil
	}
	a, e := decode(t.A)
	if e != nil {
		return nil, e
	}
	b, e := decode(t.B)
	if e != nil {
		return nil, e
	}
	return intersect.RebuildTubeCurves(ctx, intersect.TubeRecipeData{A: a, B: b, Cells: t.Cells, FreeAxis: t.FreeAxis, Options: t.Options,
		Oriented: t.Oriented, Closed: t.Closed, ExactEndpoints: t.ExactEndpoints})
}
