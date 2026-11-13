package trimmed

import (
	"context"
	"fmt"
	"math/big"

	"plainsolid/brep"
	"plainsolid/brep/parametric"
	"plainsolid/geom"
	"plainsolid/intersect"
)

// SealDisjointSplinePlaneSections seals several closed contours on one
// certified XY-injective spline. Each contour's certified parameter bounding
// rectangle must be strictly separated from every other one; nested or
// ambiguous contour arrangements are rejected. The result contains
// independent shells ordered by contour traversal.
func SealDisjointSplinePlaneSections(ctx context.Context, surface *geom.BSplineSurface, height, tolerance float64, o intersect.SplinePlaneOptions, proof geom.SurfaceInjectivityOptions) (Solid, []SplineSectionHistory, error) {
	if ctx == nil {
		return Solid{}, nil, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return Solid{}, nil, e
	}
	p, orientation, e := projectedSectionProof(surface, height, tolerance, o, proof)
	if e != nil {
		return Solid{}, nil, e
	}
	frame, e := geom.FrameFromNormal(geom.Vec3{Z: height}, geom.Vec3{Z: 1})
	if e != nil {
		return Solid{}, nil, e
	}
	plane, e := geom.NewPlane(frame.Origin(), frame.ZAxis())
	if e != nil {
		return Solid{}, nil, e
	}
	sign, e := geom.ClassifyPlaneBoundarySigns(p, plane, geom.PlaneBoundarySignOptions{MaxOperations: proof.MaxOperations, MaxCoefficientBits: proof.MaxCoefficientBits})
	if e != nil {
		return Solid{}, nil, e
	}
	if !sign.Complete || sign.StrictSign == 0 {
		return Solid{}, nil, geom.ErrConvergence
	}
	if o.ResidualTolerance == 0 {
		o.ResidualTolerance = tolerance
	}
	cs, trace, e := intersect.TraceSplinePlaneContext(ctx, surface, frame, o)
	if e != nil {
		return Solid{}, nil, e
	}
	if len(cs) == 0 || len(cs) > 32 || len(trace.Isolation.Tangencies) != 0 {
		return Solid{}, nil, fmt.Errorf("trimmed: unsupported contour count")
	}
	areas := make([]geom.UVBox, 0, len(cs))
	for _, c := range cs {
		if e := ctx.Err(); e != nil {
			return Solid{}, nil, e
		}
		if !c.Closed || len(c.Segments) == 0 {
			return Solid{}, nil, fmt.Errorf("trimmed: expected only closed contours")
		}
		var box geom.UVBox
		for i, seg := range c.Segments {
			q, e := seg.SplineTrim().BoundsUV()
			if e != nil {
				return Solid{}, nil, e
			}
			if i == 0 {
				box = q
			} else {
				box.Min.X, box.Min.Y = min(box.Min.X, q.Min.X), min(box.Min.Y, q.Min.Y)
				box.Max.X, box.Max.Y = max(box.Max.X, q.Max.X), max(box.Max.Y, q.Max.Y)
			}
		}
		for _, x := range areas {
			if box.Min.X <= x.Max.X && box.Max.X >= x.Min.X && box.Min.Y <= x.Max.Y && box.Max.Y >= x.Min.Y {
				return Solid{}, nil, fmt.Errorf("trimmed: closed contour parameter bounds overlap")
			}
		}
		areas = append(areas, box)
	}
	var parts []PlacedSolid
	var history []SplineSectionHistory
	faceOffset, edgeOffset := 0, 0
	for _, c := range cs {
		if e := ctx.Err(); e != nil {
			return Solid{}, nil, e
		}
		s, h, e := assembleClosedSection(surface, frame, c, p.Domain(), sign.StrictSign, orientation, tolerance)
		if e != nil {
			return Solid{}, nil, e
		}
		h.SplineFace += faceOffset
		h.PlaneFace += faceOffset
		for i := range h.ContourEdges {
			h.ContourEdges[i] += edgeOffset
		}
		history = append(history, h)
		faceOffset += len(s.faces)
		edgeOffset += len(s.edges)
		parts = append(parts, PlacedSolid{s, geom.Identity()})
	}
	x, e := AssembleShells(ctx, parts, 64)
	if e != nil {
		return Solid{}, nil, e
	}
	return x, history, nil
}

// SplineSectionHistory identifies the two original surface charts and the
// shared contour edges in loop order.
type SplineSectionHistory struct {
	SplineFace, PlaneFace int
	ContourEdges          []int
}

func checkUnitGraph(surface *geom.BSplineSurface) (geom.BSplineSurfaceData, error) {
	if surface == nil {
		return geom.BSplineSurfaceData{}, geom.ErrInvalid
	}
	d := surface.Data()
	u, e := unitGraphAbscissas(d.DegreeU, d.CountU, d.KnotsU)
	if e != nil {
		return d, e
	}
	v, e := unitGraphAbscissas(d.DegreeV, d.CountV, d.KnotsV)
	if e != nil {
		return d, e
	}
	for i := range d.CountU {
		for j := range d.CountV {
			k := i*d.CountV + j
			p := d.Poles[k]
			if len(d.Weights) != 0 && d.Weights[k] != d.Weights[0] ||
				new(big.Rat).SetFloat64(p.X).Cmp(u[i]) != 0 ||
				new(big.Rat).SetFloat64(p.Y).Cmp(v[j]) != 0 {
				return d, fmt.Errorf("trimmed: XY must be the exact identity spline graph")
			}
		}
	}
	return d, nil
}

func unitGraphAbscissas(degree, count int, knots []float64) ([]*big.Rat, error) {
	if degree < 1 || count <= degree || len(knots) != degree+count+1 {
		return nil, geom.ErrDomain
	}
	for i := 0; i <= degree; i++ {
		if knots[i] != 0 || knots[count+i] != 1 {
			return nil, fmt.Errorf("trimmed: expected a clamped unit parameter domain")
		}
	}
	out := make([]*big.Rat, count)
	d := big.NewRat(int64(degree), 1)
	total := new(big.Rat)
	for i := 1; i <= degree; i++ {
		total.Add(total, new(big.Rat).SetFloat64(knots[i]))
	}
	for i := range out {
		out[i] = new(big.Rat).Quo(total, d)
		if i+1 < len(out) {
			total.Sub(total, new(big.Rat).SetFloat64(knots[i+1]))
			total.Add(total, new(big.Rat).SetFloat64(knots[i+degree+1]))
		}
	}
	return out, nil
}

// SealSplinePlaneSection forms the solid between a polynomial spline graph
// z=f(u,v), x=u, y=v on [0,1]² and a horizontal plane. The section must consist
// of exactly one regular closed contour strictly inside the chart. Isolated
// tangencies, additional contours and coincident regions are rejected.
// The exact graph and implicit spline–plane edges are retained.
func SealSplinePlaneSection(surface *geom.BSplineSurface, height, tolerance float64, options intersect.SplinePlaneOptions) (Solid, SplineSectionHistory, error) {
	fail := func(reason string) (Solid, SplineSectionHistory, error) {
		return Solid{}, SplineSectionHistory{}, fmt.Errorf("trimmed: spline plane seal: %s", reason)
	}
	if surface == nil || !finite(height) || !finite(tolerance) || tolerance <= 0 || options.Domain != (geom.ParameterRectangle{}) {
		return fail("invalid graph, plane, tolerance or domain override")
	}
	_, e := checkUnitGraph(surface)
	if e != nil {
		return Solid{}, SplineSectionHistory{}, e
	}
	return sealClosedSection(surface, height, 1, tolerance, options, geom.PlaneBoundarySignOptions{})
}

// SealProjectedSplinePlaneSection forms the solid between an XY-injective
// rational spline and a horizontal plane around one closed regular contour.
// It proves global XY injectivity and consistent projected orientation on
// the requested spline rectangle before assembling the two trimmed faces.
func SealProjectedSplinePlaneSection(surface *geom.BSplineSurface, height, tolerance float64, o intersect.SplinePlaneOptions, proof geom.SurfaceInjectivityOptions) (Solid, SplineSectionHistory, error) {
	_, orientation, e := projectedSectionProof(surface, height, tolerance, o, proof)
	if e != nil {
		return Solid{}, SplineSectionHistory{}, e
	}
	return sealClosedSection(surface, height, orientation, tolerance, o,
		geom.PlaneBoundarySignOptions{MaxOperations: proof.MaxOperations, MaxCoefficientBits: proof.MaxCoefficientBits})
}

func projectedSectionProof(surface *geom.BSplineSurface, height, tolerance float64, o intersect.SplinePlaneOptions, proof geom.SurfaceInjectivityOptions) (*geom.SurfacePatch, int, error) {
	if surface == nil || !finite(height) || !finite(tolerance) || tolerance <= 0 ||
		proof.ProjectionAxes != ([2]int{}) && proof.ProjectionAxes != ([2]int{0, 1}) {
		return nil, 0, geom.ErrInvalid
	}
	p, e := geom.NewSurfacePatch(surface, o.Patch)
	if e != nil {
		return nil, 0, e
	}
	if o.Domain != (geom.ParameterRectangle{}) {
		p, e = p.Trim(o.Domain)
		if e != nil {
			return nil, 0, e
		}
	}
	orientation, e := projectedPatchProof(p, proof)
	return p, orientation, e
}

func projectedPatchProof(p *geom.SurfacePatch, proof geom.SurfaceInjectivityOptions) (int, error) {
	if proof.ProjectionAxes != ([2]int{}) && proof.ProjectionAxes != ([2]int{0, 1}) {
		return 0, geom.ErrInvalid
	}
	x, e := geom.CertifySurfaceInjectivity(p, proof)
	if e != nil {
		return 0, e
	}
	if !x.ProvenInjective {
		return 0, geom.ErrConvergence
	}
	r, e := geom.CertifySurfaceRegularity(p, geom.SurfaceRegularityOptions{ReferenceDirection: geom.Vec3{Z: 1},
		MaxDepth: proof.MaxDepth, MaxCells: proof.MaxCells, MaxOperations: proof.MaxOperations, MaxCoefficientBits: proof.MaxCoefficientBits})
	if e != nil {
		return 0, e
	}
	if !r.Complete || !r.ConsistentOrientation {
		return 0, geom.ErrConvergence
	}
	return r.Orientation, nil
}

func sealClosedSection(surface *geom.BSplineSurface, height float64, orientation int, tolerance float64, options intersect.SplinePlaneOptions, boundaryOptions geom.PlaneBoundarySignOptions) (Solid, SplineSectionHistory, error) {
	fail := func(reason string) (Solid, SplineSectionHistory, error) {
		return Solid{}, SplineSectionHistory{}, fmt.Errorf("trimmed: spline plane seal: %s", reason)
	}
	frame, e := geom.FrameFromNormal(geom.Vec3{Z: height}, geom.Vec3{Z: 1})
	if e != nil {
		return Solid{}, SplineSectionHistory{}, e
	}
	if options.ResidualTolerance == 0 {
		options.ResidualTolerance = tolerance
	}
	p, e := geom.NewSurfacePatch(surface, options.Patch)
	if e != nil {
		return Solid{}, SplineSectionHistory{}, e
	}
	if options.Domain != (geom.ParameterRectangle{}) {
		p, e = p.Trim(options.Domain)
		if e != nil {
			return Solid{}, SplineSectionHistory{}, e
		}
	}
	pl, e := geom.NewPlane(frame.Origin(), frame.ZAxis())
	if e != nil {
		return Solid{}, SplineSectionHistory{}, e
	}
	outside, e := geom.ClassifyPlaneBoundarySigns(p, pl, boundaryOptions)
	if e != nil {
		return Solid{}, SplineSectionHistory{}, e
	}
	if !outside.Complete || outside.StrictSign == 0 {
		return fail("chart boundary does not have a proved strict sign")
	}
	contours, trace, e := intersect.TraceSplinePlane(surface, frame, options)
	if e != nil {
		return Solid{}, SplineSectionHistory{}, e
	}
	if len(contours) != 1 || len(trace.Isolation.Tangencies) != 0 {
		return fail("expected one bounded regular interior contour")
	}
	return assembleClosedSection(surface, frame, contours[0], p.Domain(), outside.StrictSign, orientation, tolerance)
}

func assembleClosedSection(surface *geom.BSplineSurface, frame geom.Frame, contour intersect.CertifiedContour, domain geom.ParameterRectangle, sign, orientation int, tolerance float64) (Solid, SplineSectionHistory, error) {
	fail := func(reason string) (Solid, SplineSectionHistory, error) {
		return Solid{}, SplineSectionHistory{}, fmt.Errorf("trimmed: spline plane seal: %s", reason)
	}
	if !contour.Closed || len(contour.Segments) < 3 || len(contour.Segments) > brep.MaxHoledFaceVertices {
		return fail("expected one bounded regular interior contour")
	}
	n := len(contour.Segments)
	splineTrim, planeTrim := make([]PCurve, n), make([]PCurve, n)
	var start [2][]geom.Vec2
	start[0], start[1] = make([]geom.Vec2, n), make([]geom.Vec2, n)
	lo, hi, bottom, top := domain.UMin, domain.UMax, domain.VMin, domain.VMax
	for i, a := range contour.Segments {
		splineTrim[i], planeTrim[i] = a.SplineTrim(), a.PlaneTrim()
		p, e := splineTrim[i].EvalUV(0)
		if e != nil {
			return Solid{}, SplineSectionHistory{}, e
		}
		if p.X <= lo || p.X >= hi || p.Y <= bottom || p.Y >= top {
			return fail("contour contacts the chart boundary")
		}
		start[0][i] = p
		start[1][i], e = planeTrim[i].EvalUV(0)
		if e != nil {
			return Solid{}, SplineSectionHistory{}, e
		}
	}
	// The graph proof makes the contour injective in both charts. Use its
	// chord polygon for a winding reference; NewSolid validates the polygon.
	reversed := [2]bool{}
	for k, set := range [2][]PCurve{splineTrim, planeTrim} {
		area := new(big.Rat)
		for i, p := range start[k] {
			q := start[k][(i+1)%n]
			a := new(big.Rat).Mul(new(big.Rat).SetFloat64(p.X), new(big.Rat).SetFloat64(q.Y))
			b := new(big.Rat).Mul(new(big.Rat).SetFloat64(p.Y), new(big.Rat).SetFloat64(q.X))
			area.Add(area, a.Sub(a, b))
		}
		if area.Sign() == 0 {
			return fail("unresolved contour winding")
		}
		reversed[k] = area.Sign() < 0
		if reversed[k] {
			rev := make([]PCurve, n)
			for i := range rev {
				rev[i] = reverseTrim{set[n-1-i].(differentiableTrim)}
			}
			copy(set, rev)
		}
	}
	wantReverse := contour.Segments[0].PlaneSignOnRight() != sign
	if reversed[0] != wantReverse {
		return fail("chord winding disagrees with certified contour orientation")
	}
	if (reversed[0] == reversed[1]) != (orientation > 0) {
		return fail("contour winding disagrees with projection orientation")
	}
	plane, e := geom.NewPlaneSurface(frame)
	if e != nil {
		return Solid{}, SplineSectionHistory{}, e
	}
	// A single regular interior contour changes sign; the certified
	// boundary sign is the sign of the outside region.
	below := sign > 0
	face := [2]Face{{Surface: surface, Reversed: below == (orientation > 0)}, {Surface: plane, Reversed: !below}}
	vertices := make([]geom.Vec3, n)
	edges := make([]parametric.Edge, n)
	h := SplineSectionHistory{SplineFace: 0, PlaneFace: 1, ContourEdges: make([]int, n)}
	for i := range n {
		for k, curves := range [2][]PCurve{splineTrim, planeTrim} {
			c := curves[i]
			a, e := c.EvalUV(0)
			if e != nil {
				return Solid{}, SplineSectionHistory{}, e
			}
			b, e := c.EvalUV(1)
			if e != nil {
				return Solid{}, SplineSectionHistory{}, e
			}
			id := i
			rev := k == 1 && reversed[0] != reversed[1]
			if rev {
				id = n - 1 - i
			}
			face[k].Outer = append(face[k].Outer, Use{Edge: id, Reversed: rev, UVStart: a, UVEnd: b, Curve: c})
			if k == 0 {
				m := surfaceTrimCurve{surface, a, b, c}
				vertices[i], e = m.Eval(0)
				if e != nil {
					return Solid{}, SplineSectionHistory{}, e
				}
				edges[i] = parametric.Edge{Start: i, End: (i + 1) % n, Curve: m, Range: parametric.Interval{Max: 1}}
			}
		}
		h.ContourEdges[i] = i
	}
	out, e := NewSolid(vertices, edges, face[:], tolerance)
	if e != nil {
		return Solid{}, SplineSectionHistory{}, e
	}
	return out, h, nil
}
