package trimmed

import (
	"context"
	"math/big"
	"plainsolid/brep/edgecheck"
	"plainsolid/brep/trimvalidate"
	"plainsolid/geom"
)

// WrappedFilletedFrustumBoreHistory retains the independently rebuilt fillet
// history. Existing entities retain their IDs; the appended bore has no old
// source face. StockFaces maps output faces to the intermediate filleted stock.
type WrappedFilletedFrustumBoreHistory struct {
	Fillets    WrappedFrustumDoubleFilletHistory
	BoreFace   int
	FloorFace  int // -1 for through bore
	FloorZ     *big.Rat
	BoreRims   [2][]int
	StockFaces [][]int
}

// FilletAndBoreWrappedFrustum rolls both original rims and cuts one axial
// circular through bore strictly inside both flat caps. The convex meridian
// radius is concave in height, so its minimum is at an endpoint: the two exact
// disk clearance inequalities prove clearance from every original cone and
// toroidal blend point. No arbitrary caller-supplied fillet history is trusted.
// All original intermediate stock IDs and supports are retained. Unsupported
// representation, touching clearance and exhausted proof limits fail atomically.
func FilletAndBoreWrappedFrustum(ctx context.Context, source Solid, lower, upper float64, center geom.Vec2, radius float64, o geom.SurfacePatchOptions) (Solid, WrappedFilletedFrustumBoreHistory, error) {
	return filletAndBoreWrappedFrustum(ctx, source, lower, upper, center, radius, 0, false, o)
}

func filletAndBoreWrappedFrustum(ctx context.Context, source Solid, lower, upper float64, center geom.Vec2, radius, depth float64, fromUpper bool, o geom.SurfacePatchOptions) (Solid, WrappedFilletedFrustumBoreHistory, error) {
	fail := func(e error) (Solid, WrappedFilletedFrustumBoreHistory, error) {
		return Solid{}, WrappedFilletedFrustumBoreHistory{}, e
	}
	if ctx == nil {
		return fail(geom.ErrInvalid)
	}
	r, cx, cy := new(big.Rat).SetFloat64(radius), new(big.Rat).SetFloat64(center.X), new(big.Rat).SetFloat64(center.Y)
	if r == nil || r.Sign() <= 0 || cx == nil || cy == nil {
		return fail(geom.ErrInvalid)
	}
	_, p, e := source.ValidateCanonicalWrappedFrustum(ctx, o)
	if e != nil {
		return fail(e)
	}
	stock, fh, e := FilletWrappedFrustumRims(ctx, source, lower, upper, o)
	if e != nil {
		return fail(e)
	}
	dx, dy := new(big.Rat).Sub(cx, p.CenterX), new(big.Rat).Sub(cy, p.CenterY)
	ds := new(big.Rat).Add(new(big.Rat).Mul(dx, dx), new(big.Rat).Mul(dy, dy))
	for _, circle := range fh.Circles {
		gap := new(big.Rat).Sub(circle.Center[0], r)
		if gap.Sign() <= 0 || new(big.Rat).Mul(gap, gap).Cmp(ds) <= 0 {
			return fail(geom.ErrDomain)
		}
	}
	vals := []*big.Rat{p.CenterX, p.CenterY, p.LowerZ, new(big.Rat).Sub(p.UpperZ, p.LowerZ), p.LowerRadius}
	f := make([]float64, len(vals))
	for i, v := range vals {
		f[i], e = geom.RepresentedExactScalar(v)
		if e != nil {
			return fail(e)
		}
	}
	// A cylindrical donor supplies only the inward bore and cap-hole pcurves.
	// Its outer radius is irrelevant to the result, but both donor and source
	// flat cap frames must agree exactly before those pcurves may be reused.
	outer, e := geom.NewCanonicalWrappedCylinder(ctx, geom.Vec3{X: f[0], Y: f[1], Z: f[2]}, f[4], f[3], o)
	if e != nil {
		return fail(e)
	}
	innerZ, innerHeight := f[2], f[3]
	open := [2]bool{true, true}
	floorEnd := 0
	var floorZ *big.Rat
	if depth != 0 {
		d := new(big.Rat).SetFloat64(depth)
		if d == nil || d.Sign() <= 0 {
			return fail(geom.ErrInvalid)
		}
		floorZ = new(big.Rat).Add(p.LowerZ, d)
		open = [2]bool{true, false}
		floorEnd = 1
		if fromUpper {
			floorZ.Sub(p.UpperZ, d)
			open = [2]bool{false, true}
			floorEnd = 0
		}
		if floorZ.Cmp(fh.Circles[0].Tangency[1]) <= 0 || floorZ.Cmp(fh.Circles[1].Tangency[1]) >= 0 {
			return fail(geom.ErrDomain)
		}
		innerHeight = depth
		if fromUpper {
			innerZ, e = geom.RepresentedExactScalar(floorZ)
			if e != nil {
				return fail(e)
			}
		}
	}
	inner, e := geom.NewCanonicalWrappedCylinder(ctx, geom.Vec3{X: center.X, Y: center.Y, Z: innerZ}, radius, innerHeight, o)
	if e != nil {
		return fail(e)
	}
	donor, dh, e := NewWrappedEccentricCircularFrustumBore(ctx, outer, inner, source.Tolerance(), o)
	if e != nil {
		return fail(e)
	}
	vertices, edges, faces := stock.Vertices(), stock.Edges(), stock.Faces()
	dv, de, df := donor.Vertices(), donor.Edges(), donor.Faces()
	wall := df[dh.InnerFaces[0]]
	for end, cap := range fh.Caps {
		if !open[end] {
			continue
		}
		a, ok := faces[cap].Surface.(geom.PlaneSurface)
		if !ok {
			return fail(geom.ErrConvergence)
		}
		b, ok := df[dh.OuterFaces[end+1]].Surface.(geom.PlaneSurface)
		if !ok || a.Frame() != b.Frame() || faces[cap].Reversed != df[dh.OuterFaces[end+1]].Reversed || len(df[dh.OuterFaces[end+1]].Holes) != 1 {
			return fail(geom.ErrConvergence)
		}
	}
	usedE, usedV := make([]bool, len(de)), make([]bool, len(dv))
	for _, u := range wall.Outer {
		usedE[u.Edge] = true
		usedV[de[u.Edge].Start] = true
		usedV[de[u.Edge].End] = true
	}
	vm, em := make([]int, len(dv)), make([]int, len(de))
	for i := range em {
		em[i] = -1
	}
	for i, yes := range usedV {
		if yes {
			vm[i] = len(vertices)
			vertices = append(vertices, dv[i])
		}
	}
	for i, yes := range usedE {
		if yes {
			x := de[i]
			x.Start = vm[x.Start]
			x.End = vm[x.End]
			em[i] = len(edges)
			edges = append(edges, x)
		}
	}
	remap := func(loop []Use) ([]Use, error) {
		out := append([]Use(nil), loop...)
		for i := range out {
			if out[i].Edge < 0 || out[i].Edge >= len(em) || em[out[i].Edge] < 0 {
				return nil, geom.ErrConvergence
			}
			out[i].Edge = em[out[i].Edge]
		}
		return out, nil
	}
	h := WrappedFilletedFrustumBoreHistory{FloorFace: -1, FloorZ: floorZ, Fillets: fh, BoreFace: len(faces), StockFaces: make([][]int, len(faces)+1)}
	for i := range faces {
		h.StockFaces[i] = []int{i}
	}
	for end, cap := range fh.Caps {
		if !open[end] {
			continue
		}
		hole, e := remap(df[dh.OuterFaces[end+1]].Holes[0])
		if e != nil {
			return fail(e)
		}
		faces[cap].Holes = append(faces[cap].Holes, hole)
		for _, u := range hole {
			h.BoreRims[end] = append(h.BoreRims[end], u.Edge)
		}
		proof, e := ValidateFaceTrimLoops(ctx, faces[cap], trimvalidate.Options{})
		if e != nil {
			return fail(e)
		}
		if proof.Status != trimvalidate.Certified {
			return fail(geom.ErrConvergence)
		}
	}
	wall.Outer, e = remap(wall.Outer)
	if e != nil {
		return fail(e)
	}
	faces = append(faces, wall)
	if floorZ != nil {
		floor := df[dh.InnerFaces[floorEnd+1]]
		floor.Outer, e = remap(floor.Outer)
		if e != nil {
			return fail(e)
		}
		h.FloorFace = len(faces)
		for _, u := range floor.Outer {
			h.BoreRims[floorEnd] = append(h.BoreRims[floorEnd], u.Edge)
		}
		faces = append(faces, floor)
		h.StockFaces = append(h.StockFaces, nil)
	}
	expectedEuler := 0
	if floorZ != nil {
		expectedEuler = 2
	}
	out, e := NewSolidContext(ctx, vertices, edges, faces, source.Tolerance())
	if e != nil {
		return fail(e)
	}
	if out.Topology().EulerCharacteristic != expectedEuler || len(out.Topology().Shells) != 1 {
		return fail(geom.ErrDegenerate)
	}
	agreement, e := out.ValidateEdgeAgreement(ctx, edgecheck.Options{MaxWork: o.MaxOperations, MaxBits: o.MaxCoefficientBits})
	if e != nil {
		return fail(e)
	}
	if !agreement.Complete {
		return fail(geom.ErrConvergence)
	}
	for _, u := range agreement.Uses {
		if u.Report.SquaredUpperBound == nil || u.Report.SquaredUpperBound.Sign() != 0 {
			return fail(geom.ErrConvergence)
		}
	}
	return out, h, nil
}
