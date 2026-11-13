package document

import (
	"context"

	"plainsolid/brep/trimmed"
	"plainsolid/csg/thread"
	"plainsolid/geom"
)

// LinkedHelicalNutRecipe stores the nut's independent profile and optional
// disjoint end reliefs. Its lead, physical turns, start count, hand, and raw
// frame inputs come from the upstream shaft. Edit the shaft's corresponding
// fields and the nut regenerates against the same new helical chart.
type LinkedHelicalNutRecipe struct {
	Nut          HelicalGrooveRecipe
	Lower, Upper trimmed.HelicalReliefSpec
}

func evaluateLinkedHelicalNut(ctx context.Context, f, shaft Feature) (trimmed.Solid, error) {
	var empty trimmed.Solid
	if f.LinkedHelicalNut == nil || f.HelicalGroove != nil || f.HelicalPair != nil ||
		shaft.Kind != HelicalGrooveFeature || shaft.HelicalGroove == nil || shaft.HelicalGroove.BoreRadius != 0 {
		return empty, geom.ErrInvalid
	}
	source := *shaft.HelicalGroove
	linked := *f.LinkedHelicalNut
	nut := linked.Nut
	nut.Origin, nut.Radial, nut.Axis = source.Origin, source.Radial, source.Axis
	nut.Lead, nut.Start, nut.End, nut.Hand, nut.Starts = source.Lead, source.Start, source.End, source.Hand, source.Starts
	a, e := source.native()
	if e != nil {
		return empty, e
	}
	b, e := nut.native()
	if e != nil {
		return empty, e
	}
	recipe := thread.PairRecipe{Shaft: a, Nut: b}
	if linked.Lower != (trimmed.HelicalReliefSpec{}) || linked.Upper != (trimmed.HelicalReliefSpec{}) {
		p, e := thread.BuildRelievedPair(ctx, recipe, thread.PairReliefs{NutLower: linked.Lower, NutUpper: linked.Upper}, geom.SurfacePatchEvaluationOptions{})
		return p.Nut, e
	}
	p, e := thread.BuildPair(ctx, recipe, geom.SurfacePatchEvaluationOptions{})
	return p.Nut.Solid(), e
}
