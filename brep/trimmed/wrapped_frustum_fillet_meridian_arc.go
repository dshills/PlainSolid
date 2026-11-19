//go:build research

package trimmed

import (
	"context"
	"math/big"
	"plainsolid/geom"
)

// FilletedFrustumNearestMeridianArc certifies every point
// Center+Radius*(Radial*a+Axis*b), where a>=0, a²+b²=1 and
// AxialNormal[0]<=b<=AxialNormal[1], as globally nearest. Radial/Axis are
// exact orthonormal vectors. EndNormals indexes cap then cone endpoint;
// EndpointFaces and RimEdges retain each original paired boundary circle.
// Face owns the arc interior. This subset is not an exhaustive nearest locus.
type FilletedFrustumNearestMeridianArc struct {
	Center, Radial, Axis    [3]*big.Rat
	Radius, SquaredDistance *big.Rat
	AxialNormal             [2]*big.Rat
	EndNormals              [2][2]*big.Rat
	Face                    int
	EndpointFaces           [2][2]int
	RimEdges                [2][]int
}

type FilletedFrustumMeridianArcDistance struct {
	Baseline FilletedFrustumDistance
	Arc      *FilletedFrustumNearestMeridianArc
}

// BoundToroidCenterNearestArcRational accepts a query at the exact meridian
// center of an original rolling torus. Its positive rational azimuth is
// derived without square roots. Equal azimuth minimizes full torus distance;
// all retained meridian normals then tie at radius². One all-face query and
// two fixed center checks suffice. Other support lower bounds must separate.
// Axis queries and unproved ties return only the conservative baseline.
func (q FilletedFrustumMaterial) BoundToroidCenterNearestArcRational(ctx context.Context, p [3]*big.Rat, o FilletedFrustumDistanceOptions) (out FilletedFrustumMeridianArcDistance, err error) {
	defer func() {
		if err != nil {
			out = FilletedFrustumMeridianArcDistance{}
		}
	}()
	base, e := q.BoundBoundaryDistanceRational(ctx, p, o)
	if e != nil {
		return out, e
	}
	out.Baseline = base
	dx, dy := fdSub(p[0], q.center[0]), fdSub(p[1], q.center[1])
	S := fdAdd(fdSquare(dx), fdSquare(dy))
	for end, c := range q.fillets.Circles {
		if e := ctx.Err(); e != nil {
			return out, e
		}
		if c.Center[0].Sign() <= 0 || p[2].Cmp(c.Center[1]) != 0 || S.Cmp(fdSquare(c.Center[0])) != 0 {
			continue
		}
		d := fdSquare(c.Radius)
		if !base.Exact || base.Lower.Cmp(d) != 0 {
			continue
		}
		separated := true
		for _, f := range base.PerFace {
			if f.Face != q.fillets.Blends[end] && f.SquaredLower.Cmp(d) < 0 {
				separated = false
				break
			}
		}
		if !separated {
			continue
		}
		capNormal := big.NewRat(-1, 1)
		if end == 1 {
			capNormal.SetInt64(1)
		}
		lo, hi := new(big.Rat).Set(capNormal), new(big.Rat).Set(c.Normal[1])
		if lo.Cmp(hi) > 0 {
			lo, hi = hi, lo
		}
		a := &FilletedFrustumNearestMeridianArc{Center: [3]*big.Rat{new(big.Rat).Set(p[0]), new(big.Rat).Set(p[1]), new(big.Rat).Set(p[2])}, Radial: [3]*big.Rat{new(big.Rat).Quo(dx, c.Center[0]), new(big.Rat).Quo(dy, c.Center[0]), new(big.Rat)}, Axis: [3]*big.Rat{new(big.Rat), new(big.Rat), big.NewRat(1, 1)}, Radius: new(big.Rat).Set(c.Radius), SquaredDistance: d, AxialNormal: [2]*big.Rat{lo, hi}, EndNormals: [2][2]*big.Rat{{new(big.Rat), capNormal}, {new(big.Rat).Set(c.Normal[0]), new(big.Rat).Set(c.Normal[1])}}, Face: q.fillets.Blends[end], EndpointFaces: [2][2]int{{q.fillets.Blends[end], q.fillets.Caps[end]}, {q.fillets.Blends[end], q.fillets.Lateral}}}
		for i, faces := range a.EndpointFaces {
			a.RimEdges[i] = q.solid.filletedSharedRimEdges(faces[0], faces[1])
			if len(a.RimEdges[i]) == 0 {
				return out, geom.ErrConvergence
			}
		}
		out.Arc = a
		return out, nil
	}
	return out, nil
}

type FilletedFrustumOccurrenceMeridianArcDistance struct {
	Original FilletedFrustumMeridianArcDistance
	WorldArc *FilletedFrustumNearestMeridianArc
}

// BoundToroidCenterNearestArcRational transforms the entire certified arc,
// not one azimuth sample, using exact proper rigid coefficients and original
// owner IDs. Pose arithmetic is bounded to4096 bits and failures are atomic.
func (q FilletedFrustumOccurrence) BoundToroidCenterNearestArcRational(ctx context.Context, p [3]*big.Rat, o FilletedFrustumDistanceOptions) (out FilletedFrustumOccurrenceMeridianArcDistance, err error) {
	defer func() {
		if err != nil {
			out = FilletedFrustumOccurrenceMeridianArcDistance{}
		}
	}()
	if q.source.recipe == nil {
		return out, geom.ErrInvalid
	}
	local, e := q.pose.TransformPoint(ctx, p, true, 4096)
	if e != nil {
		return out, e
	}
	original, e := q.source.BoundToroidCenterNearestArcRational(ctx, local, o)
	if e != nil {
		return out, e
	}
	out.Original = original
	if original.Arc == nil {
		return out, nil
	}
	a := original.Arc
	center, e := q.pose.TransformPoint(ctx, a.Center, false, 4096)
	if e != nil {
		return out, e
	}
	radial, e := q.pose.TransformVector(ctx, a.Radial, false, 4096)
	if e != nil {
		return out, e
	}
	axis, e := q.pose.TransformVector(ctx, a.Axis, false, 4096)
	if e != nil {
		return out, e
	}
	w := &FilletedFrustumNearestMeridianArc{Center: center, Radial: radial, Axis: axis, Radius: new(big.Rat).Set(a.Radius), SquaredDistance: new(big.Rat).Set(a.SquaredDistance), Face: a.Face, EndpointFaces: a.EndpointFaces}
	for i := 0; i < 2; i++ {
		w.AxialNormal[i] = new(big.Rat).Set(a.AxialNormal[i])
		w.RimEdges[i] = append([]int(nil), a.RimEdges[i]...)
		for j := 0; j < 2; j++ {
			w.EndNormals[i][j] = new(big.Rat).Set(a.EndNormals[i][j])
		}
	}
	out.WorldArc = w
	return out, nil
}
