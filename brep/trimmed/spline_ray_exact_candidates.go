package trimmed

import (
	"context"
	"math/big"
	"sort"

	"plainsolid/brep/parametric"
	"plainsolid/brep/trimvalidate"
	"plainsolid/geom"
)

// ExactSplineLineCandidate proposes a face and exact original UV. The
// validator independently proves whether the evaluated model point lies on
// the finite line and in the face trim.
type ExactSplineLineCandidate struct {
	Face int
	UV   [2]*big.Rat
}

// ExactSplineLinePoint groups exact model contacts with a common original
// line parameter. Faces includes only verified candidates, not all potential
// incidence. OnTrim is aligned with Faces.
type ExactSplineLinePoint struct {
	Parameter *big.Rat
	Point     [3]*big.Rat
	Faces     []int
	OnTrim    []bool
}

// ExactSplineLineCandidateReport verifies supplied candidates without
// asserting that they exhaust the line. Unresolved lists candidate indexes
// whose face chart or trim could not be certified.
type ExactSplineLineCandidateReport struct {
	Points     []ExactSplineLinePoint
	Rejected   []int
	Unresolved []int
}

// VerifySplineLineCandidates validates proposed contact UVs against an
// unnormalized represented model line. Contacts at spline knots, face
// boundaries and shared edges retain exact rational identity. A candidate
// is not reported unless its actual model point and trim are proved.
func (s Solid) VerifySplineLineCandidates(ctx context.Context, origin, direction geom.Vec3, minT, maxT float64,
	candidates []ExactSplineLineCandidate, o SplineFacePointOptions) (ExactSplineLineCandidateReport, error) {
	var report ExactSplineLineCandidateReport
	if ctx == nil || !origin.IsFinite() || !direction.IsFinite() || direction == (geom.Vec3{}) ||
		!finite(minT) || !finite(maxT) || minT > maxT || len(candidates) > 4096 {
		return report, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return report, e
	}
	d := [3]*big.Rat{ratInset(direction.X), ratInset(direction.Y), ratInset(direction.Z)}
	x := [3]*big.Rat{ratInset(origin.X), ratInset(origin.Y), ratInset(origin.Z)}
	axis := 0
	for d[axis].Sign() == 0 {
		axis++
	}
	lo, hi := ratInset(minT), ratInset(maxT)
	rows := map[string]int{}
	for i, candidate := range candidates {
		if e := ctx.Err(); e != nil {
			return report, e
		}
		if candidate.Face < 0 || candidate.Face >= len(s.faces) || candidate.UV[0] == nil || candidate.UV[1] == nil {
			return ExactSplineLineCandidateReport{}, geom.ErrInvalid
		}
		f := s.faces[candidate.Face]
		patch, e := exactFaceSupportPatch(f, o.Patch)
		if e != nil {
			report.Unresolved = append(report.Unresolved, i)
			continue
		}
		p, e := patch.EvaluateRational(ctx, candidate.UV[0], candidate.UV[1], geom.SurfacePatchEvaluationOptions{
			MaxOperations: o.Patch.MaxOperations, MaxCoefficientBits: o.Patch.MaxCoefficientBits,
		})
		if e != nil {
			report.Unresolved = append(report.Unresolved, i)
			continue
		}
		t := new(big.Rat).Quo(new(big.Rat).Sub(p.Point[axis], x[axis]), d[axis])
		exact := t.Cmp(lo) >= 0 && t.Cmp(hi) <= 0
		for j, v := range p.Point {
			exact = exact && v.Cmp(new(big.Rat).Add(x[j], new(big.Rat).Mul(t, d[j]))) == 0
		}
		if !exact {
			report.Rejected = append(report.Rejected, i)
			continue
		}
		loc, _, e := LocateRationalInFaceTrim(ctx, f, candidate.UV, o.Trim)
		if e != nil {
			return report, e
		}
		switch loc {
		case trimvalidate.Outside:
			report.Rejected = append(report.Rejected, i)
			continue
		case trimvalidate.Unknown:
			report.Unresolved = append(report.Unresolved, i)
			continue
		}
		key := t.RatString()
		n, found := rows[key]
		if !found {
			n = len(report.Points)
			rows[key] = n
			report.Points = append(report.Points, ExactSplineLinePoint{Parameter: t, Point: p.Point})
		}
		row := &report.Points[n]
		known := false
		for j, face := range row.Faces {
			if face == candidate.Face {
				row.OnTrim[j] = row.OnTrim[j] || loc == trimvalidate.Boundary
				known = true
				break
			}
		}
		if !known {
			row.Faces = append(row.Faces, candidate.Face)
			row.OnTrim = append(row.OnTrim, loc == trimvalidate.Boundary)
		}
	}
	sort.Slice(report.Points, func(i, j int) bool { return report.Points[i].Parameter.Cmp(report.Points[j].Parameter) < 0 })
	return report, nil
}

func exactFaceSupportPatch(f Face, o geom.SurfacePatchOptions) (*geom.SurfacePatch, error) {
	base, m := parametric.SurfaceMapping(f.Surface)
	p, e := exactSplineSupportPatch(base, o)
	if e == nil && m != geom.Identity() {
		p, e = p.Transformed(m)
	}
	return p, e
}

func exactSplineSupportPatch(s geom.Surface, o geom.SurfacePatchOptions) (*geom.SurfacePatch, error) {
	switch b := s.(type) {
	case *geom.BSplineSurface:
		if b != nil {
			return geom.NewSurfacePatch(b, o)
		}
	case *geom.ExactBezierSurface:
		if b != nil && b.ExactPatch() != nil {
			p := b.ExactPatch()
			u, v := p.ControlShape()
			if o.MaxPoles < 0 || o.MaxPoles > 0 && u*v > o.MaxPoles {
				return nil, geom.ErrConvergence
			}
			return p, nil
		}
	}
	return nil, geom.ErrConvergence
}

func supportsExactSplinePatch(s geom.Surface) bool {
	switch s := s.(type) {
	case *geom.BSplineSurface:
		return s != nil
	case *geom.ExactBezierSurface:
		return s != nil && s.ExactPatch() != nil
	}
	return false
}
