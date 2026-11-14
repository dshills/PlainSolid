package trimmed

import (
	"context"
	"fmt"
	"plainsolid/geom"
)

// TessellateBoundaryCertified retains original model coordinates and exact
// chart identities for the common closed seam, including non-XY projections.
func (l AlgebraicProjectedTubeLens) TessellateBoundaryCertified(ctx context.Context, o AlgebraicTubeLensBoundaryOptions) (CertifiedAlgebraicTubeLensBoundary, error) {
	if ctx == nil || l.graphs[0] == nil {
		return CertifiedAlgebraicTubeLensBoundary{}, geom.ErrInvalid
	}
	f := l.Faces()
	reverse := [2]bool{f[0].Reversed != f[0].CutReversed, f[1].Reversed != f[1].CutReversed}
	return tessellateTubeLensBoundary(ctx, l.section, l.faces[0].Branch, reverse, o)
}

// TessellateClosedCertified uses an exact permutation of model coordinates to
// apply the common-projection fill proof. Permutation preserves all Euclidean
// error bounds and exact contacts. Odd permutations reverse triangle orientation
// on restoration. Original chart/source identities and shared seam indices are
// retained; no rounded refit or independent section discovery is performed.
// Limits and source-distance semantics are those of the multi-span lens mesher.
func (l AlgebraicProjectedTubeLens) TessellateClosedCertified(ctx context.Context, o MultiSpanTubeLensMeshOptions) (CertifiedAlgebraicTubeLensMesh, error) {
	var empty CertifiedAlgebraicTubeLensMesh
	if ctx == nil || l.graphs[0] == nil {
		return empty, geom.ErrInvalid
	}
	work, e := newTubeLensSpanWork(o)
	if e != nil {
		return empty, e
	}
	if o.Closed.Boundary.Run.Display.Point.Tolerance == 0 {
		o.Closed.Boundary.Run.Display.Point.Tolerance = o.Closed.Boundary.Run.Display.Deflection * 1e-6
	}
	boundary, e := l.TessellateBoundaryCertified(ctx, o.Closed.Boundary)
	if e != nil {
		return empty, fmt.Errorf("projected boundary: %w", e)
	}
	axes := [3]int{l.axes[0], l.axes[1], l.residual}
	forward := func(p geom.Vec3) geom.Vec3 {
		v := [3]float64{p.X, p.Y, p.Z}
		return geom.Vec3{X: v[axes[0]], Y: v[axes[1]], Z: v[axes[2]]}
	}
	inverse := func(p geom.Vec3) geom.Vec3 {
		var v [3]float64
		v[axes[0]], v[axes[1]], v[axes[2]] = p.X, p.Y, p.Z
		return geom.Vec3{X: v[0], Y: v[1], Z: v[2]}
	}
	virtual := AlgebraicTubeLens{faces: l.faces, upper: l.upper}
	for i := 0; i < 2; i++ {
		source, patch, e := permuteTubeLensSupport(l.faces[i], axes, o.Closed.Boundary.Run.Partition.Patches)
		if e != nil {
			return empty, e
		}
		virtual.faces[i].SourceSurface = source
		virtual.faces[i].Support, _ = source.(*geom.BSplineSurface)
		virtual.graphs[i], e = geom.NewAffineXYGraph(ctx, patch, o.Closed.Boundary.Run.Partition.Patches)
		if e != nil {
			return empty, e
		}
	}
	projected := boundary
	projected.Run.Vertices = append([]geom.Vec3(nil), boundary.Run.Vertices...)
	for i, p := range projected.Run.Vertices {
		projected.Run.Vertices[i] = forward(p)
	}
	out, e := virtual.tessellateClosedTubeLens(ctx, o.Closed, &work, true, &projected, nil)
	if e != nil {
		return empty, fmt.Errorf("projected fill: %w", e)
	}
	for i, p := range out.Mesh.Vertices {
		out.Mesh.Vertices[i] = inverse(p)
	}
	if projectedAxisParity(l.axes) < 0 {
		for i := range out.Mesh.Triangles {
			t := &out.Mesh.Triangles[i]
			t[1], t[2] = t[2], t[1]
		}
	}
	out.Boundary = boundary
	// The exact isometry transports the completed contact proof and positive
	// volume. Revalidate combinatorial incidence in the returned ordering.
	if e := out.Mesh.ValidateSolid(); e != nil {
		return empty, e
	}
	return out, nil
}
