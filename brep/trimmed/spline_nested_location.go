package trimmed

import (
	"context"
	"errors"
	"math/big"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

// LocateNestedSplineMaterial classifies a represented point against a
// certified nested shell arrangement. Wrapped frusta and circular toroids
// use exact predicates. Rectangular spline and exact Bézier shells try exact
// face membership and then deterministic transverse witness rays. Other boundary
// points or inconclusive rays can remain unresolved.
func (s Solid) LocateNestedSplineMaterial(ctx context.Context, point geom.Vec3, o SplineShellBooleanOptions) (parametric.Location, error) {
	if ctx == nil || !point.IsFinite() {
		return parametric.Outside, geom.ErrInvalid
	}
	if o.MaxRays == 0 {
		o.MaxRays = 12
	}
	if o.MaxRays < 1 || o.MaxRays > 12 {
		return parametric.Outside, geom.ErrInvalid
	}
	shells, e := gatherCertifiedSplineMaterial(ctx, s, o)
	if e != nil {
		return parametric.Outside, e
	}
	containment, e := splineMaterialContainment(ctx, shells, o)
	if e == nil {
		e = assignSplineMaterialDepths(shells, containment)
	}
	if e != nil {
		return parametric.Outside, e
	}
	material := false
	for _, shell := range shells {
		if e = ctx.Err(); e != nil {
			return parametric.Outside, e
		}
		if !shell.Bounds.Contains(point) {
			continue
		}
		location, e := locateCertifiedSplineMaterialShell(ctx, shell, point, o)
		if e != nil {
			return parametric.Outside, e
		}
		if location == parametric.Boundary {
			return location, nil
		}
		if location == parametric.Inside {
			material = !material
		}
	}
	if material {
		return parametric.Inside, nil
	}
	return parametric.Outside, nil
}

func locateCertifiedSplineMaterialShell(ctx context.Context, s certifiedSplineMaterialShell, p geom.Vec3, o SplineShellBooleanOptions) (parametric.Location, error) {
	if s.Toroid != nil {
		w := s.solid.faces[0].Surface.(*geom.WrappedSurfacePatch)
		location, e := geom.LocateWrappedCircularToroid(ctx, w, p, o.Embedding.Patches)
		if errors.Is(e, geom.ErrConvergence) {
			location, e = geom.LocateWrappedToroidMeridian(ctx, w, p, o.Embedding.Patches)
		}
		switch location {
		case geom.ToroidMeridianBoundary:
			return parametric.Boundary, e
		case geom.ToroidMeridianInside:
			return parametric.Inside, e
		}
		return parametric.Outside, e
	}
	if s.Cylinder != nil || s.Frustum != nil {
		for _, f := range s.solid.faces {
			if w, ok := f.Surface.(*geom.WrappedSurfacePatch); ok {
				r, e := geom.LocateWrappedFrustum(ctx, w, p, o.Embedding.Patches)
				switch r {
				case geom.CylinderBoundary:
					return parametric.Boundary, e
				case geom.CylinderInside:
					return parametric.Inside, e
				}
				return parametric.Outside, e
			}
		}
		return parametric.Outside, geom.ErrConvergence
	}
	for _, v := range s.solid.vertices {
		if v == p {
			return parametric.Boundary, nil
		}
	}
	// A proved affine spline chart gives an exact containment check for
	// points lying on the face, including at a shared original edge.
	for _, f := range s.solid.faces {
		base, m := parametric.SurfaceMapping(f.Surface)
		patch, e := exactSplineSupportPatch(base, o.Embedding.Patches)
		if e != nil {
			return parametric.Outside, e
		}
		d, e := faceUVBounds(f)
		if e == nil {
			patch, e = patch.Trim(d)
		}
		if e == nil && m != geom.Identity() {
			patch, e = patch.Transformed(m)
		}
		if e != nil {
			return parametric.Outside, e
		}
		chart, e := geom.NewAffineSurfaceChart(ctx, patch, geom.AffineSurfaceChartOptions{
			MaxOperations: o.Embedding.Patches.MaxOperations, MaxCoefficientBits: o.Embedding.Patches.MaxCoefficientBits,
		})
		if errors.Is(e, geom.ErrConvergence) || errors.Is(e, geom.ErrDegenerate) {
			if m == geom.Identity() && p.X >= d.UMin && p.X <= d.UMax && p.Y >= d.VMin && p.Y <= d.VMax &&
				provedIdentitySplineQueryChart(ctx, base, patch, o.Embedding.Patches) {
				jet, err := patch.EvaluateRational(ctx, ratInset(p.X), ratInset(p.Y),
					geom.SurfacePatchEvaluationOptions{MaxOperations: o.Embedding.Patches.MaxOperations,
						MaxCoefficientBits: o.Embedding.Patches.MaxCoefficientBits})
				if err != nil {
					return parametric.Outside, err
				}
				if jet.Point[2].Cmp(ratInset(p.Z)) == 0 {
					return parametric.Boundary, nil
				}
			}
			continue
		}
		if e != nil {
			return parametric.Outside, e
		}
		q, e := chart.InvertExactContext(ctx, [3]*big.Rat{ratInset(p.X), ratInset(p.Y), ratInset(p.Z)},
			geom.AffineChartInverseOptions{MaxOperations: o.Embedding.Patches.MaxOperations,
				MaxCoefficientBits: o.Embedding.Patches.MaxCoefficientBits})
		if e != nil {
			return parametric.Outside, e
		}
		if q.OnPlane && q.Inside {
			return parametric.Boundary, nil
		}
	}
	inside, e := classifySeparatedSplineWitness(ctx, s.solid, s.Bounds, p, o)
	if inside {
		return parametric.Inside, e
	}
	return parametric.Outside, e
}
