package trimmed

import (
	"context"
	"math/big"

	"plainsolid/brep/edgecheck"
	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

// WrappedLoftStation gives a represented model Z and positive section radius.
type WrappedLoftStation struct{ Z, Radius float64 }

// WrappedLoftHistory maps stations to their shared four edge rims and
// successive station pairs to their retained lateral faces.
type WrappedLoftHistory struct {
	StationRims [][4]int
	WallFaces   []int
	CapFaces    [2]int
}

// LoftWrappedCircularStations constructs a globally embedded rational spline
// loft along the Z axis, with circular stations and affine radii between
// stations. The two end caps are planar. Interior station rims have two
// exact lateral uses and no cap. Different slopes retain a physical crease.
func LoftWrappedCircularStations(ctx context.Context, center geom.Vec2, stations []WrappedLoftStation, tolerance float64,
	o geom.SurfacePatchOptions) (Solid, WrappedLoftHistory, error) {
	var h WrappedLoftHistory
	if ctx == nil || !uvFinite(center) || !finite(tolerance) || tolerance <= 0 || len(stations) < 2 || len(stations) > 128 {
		return Solid{}, h, geom.ErrInvalid
	}
	for i, s := range stations {
		if !finite(s.Z) || !finite(s.Radius) || s.Radius <= 0 || i > 0 && s.Z <= stations[i-1].Z {
			return Solid{}, h, geom.ErrInvalid
		}
	}
	base, e := geom.NewCanonicalWrappedCylinder(ctx, geom.Vec3{X: center.X, Y: center.Y}, stations[0].Radius, 1, o)
	if e != nil {
		return Solid{}, h, e
	}
	bands := make([]*geom.WrappedSurfacePatch, 0, len(stations)-1)
	for i, lower := range stations[:len(stations)-1] {
		if e := ctx.Err(); e != nil {
			return Solid{}, WrappedLoftHistory{}, e
		}
		upper := stations[i+1]
		w, e := geom.ReheightWrappedCylinder(ctx, base, ratInset(lower.Z), ratInset(upper.Z), o)
		if e != nil {
			return Solid{}, WrappedLoftHistory{}, e
		}
		delta := func(r float64) *big.Rat {
			return new(big.Rat).Sub(ratInset(r), ratInset(stations[0].Radius))
		}
		w, _, e = geom.DraftWrappedFrustumExact(ctx, w, delta(lower.Radius), delta(upper.Radius), o)
		if e != nil {
			return Solid{}, WrappedLoftHistory{}, e
		}
		bands = append(bands, w)
	}
	return joinWrappedStationBands(ctx, bands, tolerance, o)
}

// NewWrappedCircularStationLoft joins the original circular frustum charts
// at their exact ordered station rims and caps the two ends. Each band must
// share the same exact axis and match its neighbor's full rim evaluator.
// Existing circular parameterizations and original knot units are preserved.
func NewWrappedCircularStationLoft(ctx context.Context, bands []*geom.WrappedSurfacePatch, tolerance float64,
	o geom.SurfacePatchOptions) (Solid, WrappedLoftHistory, error) {
	if ctx == nil || len(bands) < 1 || len(bands) > 127 || !finite(tolerance) || tolerance <= 0 {
		return Solid{}, WrappedLoftHistory{}, geom.ErrInvalid
	}
	var previous geom.WrappedFrustumEmbedding
	for i, band := range bands {
		if e := ctx.Err(); e != nil {
			return Solid{}, WrappedLoftHistory{}, e
		}
		p, e := geom.CertifyWrappedFrustumEmbedding(ctx, band, o)
		if e != nil {
			return Solid{}, WrappedLoftHistory{}, e
		}
		if !p.Circular || i > 0 && (p.CenterX.Cmp(previous.CenterX) != 0 || p.CenterY.Cmp(previous.CenterY) != 0 ||
			p.LowerZ.Cmp(previous.UpperZ) != 0 || p.LowerRadius.Cmp(previous.UpperRadius) != 0) {
			return Solid{}, WrappedLoftHistory{}, geom.ErrConvergence
		}
		previous = p
	}
	return joinWrappedStationBands(ctx, bands, tolerance, o)
}

// joinWrappedStationBands assumes exact ordered full-profile station continuity.
// Every shared rim is independently checked by complete model/pcurve identity.
func joinWrappedStationBands(ctx context.Context, bands []*geom.WrappedSurfacePatch, tolerance float64,
	o geom.SurfacePatchOptions) (Solid, WrappedLoftHistory, error) {
	return joinWrappedStationBandsUsing(ctx, bands, tolerance, o, func(ctx context.Context, w *geom.WrappedSurfacePatch, tolerance float64, o geom.SurfacePatchOptions) (Solid, error) {
		s, _, e := NewWrappedFrustum(ctx, w, tolerance, o)
		return s, e
	})
}

// The builder must prove each complete embedded capped source band. Shared rim
// equality and native incidence are independently re-proved below.
func joinWrappedStationBandsUsing(ctx context.Context, bands []*geom.WrappedSurfacePatch, tolerance float64, o geom.SurfacePatchOptions, build func(context.Context, *geom.WrappedSurfacePatch, float64, geom.SurfacePatchOptions) (Solid, error)) (Solid, WrappedLoftHistory, error) {
	h := WrappedLoftHistory{StationRims: make([][4]int, len(bands)+1)}
	var vertices []geom.Vec3
	var edges []parametric.Edge
	var faces []Face
	var endCaps [2]Face
	for i, w := range bands {
		if e := ctx.Err(); e != nil {
			return Solid{}, WrappedLoftHistory{}, e
		}
		segment, e := build(ctx, w, tolerance, o)
		if e != nil {
			return Solid{}, WrappedLoftHistory{}, e
		}
		mapping := [9]int{}
		for j := range 2 {
			if j == 0 && i > 0 {
				for k := range 4 {
					mapping[k] = h.StationRims[i][k]
					want := vertices[edges[mapping[k]].Start]
					if segment.vertices[k] != want {
						return Solid{}, WrappedLoftHistory{}, geom.ErrConvergence
					}
				}
				continue
			}
			n := len(vertices)
			vertices = append(vertices, segment.vertices[4*j:4*j+4]...)
			for k := range 4 {
				edge := segment.edges[4*j+k]
				edge.Start, edge.End = n+k, n+(k+1)%4
				mapping[4*j+k] = len(edges)
				h.StationRims[i+j][k] = len(edges)
				edges = append(edges, edge)
			}
		}
		seam := segment.edges[8]
		seam.Start, seam.End = edges[mapping[0]].Start, edges[mapping[4]].Start
		mapping[8] = len(edges)
		edges = append(edges, seam)
		remap := func(f Face) Face {
			f = cloneFace(f)
			for j := range f.Outer {
				f.Outer[j].Edge = mapping[f.Outer[j].Edge]
			}
			return f
		}
		h.WallFaces = append(h.WallFaces, len(faces))
		faces = append(faces, remap(segment.faces[0]))
		if i == 0 {
			endCaps[0] = remap(segment.faces[1])
		}
		if i == len(bands)-1 {
			endCaps[1] = remap(segment.faces[2])
		}
	}
	h.CapFaces = [2]int{len(faces), len(faces) + 1}
	faces = append(faces, endCaps[:]...)
	out, e := NewSolidContext(ctx, vertices, edges, faces, tolerance)
	if e != nil {
		return Solid{}, WrappedLoftHistory{}, e
	}
	p, e := out.ValidateEdgeAgreement(ctx, edgecheck.Options{MaxWork: o.MaxOperations, MaxBits: o.MaxCoefficientBits})
	if e != nil || !p.Complete || out.Topology().EulerCharacteristic != 2 {
		if e == nil {
			e = geom.ErrConvergence
		}
		return Solid{}, WrappedLoftHistory{}, e
	}
	for _, u := range p.Uses {
		if u.Report.SquaredUpperBound == nil || u.Report.SquaredUpperBound.Sign() != 0 {
			return Solid{}, WrappedLoftHistory{}, geom.ErrConvergence
		}
	}
	return out, h, nil
}
