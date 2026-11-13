package step

import (
	"context"
	"io"
	"math/big"
	"plainsolid/brep/trimmed"
	"plainsolid/geom"
)

// Single-band adaptation is private to the bore API; the standalone station
// loft API keeps its multi-band contract.
func stationBoreSingleDescriptor(b WrappedFrustumBody, unit float64) (WrappedStationLoftBody, error) {
	p := b.Embedding
	if !p.Complete || !p.Circular {
		return WrappedStationLoftBody{}, geom.ErrConvergence
	}
	out := WrappedStationLoftBody{Name: b.Name, LengthUnit: unit, CapNames: b.CapNames, Walls: []WrappedSplineFace{b.Face}, Solid: b.Solid}
	values := []*big.Rat{p.CenterX, p.CenterY, p.LowerZ, p.UpperZ, p.LowerRadius, p.UpperRadius}
	v := make([]float64, len(values))
	for i, q := range values {
		x, e := geom.RepresentedExactScalar(q)
		if e != nil {
			return out, e
		}
		v[i] = x
	}
	out.Center = geom.Vec2{X: v[0], Y: v[1]}
	out.Stations = []trimmed.WrappedLoftStation{{Z: v[2], Radius: v[4]}, {Z: v[3], Radius: v[5]}}
	return out, nil
}
func stationBoreLoftFromSolid(ctx context.Context, s trimmed.Solid, name string, unit float64, o geom.SurfacePatchOptions) (WrappedStationLoftBody, error) {
	p, e := s.ValidateWrappedCircularStationLoft(ctx, o)
	if e != nil {
		return WrappedStationLoftBody{}, e
	}
	if len(p.Heights) != 2 {
		return WrappedStationLoftFromSolid(ctx, s, name, unit, o)
	}
	if !validLengthUnit(unit) || !p.Complete || len(p.History.WallFaces) != 1 {
		return WrappedStationLoftBody{}, geom.ErrInvalid
	}
	w, ok := s.Faces()[p.History.WallFaces[0]].Surface.(*geom.WrappedSurfacePatch)
	if !ok {
		return WrappedStationLoftBody{}, geom.ErrConvergence
	}
	proof, e := geom.CertifyWrappedFrustumEmbedding(ctx, w, o)
	if e != nil {
		return WrappedStationLoftBody{}, e
	}
	return stationBoreSingleDescriptor(WrappedFrustumBody{Name: name, Face: WrappedSplineFace{Geometry: w}, Solid: s, Embedding: proof}, unit)
}
func writeStationBoreLoft(w io.Writer, b WrappedStationLoftBody) error {
	if len(b.Stations) != 2 {
		return WriteWrappedStationLoft(w, b)
	}
	if len(b.Walls) > 1 {
		return geom.ErrInvalid
	}
	s, h, e := trimmed.LoftWrappedCircularStations(context.Background(), b.Center, b.Stations, 1e-10, geom.SurfacePatchOptions{})
	if e != nil {
		return e
	}
	face := WrappedSplineFace{Geometry: s.Faces()[h.WallFaces[0]].Surface.(*geom.WrappedSurfacePatch)}
	if len(b.Walls) == 1 {
		face.Name = b.Walls[0].Name
		if b.Walls[0].Geometry != nil {
			face.Geometry = b.Walls[0].Geometry
		}
	}
	proof, e := geom.CertifyWrappedFrustumEmbedding(context.Background(), face.Geometry, geom.SurfacePatchOptions{})
	if e != nil {
		return e
	}
	same := func(q *big.Rat, x float64) bool { return q != nil && q.Cmp(new(big.Rat).SetFloat64(x)) == 0 }
	if !proof.Complete || !proof.Circular || !same(proof.CenterX, b.Center.X) || !same(proof.CenterY, b.Center.Y) || !same(proof.LowerZ, b.Stations[0].Z) || !same(proof.UpperZ, b.Stations[1].Z) || !same(proof.LowerRadius, b.Stations[0].Radius) || !same(proof.UpperRadius, b.Stations[1].Radius) {
		return geom.ErrConvergence
	}
	return WriteWrappedFrusta(w, WrappedFrustumModel{LengthUnit: b.LengthUnit, Bodies: []WrappedFrustumBody{{Name: b.Name, Face: face, CapNames: b.CapNames}}})
}
