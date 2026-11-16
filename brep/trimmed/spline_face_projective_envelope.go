package trimmed

import (
	"context"
	"math/big"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

// ProjectiveSplineEnvelopeOptions bounds each imprint and the cumulative
// calls that decide the exact original material side of its fragments.
type ProjectiveSplineEnvelopeOptions struct {
	Imprint               ProjectiveSplineImprintOptions
	MaxClassifyOperations int
}

// ProjectiveSplineEnvelope identifies the lower (Intersection) or upper
// (Union) surface over one common projected rectangle. The two original
// faces have independent chart clocks but share one exact model section.
// This is a roof, with its exterior still open to its original stock walls.
type ProjectiveSplineEnvelope struct {
	Faces           [2]SewFace
	Sources         [2][]ProjectiveSplineImprintUse
	RetainedSide    [2]int // sign of original A height minus original B height
	Witness         [2]geom.ProjectiveContactHeight
	Section         geom.ProjectiveContactSpline
	ExactOperations int
	Complete        bool
}

// SelectProjectiveSplineFaceEnvelope selects two opposing source
// fragments of the same projected outer rectangle. The complete single
// section divides each exact parameter rectangle into precisely two
// connected regions, so an original corner witness fixes the lifted sign
// throughout its region. All original corners are checked for consistency.
func SelectProjectiveSplineFaceEnvelope(ctx context.Context, a, b Face, axes [2]int, operation parametric.BooleanOp,
	o ProjectiveSplineEnvelopeOptions) (r ProjectiveSplineEnvelope, err error) {
	if ctx == nil || operation != parametric.Intersection && operation != parametric.Union || o.MaxClassifyOperations < 0 {
		return r, geom.ErrInvalid
	}
	want := [2]int{-1, 1}
	if operation == parametric.Union {
		want = [2]int{1, -1}
	}
	return selectProjectiveSplineFaceFragments(ctx, a, b, axes, want, o)
}

// A common sign picks the same projected region for both originals. Their
// effective section directions then agree until one face is turned over.
func selectProjectiveSplineFaceFragments(ctx context.Context, a, b Face, axes [2]int, want [2]int,
	o ProjectiveSplineEnvelopeOptions) (r ProjectiveSplineEnvelope, err error) {
	if o.MaxClassifyOperations < 0 {
		return r, geom.ErrInvalid
	}
	if o.MaxClassifyOperations == 0 {
		o.MaxClassifyOperations = 2_000_000
	}
	source, err := ImprintProjectiveSplineFacePair(ctx, a, b, axes, o.Imprint)
	if err != nil {
		return r, err
	}
	overlap := source.contact.Graph.Cover.Atlas.Overlap
	orient := overlap.ProjectionOrientation
	for owner, f := range [2]Face{a, b} {
		if f.Reversed {
			orient[owner] *= -1
		}
		d := source.contact.Domains[owner]
		original := [2][2]float64{{d.UMin, d.UMax}, {d.VMin, d.VMax}}
		covered := [2]geom.ExactParameterInterval{overlap.Areas[owner].U, overlap.Areas[owner].V}
		for i, bound := range covered {
			if bound.Lower.Cmp(new(big.Rat).SetFloat64(original[i][0])) != 0 ||
				bound.Upper.Cmp(new(big.Rat).SetFloat64(original[i][1])) != 0 {
				return r, geom.ErrConvergence
			}
		}
	}
	if orient[0] == 0 || orient[0] != orient[1] {
		return r, geom.ErrConvergence
	}
	var choices [2]int
	var witnesses [2]geom.ProjectiveContactHeight
	for owner, part := range source.Fragments {
		match := -1
		d := source.contact.Domains[owner]
		for i, f := range part {
			var witness geom.ProjectiveContactHeight
			for _, use := range f.Face.Outer {
				u, v := use.UVStart.X, use.UVStart.Y
				if u != d.UMin && u != d.UMax || v != d.VMin && v != d.VMax {
					continue // This vertex lies at a section terminal.
				}
				var corner [2]int64
				if u == d.UMax {
					corner[0] = 1
				}
				if v == d.VMax {
					corner[1] = 1
				}
				if owner == 1 {
					if overlap.SwapB {
						corner[0], corner[1] = corner[1], corner[0]
					}
					for ax, reversed := range [2]bool{overlap.ReverseBU, overlap.ReverseBV} {
						if reversed {
							corner[ax] = 1 - corner[ax]
						}
					}
				}
				left := o.MaxClassifyOperations - r.ExactOperations
				if left < 1 {
					return r, geom.ErrConvergence
				}
				q, err := source.contact.Graph.ClassifyCommonHeightRational(ctx,
					[2]*big.Rat{big.NewRat(corner[0], 1), big.NewRat(corner[1], 1)},
					geom.SurfacePatchEvaluationOptions{MaxOperations: left, MaxCoefficientBits: o.Imprint.Contacts.Patches.MaxCoefficientBits})
				r.ExactOperations += q.ExactOperations
				if err != nil {
					return r, err
				}
				if q.Sign == 0 || witness.Difference != nil && witness.Sign != q.Sign {
					return r, geom.ErrConvergence
				}
				witness = q
			}
			if witness.Difference == nil {
				return r, geom.ErrConvergence
			}
			if witness.Sign == want[owner] {
				if match >= 0 {
					return r, geom.ErrConvergence
				}
				match, witnesses[owner] = i, witness
			}
		}
		if match < 0 {
			return r, geom.ErrConvergence
		}
		choices[owner] = match
	}
	var retained [2]SewFace
	var sources [2][]ProjectiveSplineImprintUse
	var orientation [2]bool
	for i, j := range choices {
		retained[i], sources[i] = source.Fragments[i][j], source.Sources[i][j]
		section := retained[i].Face.Outer[len(retained[i].Face.Outer)-1]
		if !sources[i][len(sources[i])-1].Section || retained[i].Edges[section.Edge].Curve != source.Section.Model {
			return r, geom.ErrConvergence
		}
		orientation[i] = section.Reversed != retained[i].Face.Reversed
	}
	if (orientation[0] == orientation[1]) != (want[0] == want[1]) {
		return r, geom.ErrConvergence
	}
	if err := ctx.Err(); err != nil {
		return r, err
	}
	r.Faces, r.Sources, r.RetainedSide, r.Witness, r.Section, r.Complete = retained, sources, want, witnesses, source.Section, true
	return r, nil
}
