package step

import (
	"context"
	"io"
	"math/big"
	"plainsolid/brep/trimmed"
	"plainsolid/geom"
)

// WrappedRimFilletBody is a canonical exact circular cylinder with one or both
// rim fillets. Walls are ordered lower blend (if any), cylinder, upper blend.
type WrappedRimFilletBody struct {
	Name           string
	Center         geom.Vec3
	Radius, Height float64
	Fillets        [2]float64
	Walls          []WrappedSplineFace
	CapNames       [2]string
	Solid          trimmed.Solid
	LengthUnit     float64
}
type wrappedFilletBandEnds struct{ CenterX, CenterY, LowerZ, UpperZ, LowerRadius, UpperRadius *big.Rat }

// Endpoint data only, never an embedding certificate. Full support identity
// against the constructive rolling-circle solid is mandatory after graph proof.
func wrappedFilletEndpoints(w *geom.WrappedSurfacePatch) (wrappedFilletBandEnds, error) {
	var p wrappedFilletBandEnds
	if w == nil {
		return p, geom.ErrInvalid
	}
	d := w.Domain()
	if d.UMin != 0 || d.UMax != 4 {
		return p, geom.ErrConvergence
	}
	eval := func(u, v float64) ([3]*big.Rat, error) {
		q, e := w.EvaluateRational(context.Background(), new(big.Rat).SetFloat64(u), new(big.Rat).SetFloat64(v), geom.SurfacePatchEvaluationOptions{})
		return q.Point, e
	}
	a, e := eval(0, d.VMin)
	if e != nil {
		return p, e
	}
	b, e := eval(2, d.VMin)
	if e != nil {
		return p, e
	}
	c, e := eval(0, d.VMax)
	if e != nil {
		return p, e
	}
	p.CenterX = new(big.Rat).Quo(new(big.Rat).Add(a[0], b[0]), big.NewRat(2, 1))
	p.CenterY = new(big.Rat).Set(a[1])
	p.LowerZ = new(big.Rat).Set(a[2])
	p.UpperZ = new(big.Rat).Set(c[2])
	p.LowerRadius = new(big.Rat).Sub(a[0], p.CenterX)
	p.UpperRadius = new(big.Rat).Sub(c[0], p.CenterX)
	if p.LowerZ.Cmp(p.UpperZ) >= 0 || p.LowerRadius.Sign() <= 0 || p.UpperRadius.Sign() <= 0 {
		return p, geom.ErrConvergence
	}
	return p, nil
}
func wrappedFilletDescriptor(m WrappedStationLoftBody) (WrappedRimFilletBody, error) {
	return wrappedFilletDescriptorOptions(m, geom.SurfacePatchOptions{})
}
func wrappedFilletDescriptorOptions(m WrappedStationLoftBody, o geom.SurfacePatchOptions) (WrappedRimFilletBody, error) {
	out := WrappedRimFilletBody{Name: m.Name, LengthUnit: m.LengthUnit, Walls: m.Walls, CapNames: m.CapNames, Solid: m.Solid}
	if len(m.Walls) < 2 || len(m.Walls) > 3 || len(m.Stations) != len(m.Walls)+1 {
		return out, geom.ErrInvalid
	}
	side := -1
	var proof geom.WrappedCylinderEmbedding
	for i, w := range m.Walls {
		p, e := geom.CertifyWrappedCylinderEmbedding(context.Background(), w.Geometry, o)
		if e == nil && p.Complete && p.Circular {
			if side >= 0 {
				return out, geom.ErrConvergence
			}
			side = i
			proof = p
		}
	}
	if side < 0 || (side != 0 && side != 1) || (len(m.Walls)-1-side) > 1 {
		return out, geom.ErrConvergence
	}
	val := func(q *big.Rat) (float64, error) { return geom.RepresentedExactScalar(q) }
	var e error
	out.Center.X, e = val(proof.CenterX)
	if e != nil {
		return out, e
	}
	out.Center.Y, e = val(proof.CenterY)
	if e != nil {
		return out, e
	}
	out.Center.Z = m.Stations[0].Z
	out.Radius, e = val(proof.Radius)
	if e != nil {
		return out, e
	}
	lo, hi := new(big.Rat).SetFloat64(m.Stations[0].Z), new(big.Rat).SetFloat64(m.Stations[len(m.Stations)-1].Z)
	out.Height, e = val(new(big.Rat).Sub(hi, lo))
	if e != nil {
		return out, e
	}
	out.Fillets[0], e = val(new(big.Rat).Sub(proof.LowerZ, lo))
	if e != nil {
		return out, e
	}
	out.Fillets[1], e = val(new(big.Rat).Sub(hi, proof.UpperZ))
	return out, e
}
func filletNative(m WrappedRimFilletBody) (trimmed.Solid, trimmed.WrappedCylinderRimFilletHistory, error) {
	return filletNativeOptions(m, geom.SurfacePatchOptions{})
}
func filletNativeOptions(m WrappedRimFilletBody, o geom.SurfacePatchOptions) (trimmed.Solid, trimmed.WrappedCylinderRimFilletHistory, error) {
	w, e := geom.NewCanonicalWrappedCylinder(context.Background(), m.Center, m.Radius, m.Height, o)
	if e != nil {
		return trimmed.Solid{}, trimmed.WrappedCylinderRimFilletHistory{}, e
	}
	stock, _, e := trimmed.NewWrappedCylinder(context.Background(), w, 1e-10, o)
	if e != nil {
		return trimmed.Solid{}, trimmed.WrappedCylinderRimFilletHistory{}, e
	}
	return trimmed.FilletWrappedCylinderRims(context.Background(), stock, m.Fillets[0], m.Fillets[1], o)
}
func certifyWrappedFilletBands(m WrappedStationLoftBody, o geom.SurfacePatchOptions) (trimmed.Solid, error) {
	desc, e := wrappedFilletDescriptorOptions(m, o)
	if e != nil {
		return trimmed.Solid{}, e
	}
	solid, h, e := filletNativeOptions(desc, o)
	if e != nil {
		return solid, e
	}
	ids := []int{}
	if h.Blends[0] >= 0 {
		ids = append(ids, h.Blends[0])
	}
	ids = append(ids, h.Lateral)
	if h.Blends[1] >= 0 {
		ids = append(ids, h.Blends[1])
	}
	if len(ids) != len(m.Walls) {
		return trimmed.Solid{}, geom.ErrConvergence
	}
	faces := solid.Faces()
	for i, id := range ids {
		want, ok := faces[id].Surface.(*geom.WrappedSurfacePatch)
		if !ok {
			return trimmed.Solid{}, geom.ErrConvergence
		}
		same, e := geom.CompareExactSurfaces(context.Background(), want.FundamentalPatch(), m.Walls[i].Geometry.FundamentalPatch(), geom.ExactSurfaceComparisonOptions{MaxOperations: o.MaxOperations, MaxCoefficientBits: o.MaxCoefficientBits})
		if e != nil {
			return trimmed.Solid{}, e
		}
		if !same.Complete || !same.Equal {
			return trimmed.Solid{}, geom.ErrConvergence
		}
	}
	return solid, nil
}
func ReadWrappedRimFillet(r io.Reader, l Limits, o WrappedSurfaceOptions) (WrappedRimFilletBody, error) {
	m, e := readWrappedFilletBands(r, l, o)
	if e != nil {
		return WrappedRimFilletBody{}, e
	}
	return wrappedFilletDescriptor(m)
}

// WrappedRimFilletFromSolid recovers explicit stock/radii only after complete
// native topology validation; original wall supports are retained.
func WrappedRimFilletFromSolid(ctx context.Context, s trimmed.Solid, name string, unit float64, o geom.SurfacePatchOptions) (WrappedRimFilletBody, error) {
	var out WrappedRimFilletBody
	if !validLengthUnit(unit) {
		return out, geom.ErrInvalid
	}
	p, e := s.ValidateWrappedCylinderRimFillet(ctx, o)
	if e != nil {
		return out, e
	}
	if !p.Complete {
		return out, geom.ErrConvergence
	}
	out.Name, out.LengthUnit, out.Solid = name, unit, s
	values := []*big.Rat{p.Center[0], p.Center[1], p.LowerZ, p.Radius, new(big.Rat).Sub(p.UpperZ, p.LowerZ), p.Fillets[0], p.Fillets[1]}
	v := make([]float64, len(values))
	for i, q := range values {
		v[i], e = geom.RepresentedExactScalar(q)
		if e != nil {
			return WrappedRimFilletBody{}, e
		}
	}
	out.Center = geom.Vec3{X: v[0], Y: v[1], Z: v[2]}
	out.Radius, out.Height = v[3], v[4]
	out.Fillets = [2]float64{v[5], v[6]}
	ids := []int{}
	if p.Blends[0] >= 0 {
		ids = append(ids, p.Blends[0])
	}
	ids = append(ids, p.Lateral)
	if p.Blends[1] >= 0 {
		ids = append(ids, p.Blends[1])
	}
	faces := s.Faces()
	for _, id := range ids {
		w, ok := faces[id].Surface.(*geom.WrappedSurfacePatch)
		if !ok {
			return WrappedRimFilletBody{}, geom.ErrConvergence
		}
		out.Walls = append(out.Walls, WrappedSplineFace{Geometry: w})
	}
	return out, nil
}
