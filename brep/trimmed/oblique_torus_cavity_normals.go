package trimmed

import (
	"context"
	"math/big"
	"plainsolid/brep/placement"
	"plainsolid/geom"
)

// NativeObliqueCavityNormals retains every original face incident at the exact
// rational boundary point, and an exact outward material unit normal per
// analytic owner. Smooth chart seams share a normal; cap-wall creases keep both.
type NativeObliqueCavityNormals struct {
	geom.ObliqueCavityRationalNormals
	OriginalFaces [4][]int
}

func (p *PreparedObliqueTorusCavity) BoundaryUnitNormalsRational(ctx context.Context, point [3]*big.Rat, o geom.SurfacePatchOptions) (NativeObliqueCavityNormals, error) {
	var zero NativeObliqueCavityNormals
	if p == nil || p.major == nil {
		return zero, geom.ErrInvalid
	}
	normals, e := p.source.BoundaryUnitNormalsRational(ctx, point, o)
	if e != nil {
		return zero, e
	}
	out := NativeObliqueCavityNormals{ObliqueCavityRationalNormals: normals}
	for i := 1; i < 4; i++ {
		if normals.Owners&geom.BoreBoundaryMask(1<<i) != 0 {
			out.OriginalFaces[i] = []int{i - 1}
		}
	}
	if normals.Owners&geom.TorusCylinderTorus != 0 {
		signs := normals.TorusQuarterSigns
		quarters := func(x, y int) []int {
			var q []int
			for i, s := range [][2]int{{1, 1}, {-1, 1}, {-1, -1}, {1, -1}} {
				if (x == 0 || x == s[0]) && (y == 0 || y == s[1]) {
					q = append(q, i)
				}
			}
			return q
		}
		for _, u := range quarters(signs[0], signs[1]) {
			for _, v := range quarters(signs[2], signs[3]) {
				out.OriginalFaces[0] = append(out.OriginalFaces[0], 3+4*u+v)
			}
		}
	}
	return out, nil
}

// BoundaryPlacedUnitNormalsRational pulls back the exact point once and maps
// unit vectors through the proper whole occurrence. No normalization occurs;
// point/vector mapping has a separate 4096-bit limit and at most 5 transforms.
// Returned face IDs and quarter signs remain in the original native source.
func (p *PreparedObliqueTorusCavity) BoundaryPlacedUnitNormalsRational(ctx context.Context, pose placement.RationalRigid, point [3]*big.Rat, o geom.SurfacePatchOptions) (NativeObliqueCavityNormals, error) {
	var zero NativeObliqueCavityNormals
	local, e := pose.TransformPoint(ctx, point, true, 4096)
	if e != nil {
		return zero, e
	}
	out, e := p.BoundaryUnitNormalsRational(ctx, local, o)
	if e != nil {
		return zero, e
	}
	for i, unit := range out.Units {
		if out.Owners&geom.BoreBoundaryMask(1<<i) == 0 {
			continue
		}
		out.Units[i], e = pose.TransformVector(ctx, unit, false, 4096)
		if e != nil {
			return zero, e
		}
	}
	return out, nil
}
