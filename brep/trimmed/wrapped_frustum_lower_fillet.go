package trimmed

import (
	"context"
	"math/big"

	"plainsolid/brep/edgecheck"
	"plainsolid/brep/parametric"
	"plainsolid/brep/trimvalidate"
	"plainsolid/geom"
)

// WrappedFrustumLowerFilletHistory records the exact rolling-circle proof in
// radial/Z meridian coordinates. SourceFaces gives original support ancestry.
type WrappedFrustumLowerFilletHistory struct {
	WrappedCylinderRimFilletHistory
	SourceFaces                          [][]int
	MeridianCenter, ConeNormal, Tangency [2]*big.Rat
	Radius                               *big.Rat
}

// FilletWrappedFrustumLowerRim rounds the lower cap of an ascending canonical
// circular frustum with an exact rolling-ball toroidal band. The cone normal
// length must be rational; the canonical fourth-quadrant endpoint parameter and
// native circle rims must be representable; source U quadrants must be 0..4.
// The major radius must exceed the
// ball radius, and side tangency must lie strictly between original cap planes.
// In the meridian let s be cone slope and L=sqrt(1+s²). The circle center
// is (R0-radius*(L-s), z0+radius), so its signed distances to the cap and
// cone support lines are both radius. At side contact its unit normal is
// (1/L,-s/L), identical to the cone normal. Revolving this identity proves
// tangency along the WHOLE circular rim, while common U evaluation and exact
// edge agreement independently verify the shared native curve. On the selected
// fourth-quadrant arc, height increases and radial slope decreases to s;
// the resulting positive-radius meridian is concave and the body embedded.
// No trigonometric approximation, endpoint-only tangency claim or tolerance
// welding is used: the complete common circular parameterization is rechecked.
// The upper cap and cone equation are retained. Supplied work limits apply to
// each bounded construction/proof call, as for the cylinder rim operation.
func FilletWrappedFrustumLowerRim(ctx context.Context, s Solid, radius float64, o geom.SurfacePatchOptions) (Solid, WrappedFrustumLowerFilletHistory, error) {
	return filletWrappedFrustumEndRim(ctx, s, radius, false, o)
}

func filletWrappedFrustumEndRim(ctx context.Context, s Solid, radius float64, upper bool, o geom.SurfacePatchOptions) (Solid, WrappedFrustumLowerFilletHistory, error) {
	h := WrappedFrustumLowerFilletHistory{WrappedCylinderRimFilletHistory: WrappedCylinderRimFilletHistory{Blends: [2]int{-1, -1}}}
	if ctx == nil || !finite(radius) || radius <= 0 {
		return Solid{}, WrappedFrustumLowerFilletHistory{}, geom.ErrInvalid
	}
	w, p, e := s.ValidateCanonicalWrappedFrustum(ctx, o)
	if e != nil {
		return Solid{}, WrappedFrustumLowerFilletHistory{}, e
	}
	if !p.Circular {
		return Solid{}, WrappedFrustumLowerFilletHistory{}, geom.ErrConvergence
	}

	R := p.LowerRadius
	selected := 0
	if upper {
		R = p.UpperRadius
		selected = 1
	}
	z := [2]*big.Rat{p.LowerZ, p.UpperZ}
	r := [2]*big.Rat{new(big.Rat), new(big.Rat)}
	r[selected] = ratInset(radius)
	slope := new(big.Rat).Quo(new(big.Rat).Sub(p.UpperRadius, p.LowerRadius), new(big.Rat).Sub(p.UpperZ, p.LowerZ))
	if (!upper && slope.Sign() <= 0) || (upper && slope.Sign() >= 0) {
		return Solid{}, WrappedFrustumLowerFilletHistory{}, geom.ErrDomain
	}
	lengthSquared := new(big.Rat).Add(big.NewRat(1, 1), new(big.Rat).Mul(slope, slope))
	n, d := new(big.Int).Sqrt(lengthSquared.Num()), new(big.Int).Sqrt(lengthSquared.Denom())
	if new(big.Int).Mul(n, n).Cmp(lengthSquared.Num()) != 0 || new(big.Int).Mul(d, d).Cmp(lengthSquared.Denom()) != 0 {
		return Solid{}, WrappedFrustumLowerFilletHistory{}, geom.ErrUnrepresentable
	}
	length := new(big.Rat).SetFrac(n, d)
	majorRadius := new(big.Rat).Sub(R, new(big.Rat).Mul(r[selected], new(big.Rat).Sub(length, slope)))
	centerZ := new(big.Rat).Add(z[0], r[selected])
	if upper {
		majorRadius.Sub(R, new(big.Rat).Mul(r[selected], new(big.Rat).Add(length, slope)))
		centerZ.Sub(z[1], r[selected])
	}
	normalR := new(big.Rat).Inv(length)
	normalZ := new(big.Rat).Neg(new(big.Rat).Quo(slope, length))
	a := new(big.Rat).Add(centerZ, new(big.Rat).Mul(r[selected], normalZ))
	sideRadius := new(big.Rat).Add(majorRadius, new(big.Rat).Mul(r[selected], normalR))
	if majorRadius.Cmp(r[selected]) <= 0 || a.Cmp(z[0]) <= 0 || a.Cmp(z[1]) >= 0 {
		return Solid{}, WrappedFrustumLowerFilletHistory{}, geom.ErrDomain
	}
	// Canonical fourth-quadrant circle parameter has x=(2t-t²)/(2-2t+t²),
	// y=-2(1-t)/(2-2t+t²). Its exact inverse at (1/L,-s/L) is 2/(1+L+s).
	parameter := new(big.Rat).Add(big.NewRat(3, 1), new(big.Rat).Quo(big.NewRat(2, 1), new(big.Rat).Add(big.NewRat(1, 1), new(big.Rat).Add(length, slope))))
	// First-quadrant x=(1-t²)/(1+t²), y=2t/(1+t²) has t=y/(1+x).
	if upper {
		parameter.Quo(new(big.Rat).Neg(slope), new(big.Rat).Add(length, big.NewRat(1, 1)))
	}
	sideV, e := geom.RepresentedExactScalar(parameter)
	if e != nil {
		return Solid{}, WrappedFrustumLowerFilletHistory{}, e
	}
	h.MeridianCenter = [2]*big.Rat{new(big.Rat).Set(majorRadius), new(big.Rat).Set(centerZ)}
	h.ConeNormal = [2]*big.Rat{new(big.Rat).Set(normalR), new(big.Rat).Set(normalZ)}
	h.Tangency = [2]*big.Rat{new(big.Rat).Set(sideRadius), new(big.Rat).Set(a)}
	h.Radius = new(big.Rat).Set(r[selected])

	lowerHeight, upperHeight := a, z[1]
	lowerRadius, upperRadius := sideRadius, p.UpperRadius
	if upper {
		lowerHeight, upperHeight = z[0], a
		lowerRadius, upperRadius = p.LowerRadius, sideRadius
	}
	baseRadius := p.LowerRadius
	w, _, e = geom.DraftWrappedFrustumExact(ctx, w, new(big.Rat), new(big.Rat).Sub(baseRadius, p.UpperRadius), o)
	if e != nil {
		return Solid{}, WrappedFrustumLowerFilletHistory{}, e
	}
	w, e = geom.ReheightWrappedCylinder(ctx, w, lowerHeight, upperHeight, o)
	if e != nil {
		return Solid{}, WrappedFrustumLowerFilletHistory{}, e
	}
	w, _, e = geom.DraftWrappedFrustumExact(ctx, w, new(big.Rat).Sub(lowerRadius, baseRadius), new(big.Rat).Sub(upperRadius, baseRadius), o)
	if e != nil {
		return Solid{}, WrappedFrustumLowerFilletHistory{}, e
	}
	core, _, e := NewWrappedFrustum(ctx, w, s.tolerance, o)
	if e != nil {
		return Solid{}, WrappedFrustumLowerFilletHistory{}, e
	}
	v, edges := core.Vertices(), core.Edges()
	faces := []Face{cloneFace(core.faces[0])}
	var caps [2]Face
	height := [2]*big.Rat{z[0], z[1]}
	height[selected] = centerZ
	values := func(x ...*big.Rat) ([]float64, error) {
		v := make([]float64, len(x))
		for i, q := range x {
			f, e := geom.RepresentedExactScalar(q)
			if e != nil {
				return nil, e
			}
			v[i] = f
		}
		return v, nil
	}
	for end, d := range r {
		if e := ctx.Err(); e != nil {
			return Solid{}, WrappedFrustumLowerFilletHistory{}, e
		}
		for i := range 4 {
			h.BlendRims[end][1][i] = 4*end + i
			h.BlendRims[end][0][i] = -1
		}
		if d.Sign() == 0 {
			caps[end] = core.faces[end+1]
			continue
		}
		major := majorRadius
		data, e := values(p.CenterX, p.CenterY, height[end], major, d)
		if e != nil {
			return Solid{}, WrappedFrustumLowerFilletHistory{}, e
		}
		torus, e := geom.NewCanonicalWrappedCircularToroid(ctx, geom.Vec3{X: data[0], Y: data[1], Z: data[2]}, data[3], data[4], data[4], o)
		if e != nil {
			return Solid{}, WrappedFrustumLowerFilletHistory{}, e
		}
		td := torus.Domain()
		if end == 0 {
			td.VMin = 3
			td.VMax = sideV
		} else {
			td.VMin = sideV
			td.VMax = 1
		}
		q, e := torus.FundamentalPatch().Trim(td)
		if e != nil {
			return Solid{}, WrappedFrustumLowerFilletHistory{}, e
		}
		blend, _, e := geom.NewWrappedSurfacePatch(ctx, q, geom.WrappedSurfaceOptions{WrapU: true, ContinuityOrder: 1,
			MaxPoles: o.MaxPoles, MaxOperations: o.MaxOperations, MaxCoefficientBits: o.MaxCoefficientBits})
		if e != nil {
			return Solid{}, WrappedFrustumLowerFilletHistory{}, e
		}
		// The toroidal and cylinder rims use the same canonical U
		// evaluator when the cylinder has the same rational chart.
		if p.QuarterParameters != ([5]float64{0, 1, 2, 3, 4}) {
			return Solid{}, WrappedFrustumLowerFilletHistory{}, geom.ErrConvergence
		}
		vCap, vSide := td.VMin, td.VMax
		if end == 1 {
			vCap, vSide = td.VMax, td.VMin
		}
		face := Face{Surface: blend}
		f, e := geom.RepresentedExactScalar(z[end])
		if e != nil {
			return Solid{}, WrappedFrustumLowerFilletHistory{}, e
		}
		frame, e := geom.NewFrame(geom.Vec3{Z: f}, geom.Vec3{X: 1}, geom.Vec3{Z: 1})
		if e != nil {
			return Solid{}, WrappedFrustumLowerFilletHistory{}, e
		}
		plane, e := geom.NewPlaneSurface(frame)
		if e != nil {
			return Solid{}, WrappedFrustumLowerFilletHistory{}, e
		}
		caps[end] = Face{Surface: plane, Reversed: end == 0}
		rim, e := blend.FundamentalPatch().IsoCurve(1, vCap, o)
		if e != nil {
			return Solid{}, WrappedFrustumLowerFilletHistory{}, e
		}
		firstVertex := len(v)
		for i := range 4 {
			lo, hi := float64(i), float64(i+1)
			x, e := rim.Restrict(lo, hi, o)
			if e != nil {
				return Solid{}, WrappedFrustumLowerFilletHistory{}, e
			}
			c, e := x.ToBSpline()
			if e != nil {
				return Solid{}, WrappedFrustumLowerFilletHistory{}, e
			}
			poles := c.Poles()
			v = append(v, poles[0])
			p0, p1 := geom.Vec2{X: poles[0].X, Y: poles[0].Y}, geom.Vec2{X: poles[2].X, Y: poles[2].Y}
			for j := range poles {
				poles[j].Z = 0
			}
			uv, e := geom.NewBSpline(2, poles, c.Knots(), c.Weights())
			if e != nil {
				return Solid{}, WrappedFrustumLowerFilletHistory{}, e
			}
			id := len(edges)
			edges = append(edges, parametric.Edge{Start: firstVertex + i, End: firstVertex + (i+1)%4, Curve: c,
				Range: parametric.Interval{Min: lo, Max: hi}})
			h.BlendRims[end][0][i] = id
			caps[end].Outer = append(caps[end].Outer, Use{Edge: id, UVStart: p0, UVEnd: p1, Curve: SplineUV{Curve: uv, Range: UVInterval{Min: lo, Max: hi}}})
		}
		lo, hi := h.BlendRims[end][0], h.BlendRims[end][1]
		seamStart, seamEnd := firstVertex, core.edges[4*end].Start
		if vSide < vCap {
			lo, hi = hi, lo
			seamStart, seamEnd = seamEnd, seamStart
		}
		for i, id := range lo {
			face.Outer = append(face.Outer, Use{Edge: id, UVStart: geom.Vec2{X: float64(i), Y: td.VMin}, UVEnd: geom.Vec2{X: float64(i + 1), Y: td.VMin}})
		}
		seam := len(edges)
		edges = append(edges, parametric.Edge{Start: seamStart, End: seamEnd, Curve: surfaceTrimCurve{
			blend, geom.Vec2{Y: td.VMin}, geom.Vec2{Y: td.VMax}, nil,
		}, Range: parametric.Interval{Max: 1}})
		face.Outer = append(face.Outer, Use{Edge: seam, UVStart: geom.Vec2{X: 4, Y: td.VMin}, UVEnd: geom.Vec2{X: 4, Y: td.VMax}})
		for i := 3; i >= 0; i-- {
			face.Outer = append(face.Outer, Use{Edge: hi[i], Reversed: true, UVStart: geom.Vec2{X: float64(i + 1), Y: td.VMax}, UVEnd: geom.Vec2{X: float64(i), Y: td.VMax}})
		}
		face.Outer = append(face.Outer, Use{Edge: seam, Reversed: true, UVStart: geom.Vec2{Y: td.VMax}, UVEnd: geom.Vec2{Y: td.VMin}})
		h.Blends[end] = len(faces)
		faces = append(faces, face)
		cert, e := ValidateFaceTrimLoops(ctx, caps[end], trimvalidate.Options{})
		if e != nil || cert.Status != trimvalidate.Certified {
			if e == nil {
				e = geom.ErrConvergence
			}
			return Solid{}, WrappedFrustumLowerFilletHistory{}, e
		}
	}
	h.Caps = [2]int{len(faces), len(faces) + 1}
	faces = append(faces, caps[:]...)
	out, e := NewSolidContext(ctx, v, edges, faces, s.tolerance)
	if e != nil {
		return Solid{}, WrappedFrustumLowerFilletHistory{}, e
	}
	top := out.Topology()
	if len(top.Shells) != 1 || top.EulerCharacteristic != 2 {
		return Solid{}, WrappedFrustumLowerFilletHistory{}, geom.ErrDegenerate
	}
	agreement, e := out.ValidateEdgeAgreement(ctx, edgecheck.Options{MaxWork: o.MaxOperations, MaxBits: o.MaxCoefficientBits})
	if e != nil {
		return Solid{}, WrappedFrustumLowerFilletHistory{}, e
	}
	for _, u := range agreement.Uses {
		if u.Report.SquaredUpperBound == nil || u.Report.SquaredUpperBound.Sign() != 0 {
			return Solid{}, WrappedFrustumLowerFilletHistory{}, geom.ErrConvergence
		}
	}
	if !agreement.Complete {
		return Solid{}, WrappedFrustumLowerFilletHistory{}, geom.ErrConvergence
	}

	h.SourceFaces = make([][]int, len(out.faces))
	oldWall, oldCaps := -1, [2]int{-1, -1}
	for i, f := range s.faces {
		switch chart := f.Surface.(type) {
		case *geom.WrappedSurfacePatch:
			oldWall = i
		case geom.PlaneSurface:
			level := ratInset(chart.Frame().Origin().Z)
			if level.Cmp(z[0]) == 0 {
				oldCaps[0] = i
			}
			if level.Cmp(z[1]) == 0 {
				oldCaps[1] = i
			}
		}
	}
	if oldWall < 0 || oldCaps[0] < 0 || oldCaps[1] < 0 {
		return Solid{}, WrappedFrustumLowerFilletHistory{}, geom.ErrConvergence
	}
	h.SourceFaces[h.Lateral] = []int{oldWall}
	h.SourceFaces[h.Caps[0]] = []int{oldCaps[0]}
	h.SourceFaces[h.Caps[1]] = []int{oldCaps[1]}
	h.SourceFaces[h.Blends[selected]] = []int{oldWall, oldCaps[selected]}
	return out, h, nil
}
