package iges

import (
	"bytes"
	"context"
	"io"
	"math/big"
	"plainsolid/brep/parametric"
	"plainsolid/brep/trimmed"
	"plainsolid/geom"
	"plainsolid/intersect"
	"strings"
)

// SplineLensSolid is a certified closed two-face rational graph lens over an
// interior affine ellipse. Ordinary APIs use (Radii.X,0),(Shear,Radii.Y);
// affine partitioned APIs also retain the first-axis Y component Tilt.
// The original unit-XY quadratic spline charts are
// retained; lower is reversed. Solid and ancestry are read-only derived data.
// Supports must be strictly above z=0; upper-lower has the certified ellipse
// as its only contact and is positive inside. No generic implicit trim inference.
type SplineLensSolid struct {
	Upper, Lower  *geom.BSplineSurface
	Center, Radii geom.Vec2
	Shear         float64
	// Tilt is the first conjugate axis Y component; only affine partitioned APIs admit nonzero values.
	Tilt                                 float64
	LengthUnit                           float64
	Solid                                trimmed.Solid
	SourceSolidID, SourceShellID         int
	SourceFaceIDs                        [2]int
	SourceEdgeListID, SourceVertexListID int
	SourceModelCurveIDs                  [4]int
	SourceKnotCells                      [2][4]geom.ParameterRectangle
	SourceParameterCurveIDs              [2][4]int
	rawCurves                            [4][3]RationalLensCurve
	partitioned                          *PartitionedRationalSplineLens
}

func lensExact(x, y float64, sign int) (float64, error) {
	a, b := new(big.Rat).SetFloat64(x), new(big.Rat).SetFloat64(y)
	if a == nil || b == nil {
		return 0, geom.ErrInvalid
	}
	if sign < 0 {
		a.Sub(a, b)
	} else {
		a.Add(a, b)
	}
	return geom.RepresentedExactScalar(a)
}
func lensUV(c, r geom.Vec2, shear float64) ([4]*geom.BSpline, error) {
	return lensAffineUV(c, r, shear, 0)
}
func lensAffineUV(c, r geom.Vec2, shear, tilt float64) ([4]*geom.BSpline, error) {
	var out [4]*geom.BSpline
	dirs := [9][2]int{{1, 0}, {1, 1}, {0, 1}, {-1, 1}, {-1, 0}, {-1, -1}, {0, -1}, {1, -1}, {1, 0}}
	p := make([]geom.Vec3, 9)
	if r.X <= 0 || r.Y <= 0 {
		return out, geom.ErrInvalid
	}
	for i, d := range dirs {
		var e error
		p[i].X, e = lensExact(c.X, r.X*float64(d[0]), 1)
		if e != nil {
			return out, e
		}
		p[i].X, e = lensExact(p[i].X, shear*float64(d[1]), 1)
		if e != nil {
			return out, e
		}
		p[i].Y, e = lensExact(c.Y, tilt*float64(d[0]), 1)
		if e != nil {
			return out, e
		}
		p[i].Y, e = lensExact(p[i].Y, r.Y*float64(d[1]), 1)
		if e != nil {
			return out, e
		}
		if p[i].X <= 0 || p[i].X >= 1 || p[i].Y <= 0 || p[i].Y >= 1 {
			return out, geom.ErrDomain
		}
	}
	for i := range out {
		var e error
		out[i], e = geom.NewBSpline(2, p[2*i:2*i+3], []float64{0, 0, 0, 1, 1, 1}, []float64{.5, .5, 1})
		if e != nil {
			return out, e
		}
	}
	return out, nil
}
func certifyIGESLens(m SplineLensSolid) (trimmed.Solid, error) {
	if _, e := lensAffineUV(m.Center, m.Radii, m.Shear, m.Tilt); e != nil {
		return trimmed.Solid{}, e
	}
	for _, s := range []*geom.BSplineSurface{m.Upper, m.Lower} {
		if s == nil {
			return trimmed.Solid{}, geom.ErrInvalid
		}
		d := s.Data()
		a, b, c, z := s.Domain()
		if d.DegreeU != 2 || d.DegreeV != 2 || len(d.Poles) > 256 || !igesLensKnots(d.KnotsU) || !igesLensKnots(d.KnotsV) || a != 0 || b != 1 || c != 0 || z != 1 {
			return trimmed.Solid{}, geom.ErrDomain
		}
	}
	base, e := unitOvalPlane()
	if e != nil {
		return trimmed.Solid{}, e
	}
	hint := geom.GraphEllipseHint{Center: [2]*big.Rat{new(big.Rat).SetFloat64(m.Center.X), new(big.Rat).SetFloat64(m.Center.Y)}}
	for i, v := range []float64{m.Radii.X, m.Radii.Y} {
		r := new(big.Rat).SetFloat64(v)
		hint.RadiusSquared[i] = new(big.Rat).Mul(r, r)
	}
	var s trimmed.Solid
	var h trimmed.SplineGraphBooleanHistory
	options := geom.GraphPointContactOptions{MaxDepth: 8, MaxCells: 256, MaxOperations: 8000000, MaxCoefficientBits: 4096}
	if m.Shear == 0 && m.Tilt == 0 {
		s, h, e = trimmed.CombineEllipticalContactSplineGraphs(context.Background(), base, m.Upper, m.Lower, parametric.Difference, 1e-9, intersect.SplinePlaneOptions{}, hint, options)
	} else {
		q, err := igesLensQuadratic(m)
		if err != nil {
			return trimmed.Solid{}, err
		}
		s, h, e = trimmed.CombineQuadraticEllipseContactSplineGraphs(context.Background(), base, m.Upper, m.Lower, parametric.Difference, 1e-9, intersect.SplinePlaneOptions{}, q, options)
	}
	if e != nil {
		return trimmed.Solid{}, e
	}
	proved := h.Ellipse != nil && h.Ellipse.Proven && h.Ellipse.InsideSide == 1
	if m.Shear != 0 || m.Tilt != 0 {
		proved = h.QuadraticEllipse != nil && h.QuadraticEllipse.Proven && h.QuadraticEllipse.InsideSide == 1
	}
	if !proved || len(s.Faces()) != 2 || len(s.Edges()) != 4 {
		return trimmed.Solid{}, geom.ErrConvergence
	}
	return s, nil
}

// WriteSplineLensSolid proves global material orientation, authors type186 and
// shared topology, and rechecks the complete serialized graph before output.
func WriteSplineLensSolid(w io.Writer, m SplineLensSolid) error {
	if w == nil || m.Tilt != 0 || len(m.rawCurves[0][0].Poles) > 0 || m.partitioned != nil {
		return geom.ErrInvalid
	}
	if _, e := certifyIGESLens(m); e != nil {
		return e
	}
	uv, e := lensUV(m.Center, m.Radii, m.Shear)
	if e != nil {
		return e
	}
	a := topologyAuthor{}
	var surfaces [2]int
	var model [4]int
	var pc [2][4]int
	var points []geom.Vec3
	for i, s := range []*geom.BSplineSurface{m.Upper, m.Lower} {
		surfaces[i] = a.entity(Entity{Surface: &Surface{Geometry: s, UStart: 0, UEnd: 1, VStart: 0, VEnd: 1}})
	}
	patch, e := geom.NewSurfacePatch(m.Upper, curvedOvalOptions())
	if e != nil {
		return e
	}
	for i, c := range uv {
		x, e := geom.NewExactSplineCurve(c, curvedOvalOptions())
		if e != nil {
			return e
		}
		modelCurve, _, e := composeIGESLens(patch, x, m.Upper.Data().CountU != 3 || m.Upper.Data().CountV != 3)
		if e != nil {
			return e
		}
		p, e := modelCurve.ExactPoint(0)
		if e != nil {
			return e
		}
		points = append(points, p)
		g, e := modelCurve.ToBSpline()
		if e != nil {
			return e
		}
		model[i] = a.entity(Entity{Spline: &Spline{Curve: g, Start: 0, End: 1}})
	}
	for f := 0; f < 2; f++ {
		for i, c := range uv {
			pc[f][i] = a.entity(Entity{Spline: &Spline{Curve: c, Start: 0, End: 1, Planar: true, Normal: geom.Vec3{Z: 1}}})
		}
	}
	values := []string{"502", "4"}
	for _, p := range points {
		values = append(values, number(p.X), number(p.Y), number(p.Z))
	}
	vid := a.add(502, 1, joinTopologyValues(values))
	ev := []int{504, 4}
	for i, id := range model {
		ev = append(ev, id, vid, i+1, vid, (i+1)%4+1)
	}
	eid := a.add(504, 1, topologyNumbers(ev...))
	var faces [2]int
	for f := 0; f < 2; f++ {
		v := []int{508, 4}
		for i, id := range pc[f] {
			v = append(v, 0, eid, i+1, 1, 1, 0, id)
		}
		loop := a.add(508, 1, topologyNumbers(v...))
		faces[f] = a.add(510, 1, topologyNumbers(510, surfaces[f], 1, 1, loop))
	}
	shell := a.add(514, 1, topologyNumbers(514, 2, faces[0], 1, faces[1], 0))
	a.add(186, 0, topologyNumbers(186, shell, 1, 0))
	b, e := a.file(m.LengthUnit)
	if e != nil {
		return e
	}
	if _, e = ReadSplineLensSolid(bytes.NewReader(b), Limits{}); e != nil {
		return e
	}
	n, e := w.Write(b)
	if e == nil && n != len(b) {
		return io.ErrShortWrite
	}
	return e
}

func joinTopologyValues(v []string) string { return strings.Join(v, ",") + ";" }
