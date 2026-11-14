package trimmed

import (
	"context"
	"math/big"
	"plainsolid/geom"
	"reflect"
	"testing"
)

func TestNativeRadialTubeRegionPlanarMoments(t *testing.T) {
	ctx := context.Background()
	s := radialMaterialForBounds(t)
	center := [2]*big.Rat{big.NewRat(1, 2), big.NewRat(1, 2)}
	radii := []*big.Rat{big.NewRat(5, 16)}
	out, e := s.RadialTubeRegionPlanarMoments(ctx, center, radii, geom.RadialGraphMomentOptions{})
	if e != nil {
		t.Fatal(e)
	}
	volume, e := s.BoundRadialTubeRegionVolume(ctx, center, radii, geom.RadialGraphVolumeOptions{})
	if e != nil {
		t.Fatal(e)
	}
	if !reflect.DeepEqual(out.Components, volume.Components) || len(out.Moments.Bands) != 1 || out.Moments.Bands[0].Band != out.Components[0].Band || out.Moments.Total.VolumePiCoefficient.Cmp(volume.Volume.PiCoefficient) != 0 {
		t.Fatal("original band ownership mismatch", out, volume)
	}
	m := out.Moments.Total
	v := big.NewRat(625, 131072)
	if m.ProjectedAreaPiCoefficient.Cmp(big.NewRat(25, 256)) != 0 || m.VolumePiCoefficient.Cmp(v) != 0 {
		t.Fatal("independent area/volume", m)
	}
	wantSecond := new(big.Rat).Add(new(big.Rat).Quo(v, big.NewRat(4, 1)), big.NewRat(15625, 201326592))
	if m.SecondMomentPiCoefficients[0].Cmp(wantSecond) != 0 || m.SecondMomentPiCoefficients[2].Cmp(wantSecond) != 0 || m.SecondMomentPiCoefficients[1].Cmp(new(big.Rat).Quo(v, big.NewRat(4, 1))) != 0 {
		t.Fatal("independent disk moments", m)
	}
	work := out.Moments.ExactOperations
	if _, e = s.RadialTubeRegionPlanarMoments(ctx, center, radii, geom.RadialGraphMomentOptions{MaxOperations: work}); e != nil {
		t.Fatal("exact total quota", e)
	}
	if x, e := s.RadialTubeRegionPlanarMoments(ctx, center, radii, geom.RadialGraphMomentOptions{MaxOperations: work - 1}); e == nil || x.Moments.Total.VolumePiCoefficient != nil || len(x.Components) != 0 {
		t.Fatal("atomic deficit", x, e)
	}
	s.Faces = nil
	s.Edges = nil
	s.Vertices = nil
	out.Components[0].Band = 99
	out.Moments.Total.CentroidXY[0].SetInt64(9)
	x, e := s.RadialTubeRegionPlanarMoments(ctx, center, radii, geom.RadialGraphMomentOptions{})
	if e != nil || x.Components[0].Band != 0 || x.Moments.Total.CentroidXY[0].Cmp(center[0]) != 0 {
		t.Fatal("public diagnostics or returned copies changed source", e)
	}
	bad := s
	bad.faces = append([]AlgebraicShellBooleanFace(nil), s.faces...)
	bad.faces[0].Outer = append([]AlgebraicShellBooleanUse(nil), bad.faces[0].Outer[1:]...)
	if x, e := bad.RadialTubeRegionPlanarMoments(ctx, center, radii, geom.RadialGraphMomentOptions{}); e == nil || len(x.Components) != 0 {
		t.Fatal("missing original edge", e)
	}
	bad = s
	bad.faces = append([]AlgebraicShellBooleanFace(nil), s.faces...)
	bad.faces[1].Reversed = !bad.faces[1].Reversed
	if x, e := bad.RadialTubeRegionPlanarMoments(ctx, center, radii, geom.RadialGraphMomentOptions{}); e == nil || len(x.Components) != 0 {
		t.Fatal("wrong material side", e)
	}
	if x, e := s.RadialTubeRegionPlanarMoments(ctx, center, []*big.Rat{big.NewRat(1, 4)}, geom.RadialGraphMomentOptions{}); e == nil || len(x.Components) != 0 {
		t.Fatal("unproved circle", e)
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if x, e := s.RadialTubeRegionPlanarMoments(cancelled, center, radii, geom.RadialGraphMomentOptions{}); e != context.Canceled || len(x.Components) != 0 {
		t.Fatal("cancellation", e)
	}
	mass, e := s.RadialTubeRegionMassMoments(ctx, center, radii, geom.RadialGraphMomentOptions{})
	if e != nil {
		t.Fatal(e)
	}
	if !reflect.DeepEqual(mass.Components, volume.Components) || mass.Moments.Height.CentroidZ.Cmp(big.NewRat(25, 768)) != 0 || mass.Moments.Planar.Total.CentroidXY[0].Cmp(center[0]) != 0 || mass.Moments.Planar.Total.CentroidXY[1].Cmp(center[1]) != 0 {
		t.Fatal("original material XYZ centroid", mass)
	}
	if x, e := s.RadialTubeRegionMassMoments(ctx, center, radii, geom.RadialGraphMomentOptions{MaxOperations: mass.Moments.ExactOperations - 1}); e == nil || x.Moments.Height.CentroidZ != nil || len(x.Components) != 0 {
		t.Fatal("atomic centroid quota", e)
	}
	if _, e := s.RadialTubeRegionMassMoments(ctx, center, radii, geom.RadialGraphMomentOptions{MaxOperations: mass.Moments.ExactOperations}); e != nil {
		t.Fatal("exact centroid quota", e)
	}
	if x, e := bad.RadialTubeRegionMassMoments(ctx, center, radii, geom.RadialGraphMomentOptions{}); e == nil || len(x.Components) != 0 {
		t.Fatal("wrong original centroid owner side", e)
	}
	if x, e := s.RadialTubeRegionMassMoments(cancelled, center, radii, geom.RadialGraphMomentOptions{}); e != context.Canceled || len(x.Components) != 0 {
		t.Fatal("centroid cancellation", e)
	}
	tensor, e := s.RadialTubeRegionMassTensor(ctx, center, radii, geom.RadialGraphMomentOptions{})
	if e != nil {
		t.Fatal(e)
	}
	if !reflect.DeepEqual(tensor.Components, volume.Components) || tensor.Tensor.Total.SecondMomentPiCoefficients[2][2].Cmp(big.NewRat(390625, 51539607552)) != 0 || tensor.Tensor.Total.SecondMomentPiCoefficients[0][2].Cmp(big.NewRat(15625, 201326592)) != 0 || tensor.Tensor.Total.SecondMomentPiCoefficients[1][2].Cmp(big.NewRat(15625, 201326592)) != 0 {
		t.Fatal("independent original material tensor", tensor)
	}
	for i := range 3 {
		for j := range 3 {
			want := new(big.Rat).Neg(tensor.Tensor.Total.SecondMomentPiCoefficients[i][j])
			if i == j {
				for k := range 3 {
					want.Add(want, tensor.Tensor.Total.SecondMomentPiCoefficients[k][k])
				}
			}
			if tensor.Tensor.Total.OriginInertiaPiCoefficients[i][j].Cmp(want) != 0 {
				t.Fatal("original world-origin inertia", i, j)
			}
		}
	}
	if x, e := s.RadialTubeRegionMassTensor(ctx, center, radii, geom.RadialGraphMomentOptions{MaxOperations: tensor.Tensor.ExactOperations - 1}); e == nil || len(x.Components) != 0 || x.Tensor.Total.SecondMomentPiCoefficients[0][0] != nil {
		t.Fatal("atomic tensor quota", e)
	}
	if _, e := s.RadialTubeRegionMassTensor(ctx, center, radii, geom.RadialGraphMomentOptions{MaxOperations: tensor.Tensor.ExactOperations}); e != nil {
		t.Fatal("exact tensor quota", e)
	}
	if x, e := bad.RadialTubeRegionMassTensor(ctx, center, radii, geom.RadialGraphMomentOptions{}); e == nil || len(x.Components) != 0 {
		t.Fatal("wrong tensor owner side", e)
	}
	if x, e := s.RadialTubeRegionMassTensor(cancelled, center, radii, geom.RadialGraphMomentOptions{}); e != context.Canceled || len(x.Components) != 0 {
		t.Fatal("tensor cancellation", e)
	}
	t.Log("native tensor cumulative work", tensor.Tensor.ExactOperations)
	t.Log("native original height moment work", mass.Moments.ExactOperations)
	t.Log("native source moments total work", work)
}
