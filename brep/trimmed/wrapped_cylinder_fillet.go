package trimmed

import (
	"context"
	"math/big"

	"plainsolid/brep/edgecheck"
	"plainsolid/brep/parametric"
	"plainsolid/brep/trimvalidate"
	"plainsolid/geom"
)

// WrappedCylinderRimFilletHistory identifies the retained lateral and cap
// faces and the toroidal blend faces. A zero-radius end has blend ID -1.
type WrappedCylinderRimFilletHistory struct {
	Lateral int
	Caps    [2]int
	Blends  [2]int
	// BlendRims indexes end, cap/side, then quadrant. At a sharp end
	// the cap row is -1 and the side row identifies the unchanged rim.
	BlendRims [2][2][4]int
}

// FilletWrappedCylinderRims rounds either or both circular rims of a
// canonical circular cylinder with exact rational quarter-toroid patches.
// The blend is a true rolling-circle surface. Radii must be nonnegative,
// smaller than half the cylinder radius, and have sum below its height.
// The original U chart must use canonical unit quadrants and agree exactly
// with the toroid rim parametrization. All seams and quadrant edges are shared.
func FilletWrappedCylinderRims(ctx context.Context, s Solid, lower, upper float64, o geom.SurfacePatchOptions) (Solid, WrappedCylinderRimFilletHistory, error) {
	return filletWrappedCylinderRims(ctx, s, lower, upper, false, o)
}

func filletWrappedCylinderRims(ctx context.Context, s Solid, lower, upper float64, boreTool bool, o geom.SurfacePatchOptions) (Solid, WrappedCylinderRimFilletHistory, error) {
	h := WrappedCylinderRimFilletHistory{Blends: [2]int{-1, -1}}
	if ctx == nil || !finite(lower) || !finite(upper) || lower < 0 || upper < 0 || lower == 0 && upper == 0 {
		return Solid{}, h, geom.ErrInvalid
	}
	w, p, e := s.ValidateCanonicalWrappedCylinder(ctx, o)
	if e != nil {
		return Solid{}, h, e
	}
	if !p.Circular {
		return Solid{}, h, geom.ErrConvergence
	}
	R := p.Radius
	z := [2]*big.Rat{p.LowerZ, p.UpperZ}
	r := [2]*big.Rat{ratInset(lower), ratInset(upper)}
	for _, d := range r {
		if !boreTool && new(big.Rat).Mul(d, big.NewRat(2, 1)).Cmp(R) >= 0 {
			return Solid{}, h, geom.ErrDomain
		}
	}
	a := new(big.Rat).Add(z[0], r[0])
	b := new(big.Rat).Sub(z[1], r[1])
	if a.Cmp(b) >= 0 {
		return Solid{}, h, geom.ErrDomain
	}
	w, e = geom.ReheightWrappedCylinder(ctx, w, a, b, o)
	if e != nil {
		return Solid{}, h, e
	}
	core, _, e := NewWrappedCylinder(ctx, w, s.tolerance, o)
	if e != nil {
		return Solid{}, h, e
	}
	v, edges := core.Vertices(), core.Edges()
	faces := []Face{cloneFace(core.faces[0])}
	var caps [2]Face
	height := [2]*big.Rat{a, b}
	for end, d := range r {
		if e := ctx.Err(); e != nil {
			return Solid{}, h, e
		}
		for i := range 4 {
			h.BlendRims[end][1][i] = 4*end + i
			h.BlendRims[end][0][i] = -1
		}
		if d.Sign() == 0 {
			caps[end] = core.faces[end+1]
			continue
		}
		major := new(big.Rat).Sub(R, d)
		if boreTool {
			major.Add(R, d)
		}
		patch, e := wrappedCylinderRimBlendPatch(ctx, [3]*big.Rat{p.CenterX, p.CenterY, height[end]}, major, d, end, boreTool, o)
		if e != nil {
			return Solid{}, h, e
		}
		td := patch.Domain()
		blend, _, e := geom.NewWrappedSurfacePatch(ctx, patch, geom.WrappedSurfaceOptions{WrapU: true, ContinuityOrder: 1,
			MaxPoles: o.MaxPoles, MaxOperations: o.MaxOperations, MaxCoefficientBits: o.MaxCoefficientBits})
		if e != nil {
			return Solid{}, h, e
		}
		// The toroidal and cylinder rims use the same canonical U
		// evaluator when the cylinder has the same rational chart.
		if p.QuarterParameters != ([5]float64{0, 1, 2, 3, 4}) {
			return Solid{}, h, geom.ErrConvergence
		}
		vCap, vSide := td.VMin, td.VMax
		if (end == 1) != boreTool {
			vCap, vSide = td.VMax, td.VMin
		}
		// The inner half of a toroid has an inward chart normal. The
		// standalone flared tool needs an outward wall; grafting it into
		// the bore later reverses this face along with the cylindrical wall.
		face := Face{Surface: blend, Reversed: boreTool}
		f, e := geom.RepresentedExactScalar(z[end])
		if e != nil {
			return Solid{}, h, e
		}
		frame, e := geom.NewFrame(geom.Vec3{Z: f}, geom.Vec3{X: 1}, geom.Vec3{Z: 1})
		if e != nil {
			return Solid{}, h, e
		}
		plane, e := geom.NewPlaneSurface(frame)
		if e != nil {
			return Solid{}, h, e
		}
		caps[end] = Face{Surface: plane, Reversed: end == 0}
		rim, e := blend.FundamentalPatch().IsoCurve(1, vCap, o)
		if e != nil {
			return Solid{}, h, e
		}
		firstVertex := len(v)
		for i := range 4 {
			lo, hi := float64(i), float64(i+1)
			x, e := rim.Restrict(lo, hi, o)
			if e != nil {
				return Solid{}, h, e
			}
			c, e := x.ToBSpline()
			if e != nil {
				return Solid{}, h, e
			}
			poles := c.Poles()
			v = append(v, poles[0])
			p0, p1 := geom.Vec2{X: poles[0].X, Y: poles[0].Y}, geom.Vec2{X: poles[2].X, Y: poles[2].Y}
			for j := range poles {
				poles[j].Z = 0
			}
			uv, e := geom.NewBSpline(2, poles, c.Knots(), c.Weights())
			if e != nil {
				return Solid{}, h, e
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
			return Solid{}, WrappedCylinderRimFilletHistory{}, e
		}
	}
	h.Caps = [2]int{len(faces), len(faces) + 1}
	faces = append(faces, caps[:]...)
	out, e := NewSolidContext(ctx, v, edges, faces, s.tolerance)
	if e != nil {
		return Solid{}, WrappedCylinderRimFilletHistory{}, e
	}
	top := out.Topology()
	if len(top.Shells) != 1 || top.EulerCharacteristic != 2 {
		return Solid{}, WrappedCylinderRimFilletHistory{}, geom.ErrDegenerate
	}
	agreement, e := out.ValidateEdgeAgreement(ctx, edgecheck.Options{MaxWork: o.MaxOperations, MaxBits: o.MaxCoefficientBits})
	if e != nil {
		return Solid{}, WrappedCylinderRimFilletHistory{}, e
	}
	for _, u := range agreement.Uses {
		if u.Report.SquaredUpperBound == nil || u.Report.SquaredUpperBound.Sign() != 0 {
			return Solid{}, WrappedCylinderRimFilletHistory{}, geom.ErrConvergence
		}
	}
	if !agreement.Complete {
		return Solid{}, WrappedCylinderRimFilletHistory{}, geom.ErrConvergence
	}
	return out, h, nil
}

func wrappedCylinderRimBlendPatch(ctx context.Context, center [3]*big.Rat, major, minor *big.Rat, end int, interior bool,
	o geom.SurfacePatchOptions) (*geom.SurfacePatch, error) {
	p, err := geom.RepresentedExactPoint(center)
	if err != nil {
		return nil, err
	}
	R, err := geom.RepresentedExactScalar(major)
	if err != nil {
		return nil, err
	}
	r, err := geom.RepresentedExactScalar(minor)
	if err != nil {
		return nil, err
	}
	torus, err := geom.NewCanonicalWrappedCircularToroid(ctx, p, R, r, r, o)
	if err != nil {
		return nil, err
	}
	d := torus.Domain()
	if interior {
		d.VMin, d.VMax = float64(2-end), float64(3-end)
	} else if end == 0 {
		d.VMin = 3
	} else {
		d.VMax = 1
	}
	return torus.FundamentalPatch().Trim(d)
}
