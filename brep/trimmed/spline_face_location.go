package trimmed

import (
	"context"
	"errors"
	"math/big"

	"plainsolid/brep/parametric"
	"plainsolid/brep/trimvalidate"
	"plainsolid/geom"
)

// SplineFacePointLocation distinguishes a model point outside the surface
// face, on the interior of the face, or exactly on one of its trim wires.
// Unknown means no conclusion was proved.
type SplineFacePointLocation uint8

const (
	SplineFacePointUnknown SplineFacePointLocation = iota
	SplineFacePointOutside
	SplineFacePointInterior
	SplineFacePointOnTrim
)

// SplineFacePointOptions bounds patch extraction, injectivity and trim
// topology work. UV supplies optional exact original-chart proposals.
type SplineFacePointOptions struct {
	Patch       geom.SurfacePatchOptions
	Injectivity geom.SurfaceInjectivityOptions
	Trim        trimvalidate.Options
	UV          [][2]*big.Rat
	// Triangular selects an optional stronger inverse for charts with
	// one affine projected parameter coordinate.
	Triangular *geom.TriangularSurfaceInverseOptions
	// MaxTrimSubdivisions bounds trim exclusion for an algebraic UV box.
	// Zero selects 1024.
	MaxTrimSubdivisions int
}

// SplineFacePointResult retains the exact chart coordinates when a projected
// preimage has been proved, even when the model height or trim excludes it.
type SplineFacePointResult struct {
	Location SplineFacePointLocation
	UV       [2]*big.Rat
	Trim     trimvalidate.Report
	Inverse  *geom.RationalSurfaceInverse
	Triangle *geom.TriangularSurfaceInverse
}

// LocateOnSplineFace proves model point membership on a clamped spline
// face. Affine charts are inverted exactly. Other charts require a proved
// projection-injective chart and a matching exact rational UV candidate.
// The input may have arbitrary supported trim topology and affine placement.
func (s Solid) LocateOnSplineFace(ctx context.Context, face int, point geom.Vec3, o SplineFacePointOptions) (SplineFacePointResult, error) {
	if !point.IsFinite() {
		return SplineFacePointResult{}, geom.ErrInvalid
	}
	return s.LocateRationalOnSplineFace(ctx, face, [3]*big.Rat{ratInset(point.X), ratInset(point.Y), ratInset(point.Z)}, o)
}

// LocateRationalOnSplineFace accepts an exact model point, including a
// point whose coordinates cannot be represented as float64. The point
// and candidate rationals remain caller-owned.
func (s Solid) LocateRationalOnSplineFace(ctx context.Context, face int, query [3]*big.Rat, o SplineFacePointOptions) (SplineFacePointResult, error) {
	var r SplineFacePointResult
	if ctx == nil || face < 0 || face >= len(s.faces) || len(o.UV) > 256 || query[0] == nil || query[1] == nil || query[2] == nil {
		return r, geom.ErrInvalid
	}
	if o.MaxTrimSubdivisions == 0 {
		o.MaxTrimSubdivisions = 1024
	}
	if o.MaxTrimSubdivisions < 1 || o.MaxTrimSubdivisions > 1<<20 {
		return r, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return r, e
	}
	f := s.faces[face]
	base, placement := parametric.SurfaceMapping(f.Surface)
	bs, ok := base.(*geom.BSplineSurface)
	if !ok || bs == nil {
		return r, geom.ErrConvergence
	}
	patch, e := geom.NewSurfacePatch(bs, o.Patch)
	if e != nil {
		return r, e
	}
	d, e := faceUVBounds(f)
	if e == nil {
		full := patch.Domain()
		// The control hull of an active trim may conservatively extend
		// beyond the spline's valid model domain. Intersect the enclosure
		// with the full chart before inversion.
		d.UMin, d.UMax = max(d.UMin, full.UMin), min(d.UMax, full.UMax)
		d.VMin, d.VMax = max(d.VMin, full.VMin), min(d.VMax, full.VMax)
		patch, e = patch.Trim(d)
	}
	if e == nil && placement != geom.Identity() {
		patch, e = patch.Transformed(placement)
	}
	if e != nil {
		return r, e
	}
	box, e := patch.Bounds()
	if e != nil {
		return r, e
	}
	for i, q := range query {
		lo, hi := [3]float64{box.Min.X, box.Min.Y, box.Min.Z}, [3]float64{box.Max.X, box.Max.Y, box.Max.Z}
		if q.Cmp(ratInset(lo[i])) < 0 || q.Cmp(ratInset(hi[i])) > 0 {
			r.Location = SplineFacePointOutside
			return r, nil
		}
	}
	chart, e := geom.NewAffineSurfaceChart(ctx, patch, geom.AffineSurfaceChartOptions{
		MaxOperations: o.Patch.MaxOperations, MaxCoefficientBits: o.Patch.MaxCoefficientBits,
	})
	if e == nil {
		q, e := chart.InvertExactContext(ctx, query, geom.AffineChartInverseOptions{
			MaxOperations: o.Patch.MaxOperations, MaxCoefficientBits: o.Patch.MaxCoefficientBits,
		})
		if e != nil {
			return r, e
		}
		if !q.OnPlane || !q.Inside {
			r.Location = SplineFacePointOutside
			return r, nil
		}
		r.UV = [2]*big.Rat{q.U, q.V}
	} else {
		if !errors.Is(e, geom.ErrConvergence) && !errors.Is(e, geom.ErrDegenerate) {
			return r, e
		}
		if o.Triangular != nil {
			inverse, e := geom.InvertTriangularSurfacePointRational(ctx, patch, query, *o.Triangular)
			r.Triangle = &inverse
			if e == nil && inverse.Excluded {
				r.Location = SplineFacePointOutside
				return r, nil
			}
			if e == nil && inverse.Found && inverse.ModelPointValidated {
				if inverse.Parameters[0] != nil && inverse.Parameters[1] != nil {
					r.UV = inverse.Parameters
				} else {
					return classifyAlgebraicFacePoint(ctx, f, inverse, o, r)
				}
			} else if e != nil && !errors.Is(e, geom.ErrConvergence) {
				return r, e
			}
		}
		if r.UV[0] == nil {
			if e := locateCandidateFacePoint(ctx, patch, bs, placement, query, o, &r); e != nil || r.Location == SplineFacePointOutside {
				return r, e
			}
		}
	}
	loc, proof, e := LocateRationalInFaceTrim(ctx, f, r.UV, o.Trim)
	r.Trim = proof
	if e != nil {
		return r, e
	}
	switch loc {
	case trimvalidate.Outside:
		r.Location = SplineFacePointOutside
	case trimvalidate.Inside:
		r.Location = SplineFacePointInterior
	case trimvalidate.Boundary:
		r.Location = SplineFacePointOnTrim
	default:
		return r, geom.ErrConvergence
	}
	return r, nil
}

func classifyAlgebraicFacePoint(ctx context.Context, f Face, inverse geom.TriangularSurfaceInverse,
	o SplineFacePointOptions, r SplineFacePointResult) (SplineFacePointResult, error) {
	proof, e := ValidateFaceTrimLoops(ctx, f, o.Trim)
	r.Trim = proof
	if e != nil || proof.Status != trimvalidate.Certified {
		if e == nil {
			e = geom.ErrConvergence
		}
		return r, e
	}
	// One inverse coordinate is exact by the triangular projection
	// proof. A straight trim with that fixed coordinate contains the
	// root when the full remaining isolating interval lies on its span.
	for _, loop := range append([][]Use{f.Outer}, f.Holes...) {
		for _, u := range loop {
			if u.Curve != nil {
				continue
			}
			a, b := [2]float64{u.UVStart.X, u.UVStart.Y}, [2]float64{u.UVEnd.X, u.UVEnd.Y}
			for axis, x := range inverse.Parameters {
				other := axis ^ 1
				if x != nil && a[axis] == b[axis] && x.Cmp(ratInset(a[axis])) == 0 &&
					inverse.Lower[other].Cmp(ratInset(min(a[other], b[other]))) >= 0 &&
					inverse.Upper[other].Cmp(ratInset(max(a[other], b[other]))) <= 0 {
					r.Location = SplineFacePointOnTrim
					return r, nil
				}
			}
		}
	}
	var lo, hi [2]float64
	for i := range lo {
		l, e := geom.RationalOutwardRange(inverse.Lower[i])
		if e != nil {
			return r, e
		}
		h, e := geom.RationalOutwardRange(inverse.Upper[i])
		if e != nil {
			return r, e
		}
		lo[i], hi[i] = l.Min, h.Max
	}
	box := geom.UVBox{Min: geom.Vec2{X: lo[0], Y: lo[1]}, Max: geom.Vec2{X: hi[0], Y: hi[1]}}
	location, known, e := classifyTrimBox(ctx, f, box, o.MaxTrimSubdivisions)
	if e != nil || !known {
		if e == nil {
			e = geom.ErrConvergence
		}
		return r, e
	}
	if location == parametric.Inside {
		r.Location = SplineFacePointInterior
	} else {
		r.Location = SplineFacePointOutside
	}
	return r, nil
}

func locateCandidateFacePoint(ctx context.Context, patch *geom.SurfacePatch, bs *geom.BSplineSurface, placement geom.Affine,
	query [3]*big.Rat, o SplineFacePointOptions, r *SplineFacePointResult) error {
	axes := o.Injectivity.ProjectionAxes
	if axes == ([2]int{}) {
		axes = [2]int{0, 1}
		o.Injectivity.ProjectionAxes = axes
	}
	if axes[0] < 0 || axes[0] > 2 || axes[1] < 0 || axes[1] > 2 || axes[0] == axes[1] {
		return geom.ErrInvalid
	}
	candidates := append([][2]*big.Rat(nil), o.UV...)
	if placement == geom.Identity() && axes == ([2]int{0, 1}) && requireIdentitySplineChart(ctx, bs, o.Patch) == nil {
		candidates = append(candidates, [2]*big.Rat{query[0], query[1]})
	}
	inverse, e := geom.InvertSurfaceProjectionRational(ctx, patch, [2]*big.Rat{query[axes[0]], query[axes[1]]},
		geom.RationalSurfaceInverseOptions{ProjectionAxes: axes, Candidates: candidates,
			MaxDepth: o.Injectivity.MaxDepth, MaxCells: o.Injectivity.MaxCells,
			MaxOperations: o.Injectivity.MaxOperations, MaxCoefficientBits: o.Injectivity.MaxCoefficientBits})
	r.Inverse = &inverse
	if e != nil {
		return e
	}
	if inverse.Excluded {
		r.Location = SplineFacePointOutside
		return nil
	}
	if !inverse.Found {
		return geom.ErrConvergence
	}
	r.UV = inverse.Parameters
	if !exactPointCoordinatesEqual(inverse.Point, query) {
		r.Location = SplineFacePointOutside
		return nil
	}
	return nil
}
