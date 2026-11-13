package step

import (
	"context"
	"fmt"
	"math/big"
	"plainsolid/geom"
	"sort"
)

func rawLensUV(f *File, id int64, o CurvedOvalOptions) (*geom.ExactSplineCurve, error) {
	base, ok := component(f.Entities[id], "B_SPLINE_CURVE")
	if !ok {
		base, ok = component(f.Entities[id], "B_SPLINE_CURVE_WITH_KNOTS")
		if !ok || len(base.Args) < 3 {
			return nil, geom.ErrInvalid
		}
		base.Args = base.Args[1:]
	}
	if len(base.Args) < 2 {
		return nil, geom.ErrInvalid
	}
	ids, e := listRefs(base.Args[1], 3)
	if e != nil || len(ids) != 3 {
		return nil, geom.ErrInvalid
	}
	local := cloneOvalFile(f)
	for _, pid := range ids {
		p, e := ovalRecord(f, pid, "CARTESIAN_POINT", 2)
		if e != nil || p.Args[1].Kind != Aggregate || len(p.Args[1].Items) != 2 {
			return nil, geom.ErrInvalid
		}
		p.Args[1] = Value{Kind: Aggregate, Items: append(append([]Value(nil), p.Args[1].Items...), Value{Kind: Number, Text: "0."})}
		ovalSet(local, pid, p)
	}
	c, exact, e := decodeCurvedOvalBezier(local, id, false, o)
	if e != nil {
		return nil, e
	}
	if c.Degree() != 2 {
		return nil, geom.ErrInvalid
	}
	return exact, nil
}
func partitionedEllipseDescriptor(points []geom.Vec3) (QuadraticOval, float64, error) {
	if len(points) < 4 {
		return QuadraticOval{}, 0, geom.ErrInvalid
	}
	n, s := 0, 0
	for i, p := range points {
		if p.Y > points[n].Y {
			n = i
		}
		if p.Y < points[s].Y {
			s = i
		}
	}
	cy, e := geom.RepresentedExactScalar(new(big.Rat).Quo(new(big.Rat).Add(new(big.Rat).SetFloat64(points[n].Y), new(big.Rat).SetFloat64(points[s].Y)), big.NewRat(2, 1)))
	if e != nil {
		return QuadraticOval{}, 0, e
	}
	cardinal := []geom.Vec3{points[n], points[s]}
	for _, p := range points {
		if p.Y == cy {
			cardinal = append(cardinal, p)
		}
	}
	if len(cardinal) != 4 {
		return QuadraticOval{}, 0, geom.ErrInvalid
	}
	var fake [4]*geom.BSpline
	for i, p := range cardinal {
		fake[i], e = geom.NewBSpline(2, []geom.Vec3{p, p, p}, []float64{0, 0, 0, 1, 1, 1}, nil)
		if e != nil {
			return QuadraticOval{}, 0, e
		}
	}
	return curvedEllipseDescriptor(fake)
}
func lensConicParameter(point geom.Vec3, q QuadraticOval, shear float64, quarter int) (*big.Rat, error) {
	rat := func(v float64) *big.Rat { return new(big.Rat).SetFloat64(v) }
	b := new(big.Rat).Quo(new(big.Rat).Sub(rat(point.Y), rat(q.Center.Y)), rat(q.Radii.Y))
	a := new(big.Rat).Quo(new(big.Rat).Sub(new(big.Rat).Sub(rat(point.X), rat(q.Center.X)), new(big.Rat).Mul(rat(shear), b)), rat(q.Radii.X))
	if point.Z != 0 {
		return nil, geom.ErrInvalid
	}
	switch quarter {
	case 1:
		a, b = b, new(big.Rat).Neg(a)
	case 2:
		a, b = new(big.Rat).Neg(a), new(big.Rat).Neg(b)
	case 3:
		a, b = new(big.Rat).Neg(b), a
	}
	norm := new(big.Rat).Add(new(big.Rat).Mul(a, a), new(big.Rat).Mul(b, b))
	if a.Sign() < 0 || b.Sign() < 0 || norm.Cmp(big.NewRat(1, 1)) != 0 {
		return nil, geom.ErrInvalid
	}
	return new(big.Rat).Quo(b, new(big.Rat).Add(big.NewRat(1, 1), a)), nil
}
func decodePartitionedLensFace(f *File, fid int64, o CurvedOvalOptions, refined bool, raw ...bool) (CurvedOvalFace, []lensPieceGeometry, error) {
	fail := func() (CurvedOvalFace, []lensPieceGeometry, error) {
		return CurvedOvalFace{}, nil, geom.ErrInvalid
	}
	face, e := simple(f, fid, "ADVANCED_FACE", 4)
	if e != nil {
		return fail()
	}
	positive, e := boolean(face.Args[3])
	if e != nil || !positive {
		return fail()
	}
	name, e := textValue(face.Args[0])
	if e != nil {
		return fail()
	}
	sid, e := ref(face.Args[2])
	if e != nil {
		return fail()
	}
	if e = exactSplineWeightNormalization(f, sid, "RATIONAL_B_SPLINE_SURFACE"); e != nil {
		return CurvedOvalFace{}, nil, e
	}
	surface, e := DecodeBSplineSurface(f, sid, o.MaxPoles)
	if e != nil || surface.ClosedU == "T" || surface.ClosedV == "T" || surface.SelfIntersect == "T" {
		return fail()
	}
	bounds, e := listRefs(face.Args[1], 1)
	if e != nil || len(bounds) != 1 {
		return fail()
	}
	uses, e := curvedOvalFaceUsesRange(f, fid, 4, 128)
	if e != nil {
		return fail()
	}
	owned := map[int64]bool{}
	claim := func(id int64) bool {
		if id <= 0 || owned[id] {
			return false
		}
		owned[id] = true
		return true
	}
	pieces := []lensPieceGeometry{}
	points := []geom.Vec3{}
	var first, previous int64
	for index, u := range uses {
		if !u.sense || !claim(u.edge) || !claim(u.curve) || !claim(u.use) {
			return fail()
		}
		edge, e := simple(f, u.edge, "EDGE_CURVE", 5)
		if e != nil {
			return fail()
		}
		sense, e := boolean(edge.Args[4])
		if e != nil || !sense {
			return fail()
		}
		start, e := ref(edge.Args[1])
		end, ee := ref(edge.Args[2])
		if e != nil || ee != nil || start == end || !claim(start) {
			return fail()
		}
		if index == 0 {
			first = start
		} else if previous != start {
			return fail()
		}
		previous = end
		sc, e := simple(f, u.curve, "SURFACE_CURVE", 4)
		if e != nil || sc.Args[3].Kind != Enumeration || (sc.Args[3].Text != "PCURVE_S1" && sc.Args[3].Text != "CURVE_3D") {
			return fail()
		}
		mid, e := ref(sc.Args[1])
		if e != nil || !claim(mid) {
			return fail()
		}
		pcs, e := listRefs(sc.Args[2], 1)
		if e != nil || len(pcs) != 1 || !claim(pcs[0]) {
			return fail()
		}
		pc, e := simple(f, pcs[0], "PCURVE", 3)
		if e != nil || pc.Args[1].Ref != sid {
			return fail()
		}
		rid, e := ref(pc.Args[2])
		if e != nil || !claim(rid) {
			return fail()
		}
		rep, e := simple(f, rid, "DEFINITIONAL_REPRESENTATION", 3)
		if e != nil {
			return fail()
		}
		ctx, e := ref(rep.Args[2])
		if e != nil || validatePCurveContext(f, ctx) != nil {
			return fail()
		}
		ids, e := listRefs(rep.Args[1], 1)
		if e != nil || len(ids) != 1 || !claim(ids[0]) {
			return fail()
		}
		var uv *geom.ExactSplineCurve
		var uvSpline, modelSpline *geom.BSpline
		var uvRational, modelRational RationalLensCurve
		if len(raw) > 0 && raw[0] {
			uvRational, uv, e = decodeRationalLensCurve(f, ids[0], true, o)
		} else if refined {
			uvSpline, uv, e = decodeRefinedLensCurve(f, ids[0], true, o)
		} else {
			uv, e = rawLensUV(f, ids[0], o)
		}
		if e != nil {
			return CurvedOvalFace{}, nil, fmt.Errorf("source curve: %w", e)
		}
		var model *geom.ExactSplineCurve
		if len(raw) > 0 && raw[0] {
			modelRational, model, e = decodeRationalLensCurve(f, mid, false, o)
		} else if refined {
			modelSpline, model, e = decodeRefinedLensCurve(f, mid, false, o)
		} else {
			_, model, e = decodeCurvedOvalBezier(f, mid, false, o)
		}
		if e != nil {
			return CurvedOvalFace{}, nil, fmt.Errorf("source curve: %w", e)
		}
		p, e := uv.ExactPoint(0)
		if e != nil {
			return fail()
		}
		points = append(points, p)
		for k, vid := range []int64{start, end} {
			v, e := simple(f, vid, "VERTEX_POINT", 2)
			if e != nil {
				return fail()
			}
			pid, e := ref(v.Args[1])
			if e != nil {
				return fail()
			}
			point, e := coordinate(f, pid, "CARTESIAN_POINT", 3)
			if e != nil {
				return fail()
			}
			want, e := model.ExactPoint(float64(k))
			if e != nil || want != point {
				return fail()
			}
		}
		pieces = append(pieces, lensPieceGeometry{LensSeamPiece: LensSeamPiece{SourceEdgeID: u.edge}, UV: uv, Model: model, UVSpline: uvSpline, ModelSpline: modelSpline, UVRational: uvRational, ModelRational: modelRational})
	}
	if previous != first {
		return fail()
	}
	q, shear, e := partitionedEllipseDescriptor(points)
	if e != nil {
		return CurvedOvalFace{}, nil, e
	}
	if _, e = curvedEllipseUV(q, shear); e != nil {
		return CurvedOvalFace{}, nil, e
	}
	for i := range pieces {
		a, e := pieces[i].UV.ExactPoint(0)
		if e != nil {
			return fail()
		}
		b, e := pieces[i].UV.ExactPoint(1)
		if e != nil {
			return fail()
		}
		found := false
		for quarter := 0; quarter < 4; quarter++ {
			start, e := lensConicParameter(a, q, shear, quarter)
			if e != nil {
				continue
			}
			end, e := lensConicParameter(b, q, shear, quarter)
			if e != nil || start.Cmp(end) >= 0 {
				continue
			}
			pieces[i].Quarter, pieces[i].Start, pieces[i].End = quarter, start, end
			found = true
			break
		}
		if !found {
			return fail()
		}
	}
	sort.Slice(pieces, func(i, j int) bool {
		if pieces[i].Quarter != pieces[j].Quarter {
			return pieces[i].Quarter < pieces[j].Quarter
		}
		return pieces[i].Start.Cmp(pieces[j].Start) < 0
	})
	quarter := 0
	end := new(big.Rat)
	for _, piece := range pieces {
		if end.Cmp(big.NewRat(1, 1)) == 0 {
			quarter++
			end = new(big.Rat)
		}
		if piece.Quarter != quarter || piece.Start.Cmp(end) != 0 || piece.End.Cmp(big.NewRat(1, 1)) > 0 {
			return fail()
		}
		end = piece.End
	}
	if quarter != 3 || end.Cmp(big.NewRat(1, 1)) != 0 {
		return fail()
	}
	return CurvedOvalFace{Name: name, Geometry: surface.Geometry, Outer: q, ellipseShear: shear, SourceFaceID: fid}, pieces, nil
}
func compareLensPiece(a, b *geom.ExactSplineCurve, o CurvedOvalOptions) error {
	same, e := geom.CompareExactCurvesContext(context.Background(), a, b, geom.ExactCurveComparisonOptions{MaxOperations: o.MaxOperations / 16, MaxCoefficientBits: o.MaxCoefficientBits})
	if e != nil {
		return e
	}
	if !same.Complete || !same.Equal {
		return geom.ErrConvergence
	}
	return nil
}
