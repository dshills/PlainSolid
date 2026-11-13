package step

import (
	"context"
	"math/big"
	"plainsolid/brep/parametric"
	"plainsolid/brep/trimmed"
	"plainsolid/geom"
	"plainsolid/intersect"
)

// SplineLensBody is a closed two-face graph lens over an interior axis-aligned
// ellipse. Independently knot-refined degree-two supports retain their original
// bases. ReadSplineLensBody requires each whole seam quarter to lie in one
// exact knot cell; ReadPartitionedSplineLensBody handles certified rational cuts.
// Its conjugate UV basis is (Radii.X,0),(Shear,Radii.Y);
// zero Shear is the original axis-aligned family. These are chart scales, not
// principal ellipse radii. Supports retain the common unit XY chart and must both be strictly
// above z=0 on that chart. Upper-Lower is strictly positive inside the ellipse
// and strictly negative outside it. FaceNames follows upper, lower order.
// Solid and source IDs are read-only; writers prove geometry again.
type SplineLensBody struct {
	Name                 string
	FaceNames            [2]string
	Upper, Lower         *geom.BSplineSurface
	Center, Radii        geom.Vec2
	Shear                float64
	LengthUnit           float64
	Solid                trimmed.Solid
	SourceBodyID         int64
	SourceFaceIDs        [2]int64
	SourceEdges          [4]int64
	sourcePieces         []LensSeamPiece
	sourceCurves         [][3]*geom.BSpline
	sourceRationalCurves [][3]RationalLensCurve
}

func certifySplineLens(m SplineLensBody, o CurvedOvalOptions) (trimmed.Solid, error) {
	if m.Upper == nil || m.Lower == nil {
		return trimmed.Solid{}, geom.ErrInvalid
	}
	for _, s := range []*geom.BSplineSurface{m.Upper, m.Lower} {
		d := s.Data()
		if d.DegreeU != 2 || d.DegreeV != 2 || len(d.Poles) > o.MaxPoles || !lensContinuousQuadraticKnots(d.KnotsU) || !lensContinuousQuadraticKnots(d.KnotsV) {
			return trimmed.Solid{}, geom.ErrConvergence
		}
		a, b, c, e := s.Domain()
		if a != 0 || b != 1 || c != 0 || e != 1 {
			return trimmed.Solid{}, geom.ErrDomain
		}
	}
	q := QuadraticOval{Center: m.Center, Radii: m.Radii, QuarterWeight: .5}
	if _, e := curvedEllipseUV(q, m.Shear); e != nil {
		return trimmed.Solid{}, e
	}
	base, e := geom.NewBSplineSurface(1, 1, 2, 2, []geom.Vec3{{}, {Y: 1}, {X: 1}, {X: 1, Y: 1}}, []float64{0, 0, 1, 1}, []float64{0, 0, 1, 1}, nil)
	if e != nil {
		return trimmed.Solid{}, e
	}
	hint := geom.GraphEllipseHint{Center: [2]*big.Rat{new(big.Rat).SetFloat64(m.Center.X), new(big.Rat).SetFloat64(m.Center.Y)}}
	for i, r := range []float64{m.Radii.X, m.Radii.Y} {
		v := new(big.Rat).SetFloat64(r)
		hint.RadiusSquared[i] = new(big.Rat).Mul(v, v)
	}
	var solid trimmed.Solid
	var h trimmed.SplineGraphBooleanHistory
	if m.Shear == 0 {
		solid, h, e = trimmed.CombineEllipticalContactSplineGraphs(context.Background(), base, m.Upper, m.Lower, parametric.Difference, 1e-9, intersect.SplinePlaneOptions{}, hint, geom.GraphPointContactOptions{MaxDepth: 8, MaxCells: o.MaxCells, MaxOperations: o.MaxOperations, MaxCoefficientBits: o.MaxCoefficientBits})
	} else {
		q, err := splineLensQuadraticHint(m)
		if err != nil {
			return trimmed.Solid{}, err
		}
		solid, h, e = trimmed.CombineQuadraticEllipseContactSplineGraphs(context.Background(), base, m.Upper, m.Lower, parametric.Difference, 1e-9, intersect.SplinePlaneOptions{}, q, geom.GraphPointContactOptions{MaxDepth: 8, MaxCells: o.MaxCells, MaxOperations: o.MaxOperations, MaxCoefficientBits: o.MaxCoefficientBits})
	}
	if e != nil {
		return trimmed.Solid{}, e
	}
	proved := h.Ellipse != nil && h.Ellipse.Proven && h.Ellipse.InsideSide == 1
	if m.Shear != 0 {
		proved = h.QuadraticEllipse != nil && h.QuadraticEllipse.Proven && h.QuadraticEllipse.InsideSide == 1
	}
	if !proved || len(solid.Faces()) != 2 || len(solid.Edges()) != 4 {
		return trimmed.Solid{}, geom.ErrConvergence
	}
	return solid, nil
}

func splineLensQuadraticHint(m SplineLensBody) (geom.GraphQuadraticEllipseHint, error) {
	rat := func(v float64) *big.Rat { return new(big.Rat).SetFloat64(v) }
	rx, ry, sh, cx, cy := rat(m.Radii.X), rat(m.Radii.Y), rat(m.Shear), rat(m.Center.X), rat(m.Center.Y)
	if rx == nil || ry == nil || sh == nil || cx == nil || cy == nil || rx.Sign() <= 0 || ry.Sign() <= 0 {
		return geom.GraphQuadraticEllipseHint{}, geom.ErrInvalid
	}
	a := new(big.Rat).Inv(new(big.Rat).Mul(rx, rx))
	slope := new(big.Rat).Quo(sh, ry)
	cross := new(big.Rat).Mul(big.NewRat(-2, 1), new(big.Rat).Mul(a, slope))
	c := new(big.Rat).Add(new(big.Rat).Mul(a, new(big.Rat).Mul(slope, slope)), new(big.Rat).Inv(new(big.Rat).Mul(ry, ry)))
	lu := new(big.Rat).Neg(new(big.Rat).Add(new(big.Rat).Mul(big.NewRat(2, 1), new(big.Rat).Mul(a, cx)), new(big.Rat).Mul(cross, cy)))
	lv := new(big.Rat).Neg(new(big.Rat).Add(new(big.Rat).Mul(big.NewRat(2, 1), new(big.Rat).Mul(c, cy)), new(big.Rat).Mul(cross, cx)))
	constant := new(big.Rat).Add(new(big.Rat).Mul(a, new(big.Rat).Mul(cx, cx)), new(big.Rat).Mul(c, new(big.Rat).Mul(cy, cy)))
	constant.Add(constant, new(big.Rat).Mul(cross, new(big.Rat).Mul(cx, cy)))
	constant.Sub(constant, big.NewRat(1, 1))
	return geom.GraphQuadraticEllipseHint{Coefficients: [6]*big.Rat{constant, lu, a, lv, c, cross}}, nil
}

// Positive-weight quadratic splines with internal multiplicity at most two
// remain continuous. Full-multiplicity breaks are not promoted to one face,
// even if selected rim samples happen to agree.
func lensContinuousQuadraticKnots(k []float64) bool {
	if len(k) < 6 {
		return false
	}
	for i := 0; i < 3; i++ {
		if k[i] != 0 || k[len(k)-1-i] != 1 {
			return false
		}
	}
	for i := 3; i < len(k)-3; {
		v := k[i]
		if v <= 0 || v >= 1 {
			return false
		}
		j := i + 1
		for j < len(k)-3 && k[j] == v {
			j++
		}
		if j-i > 2 {
			return false
		}
		i = j
	}
	return true
}
