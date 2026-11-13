package trimmed

import (
	"context"
	"math/big"
	"plainsolid/geom"
	"plainsolid/mesh"
)

// ExactBezierGraphFaceOptions bounds independent chart, composition and chord
// proof phases. MaxTriangles defaults to100000 and is capped at1000000;
// brep.MaxHoledFaceVertices additionally bounds the triangulation proposal.
// Chart evaluation limits apply per boundary vertex (at most the display
// segment count plus three); composition and display each have separate budgets.
type ExactBezierGraphFaceOptions struct {
	Chart        geom.AffineSurfaceChartOptions
	Composition  geom.SurfaceCurveAgreementOptions
	Display      geom.ComposedCollectionPolylineOptions
	MaxTriangles int
}

// CertifiedExactBezierGraphFace is the closed region below Source and above
// Baseline, with straight vertical end edges, mapped by an exact affine chart.
// CurveVertices maps each original certified polyline vertex to its one mesh
// index. The CCW boundary uses those indices in reverse curve order. ExactUV
// and ExactPoints retain unrounded geometry. FillSquaredDistance bounds BOTH
// directed distances from the source filled face to the displayed triangles.
// It includes display rounding and need not equal Display.Deflection squared.
// This standalone face certificate makes no cross-face embedding claim.
type CertifiedExactBezierGraphFace struct {
	Source              ExactBezierUV
	Baseline            *big.Rat
	Mesh                mesh.Mesh
	ExactUV             [][2]*big.Rat
	ExactPoints         [][3]*big.Rat
	Boundary            []int
	CurveVertices       []int
	Curve               geom.CertifiedComposedCollectionPolyline
	PolygonUVArea       *big.Rat
	FillSquaredDistance *big.Rat
}

// TessellateExactBezierGraphFaceCertified accepts one cubic/quartic upper trim
// with positive weights, strictly increasing Euclidean U controls, and every
// V control strictly above baseline. Its complete control hull and baseline
// must lie in the affine chart domain. These exact inequalities prove a Jordan
// graph and its three straight closing sides without sampled topology checks.
// The exact chord polygon has the same graph property. Parameterwise boundary
// homotopy and winding equality away from its error neighborhood give two-sided
// filled-region distance; exact triangle orientation/incidence/area checks
// independently prove coverage of the chord polygon. Failure is atomic.
func TessellateExactBezierGraphFaceCertified(ctx context.Context, p *geom.SurfacePatch, trim ExactBezierUV, baseline *big.Rat, o ExactBezierGraphFaceOptions) (CertifiedExactBezierGraphFace, error) {
	var empty CertifiedExactBezierGraphFace
	if ctx == nil || p == nil || baseline == nil {
		return empty, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return empty, e
	}
	if baseline.Num().BitLen() > 4096 || baseline.Denom().BitLen() > 4096 {
		return empty, geom.ErrConvergence
	}
	if o.MaxTriangles == 0 {
		o.MaxTriangles = 100000
	}
	if o.MaxTriangles < 1 || o.MaxTriangles > 1000000 {
		return empty, geom.ErrInvalid
	}
	chart, e := geom.NewAffineSurfaceChart(ctx, p, o.Chart)
	if e != nil {
		return empty, e
	}
	d := chart.Domain()
	h := trim.HomogeneousControls()
	if len(h) < 4 || len(h) > 5 {
		return empty, geom.ErrInvalid
	}
	within := func(x *big.Rat, lo, hi float64) bool { return x.Cmp(ratInset(lo)) >= 0 && x.Cmp(ratInset(hi)) <= 0 }
	if !within(baseline, d.VMin, d.VMax) {
		return empty, geom.ErrInvalid
	}
	var previous *big.Rat
	for _, c := range h {
		u, v := new(big.Rat).Quo(c[0], c[2]), new(big.Rat).Quo(c[1], c[2])
		if !within(u, d.UMin, d.UMax) || !within(v, d.VMin, d.VMax) || v.Cmp(baseline) <= 0 || previous != nil && u.Cmp(previous) <= 0 {
			return empty, geom.ErrInvalid
		}
		previous = u
	}
	cells, e := trim.ModelCurveCells(ctx, p, o.Composition)
	if e != nil {
		return empty, e
	}
	if !cells.Complete {
		return empty, geom.ErrConvergence
	}
	line, e := cells.TessellateCertified(ctx, o.Display)
	if e != nil {
		return empty, e
	}
	if !line.Complete || line.DisplayDegenerate {
		return empty, geom.ErrConvergence
	}
	boundary := CertifiedQuadraticTrimLoop{}
	rounding := new(big.Rat)
	appendPoint := func(uv [2]*big.Rat, display *geom.Vec3) error {
		point, e := chart.EvalExact(uv[0], uv[1])
		if e != nil {
			return e
		}
		v := geom.Vec3{}
		coords := []*float64{&v.X, &v.Y, &v.Z}
		for i := range coords {
			*coords[i], _ = point[i].Float64()
		}
		if display != nil {
			v = *display
		}
		if !v.IsFinite() {
			return geom.ErrConvergence
		}
		sq := new(big.Rat)
		for i, f := range []float64{v.X, v.Y, v.Z} {
			z := new(big.Rat).Sub(point[i], ratInset(f))
			sq.Add(sq, new(big.Rat).Mul(z, z))
		}
		if sq.Cmp(rounding) > 0 {
			rounding.Set(sq)
		}
		boundary.ExactUV = append(boundary.ExactUV, uv)
		boundary.ExactPoints = append(boundary.ExactPoints, point)
		boundary.Points = append(boundary.Points, v)
		return nil
	}
	ends := [2][2]*big.Rat{}
	for i, t := range []*big.Rat{new(big.Rat), big.NewRat(1, 1)} {
		uv, _, e := trim.EvaluateRational(t)
		if e != nil {
			return empty, e
		}
		ends[i] = uv
	}
	for _, uv := range [][2]*big.Rat{{ends[0][0], new(big.Rat).Set(baseline)}, {ends[1][0], new(big.Rat).Set(baseline)}} {
		if e = appendPoint(uv, nil); e != nil {
			return empty, e
		}
	}
	mapping := make([]int, len(line.Vertices))
	var lastU *big.Rat
	for i := len(line.Vertices) - 1; i >= 0; i-- {
		v := line.Vertices[i]
		if v.Parameter.Lower.Cmp(v.Parameter.Upper) != 0 {
			return empty, geom.ErrConvergence
		}
		uv, _, e := trim.EvaluateRational(v.Parameter.Lower)
		if e != nil {
			return empty, e
		}
		if lastU != nil && uv[0].Cmp(lastU) >= 0 {
			return empty, geom.ErrConvergence
		}
		lastU = uv[0]
		mapping[i] = len(boundary.Points)
		if e = appendPoint(uv, &v.Point); e != nil {
			return empty, e
		}
	}
	area := new(big.Rat)
	for i, a := range boundary.ExactUV {
		b := boundary.ExactUV[(i+1)%len(boundary.ExactUV)]
		area.Add(area, new(big.Rat).Sub(new(big.Rat).Mul(a[0], b[1]), new(big.Rat).Mul(a[1], b[0])))
	}
	area.Quo(area, big.NewRat(2, 1))
	if area.Sign() <= 0 {
		return empty, geom.ErrConvergence
	}
	// The model chord certificate includes rounded vertices. Moving those to
	// their exact chart points adds at most sqrt(rounding); use (a+b)^2<=2a²+2b².
	errorRat := ratInset(line.MaximumError)
	exactFill := new(big.Rat).Mul(errorRat, errorRat)
	exactFill.Add(exactFill, rounding)
	exactFill.Mul(exactFill, big.NewRat(2, 1))
	region := AffineQuadraticTrimRegionArea{Loops: []AffineQuadraticTrimLoopArea{{Boundary: boundary}}, PolygonUVArea: area, FillSquaredDistance: exactFill}
	filled, e := triangulateCertifiedAffineTrimRegion(ctx, region, 0, o.MaxTriangles)
	if e != nil {
		return empty, e
	}
	return CertifiedExactBezierGraphFace{Source: trim, Baseline: new(big.Rat).Set(baseline), Mesh: filled.Mesh, ExactUV: filled.ExactUV, ExactPoints: filled.ExactPoints, Boundary: filled.BoundaryLoops[0], CurveVertices: mapping, Curve: line, PolygonUVArea: new(big.Rat).Set(area), FillSquaredDistance: filled.FillSquaredDistance}, nil
}
