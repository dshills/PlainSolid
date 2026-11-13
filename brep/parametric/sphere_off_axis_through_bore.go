package parametric

import (
	"context"
	"math"

	"plainsolid/geom"
)

// SphereOffAxisThroughBoreResult identifies the native outward spherical
// annuli and inward cylindrical wall of a sphere pierced by a round hole.
type SphereOffAxisThroughBoreResult struct {
	Solid                                    Solid
	LowerSphericalFaces, UpperSphericalFaces []int
	CylindricalFaces                         []int
}

// SphereOffAxisThroughBore subtracts an infinite circular cylinder with
// local Z axis through (offset,0) from a sphere centered on frame.Origin.
// The tool footprint must be strictly contained in the sphere's equatorial
// disk. Both transverse sphere/cylinder rims are shared by the native shell.
func SphereOffAxisThroughBore(ctx context.Context, frame geom.Frame, sphereRadius, boreRadius, offset, tolerance float64) (SphereOffAxisThroughBoreResult, error) {
	var empty SphereOffAxisThroughBoreResult
	if ctx == nil || !finite(tolerance) || tolerance <= 0 {
		return empty, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return empty, e
	}
	cfg := offsetBoreChart{frame: frame, radius: sphereRadius, bore: boreRadius, offset: offset, kind: toolBoreLowerAnnulus}
	chart, e := newToolBoreChart(ctx, cfg, tolerance)
	if e != nil {
		return empty, e
	}
	var faces []Face
	var out SphereOffAxisThroughBoreResult
	for i := range 4 {
		u := Interval{float64(i) * math.Pi / 2, float64(i+1) * math.Pi / 2}
		v := Interval{0, 1}
		chart.kind = toolBoreLowerAnnulus
		out.LowerSphericalFaces = append(out.LowerSphericalFaces, len(faces))
		faces = append(faces, Face{Surface: chart, U: u, V: v})
		chart.kind = toolBoreUpper
		out.UpperSphericalFaces = append(out.UpperSphericalFaces, len(faces))
		faces = append(faces, Face{Surface: chart, U: u, V: v, Reversed: true})
		chart.kind = toolBoreThroughWall
		out.CylindricalFaces = append(out.CylindricalFaces, len(faces))
		faces = append(faces, Face{Surface: chart, U: u, V: v, Reversed: true})
	}
	out.Solid, e = joinPrimitivePatches(faces, tolerance)
	if e != nil {
		return empty, e
	}
	if e := ctx.Err(); e != nil {
		return empty, e
	}
	return out, nil
}
