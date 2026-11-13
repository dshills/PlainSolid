package parametric

import (
	"context"
	"math"
	"math/big"
	"plainsolid/geom"
)

// SteinmetzQuadrant is the equal-radius perpendicular common with one exact
// equatorial cap on each cylinder axis. PositiveX/PositiveZ select the retained
// halfspaces relative to Center. Two genuine cylinder faces and two planar
// half disks retain shared curves and two collapsed poles.
type SteinmetzQuadrant struct {
	Solid                                    Solid
	XAxisFace, ZAxisFace, XCapFace, ZCapFace int
	PositiveX, PositiveZ                     bool
	IdealVolume                              *big.Rat
}

// PerpendicularCylinderQuadrant bounds both finite cylinders at their center
// plane, with each opposite end strictly beyond the relevant radial extent.
// The coordinate-permuted disk uses the existing native mapped-surface codec;
// no approximate clipping or polygonal surrogate is introduced.
func PerpendicularCylinderQuadrant(ctx context.Context, center geom.Vec3, radius, xOtherHalfLength, zOtherHalfLength, tolerance float64, positiveX, positiveZ bool) (SteinmetzQuadrant, error) {
	fail := func(e error) (SteinmetzQuadrant, error) { return SteinmetzQuadrant{}, e }
	full, e := PerpendicularCylinderCommon(ctx, center, radius, xOtherHalfLength, zOtherHalfLength, tolerance)
	if e != nil {
		return fail(e)
	}
	sx, sz := -1, -1
	vx, vz := Interval{-1, 0}, Interval{-1, 0}
	if positiveX {
		sx = 1
		vx = Interval{0, 1}
	}
	if positiveZ {
		sz = 1
		vz = Interval{0, 1}
	}
	u := Interval{-math.Pi / 2, math.Pi / 2}
	swap := geom.Affine{{0, 0, 1, center.X}, {0, 1, 0, center.Y}, {1, 0, 0, center.Z}}
	faces := []Face{
		{Surface: steinmetzChart{center, radius, 0, sx}, U: u, V: vz, Reversed: sx < 0},
		{Surface: steinmetzChart{center, radius, 1, sz}, U: u, V: vx, Reversed: sz > 0},
		{Surface: steinmetzDisk{center, radius}, U: u, V: vz, Reversed: positiveX},
		{Surface: MapSurface(steinmetzDisk{geom.Vec3{}, radius}, swap), U: u, V: vx, Reversed: !positiveZ},
	}
	solid, e := joinPrimitivePatches(faces, tolerance)
	if e != nil {
		return fail(e)
	}
	if e = ctx.Err(); e != nil {
		return fail(e)
	}
	return SteinmetzQuadrant{Solid: solid, ZAxisFace: 0, XAxisFace: 1, XCapFace: 2, ZCapFace: 3, PositiveX: positiveX, PositiveZ: positiveZ, IdealVolume: new(big.Rat).Quo(full.IdealVolume, big.NewRat(4, 1))}, nil
}
