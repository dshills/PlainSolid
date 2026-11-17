package offset

import (
	"context"
	"errors"
	"math"
	"math/big"
	"testing"

	"plainsolid/collision"
	"plainsolid/geom"
)

func TestTessellateCertifiedConvexFootprint(t *testing.T) {
	ctx := context.Background()
	frame, err := geom.NewFrame(geom.Vec3{X: .1, Y: -2.1, Z: 3.7}, geom.Vec3{Z: -1}, geom.Vec3{X: -1})
	if err != nil {
		t.Fatal(err)
	}
	shape, err := DilateConvexPrismFootprint(ctx, frame, []geom.Vec2{{}, {X: 4}, {Y: 3}}, -2, .625, 1e-9)
	if err != nil {
		t.Fatal(err)
	}
	coarse, err := shape.TessellateCertified(ctx, ConvexFootprintMeshOptions{Deflection: .04, NormalTangent: .4})
	if err != nil {
		t.Fatal(err)
	}
	// The public presentation data is independently mutable. All geometry and
	// face ancestry below must come from the original immutable certificate.
	shape.Primitives[0].Start.X = 100
	shape.Primitives[0].OriginalEdge = 123
	fine, err := shape.TessellateCertified(ctx, ConvexFootprintMeshOptions{Deflection: .002, NormalTangent: .1})
	if err != nil {
		t.Fatal(err)
	}
	n := len(fine.Rim)
	if n <= len(coarse.Rim) || fine.Distance.Cmp(num(.002)) > 0 || fine.Distance.Cmp(coarse.Distance) >= 0 ||
		fine.SquaredNormalTangent.Cmp(mul(num(.1), num(.1))) > 0 || !fine.Collision.Complete || len(fine.Collision.Contacts) != 0 ||
		len(fine.Mesh.Vertices) != 2*n+2 || len(fine.Mesh.Triangles) != 4*n || len(fine.OriginalFace) != 4*n {
		t.Fatal("invalid closed display certificate", n, fine.Distance, fine.SquaredNormalTangent, fine.Collision)
	}
	wantVolume := 27 + math.Pi*25/32
	if v := fine.Mesh.SignedVolume(); math.Abs(v-wantVolume) > .02 || math.Abs(v-wantVolume) >= math.Abs(coarse.Mesh.SignedVolume()-wantVolume) {
		t.Fatal("original rounded section volume did not converge", v, coarse.Mesh.SignedVolume(), wantVolume)
	}
	counts := make([]int, len(shape.source.primitives))
	for i, interval := range fine.Rim {
		source := shape.source.primitives[interval.Primitive]
		if interval.OriginalEdge != source.OriginalEdge || interval.OriginalVertex != source.OriginalVertex ||
			fine.OriginalFace[4*i] != 0 || fine.OriginalFace[4*i+1] != 1 || fine.OriginalFace[4*i+2] != interval.Primitive+2 ||
			fine.OriginalFace[4*i+3] != interval.Primitive+2 || interval.Lower.Cmp(interval.Upper) >= 0 {
			t.Fatal("original source ownership or interval order lost", i, interval)
		}
		counts[interval.Primitive]++
		prior := fine.Rim[(i+n-1)%n]
		if interval.Primitive == prior.Primitive && prior.Upper.Cmp(interval.Lower) != 0 ||
			interval.Primitive != prior.Primitive && (interval.Lower.Sign() != 0 || prior.Upper.Cmp(big.NewRat(1, 1)) != 0) {
			t.Fatal("original rim parameter gap", prior, interval)
		}
	}
	for i, primitive := range shape.source.primitives {
		if primitive.Radius == 0 && counts[i] != 1 || primitive.Radius > 0 && counts[i] <= 1 {
			t.Fatal("incorrect original line or curved subdivision", primitive, counts[i])
		}
	}
}

func TestTessellateCertifiedConvexFootprintAtomic(t *testing.T) {
	ctx := context.Background()
	frame, err := geom.NewFrame(geom.Vec3{}, geom.Vec3{X: 1}, geom.Vec3{Z: 1})
	if err != nil {
		t.Fatal(err)
	}
	triangle := []geom.Vec2{{}, {X: 4}, {Y: 4}}
	bare, err := DilateConvexPrismFootprint(ctx, frame, triangle, 2, 0, 1e-9)
	if err != nil {
		t.Fatal(err)
	}
	got, err := bare.TessellateCertified(ctx, ConvexFootprintMeshOptions{Deflection: 1e-8, MaxRimIntervals: 3})
	if err != nil || len(got.Rim) != 3 || got.SquaredNormalTangent.Sign() != 0 {
		t.Fatal("zero-radius prismatic mesh", got.Distance, got.SquaredNormalTangent, err)
	}
	// The rounded fan centers stay in their cap planes. Check the exact
	// enclosed volume without relying on floating-point determinant rounding.
	locator, err := collision.NewMeshPointLocator(ctx, got.Mesh, nil, collision.LocationOptions{})
	if err != nil {
		t.Fatal("zero-radius prismatic shell", err)
	}
	shells := locator.Shells()
	if len(shells) != 1 || shells[0].SignedVolume.Cmp(big.NewRat(16, 1)) != 0 {
		t.Fatal("zero-radius prismatic volume", shells)
	}
	triangle[2].Y = 3
	shape, err := DilateConvexPrismFootprint(ctx, frame, triangle, 2, .625, 1e-9)
	if err != nil {
		t.Fatal(err)
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	for _, test := range []struct {
		ctx     context.Context
		options ConvexFootprintMeshOptions
		want    error
	}{
		{ctx, ConvexFootprintMeshOptions{Deflection: .01, MaxRimIntervals: 6}, geom.ErrConvergence},
		{ctx, ConvexFootprintMeshOptions{Deflection: .01, Collision: collision.Options{MaxPairs: 1}}, collision.ErrLimit},
		{cancelled, ConvexFootprintMeshOptions{Deflection: .01}, context.Canceled},
		{ctx, ConvexFootprintMeshOptions{Deflection: math.NaN()}, geom.ErrInvalid},
	} {
		got, err := shape.TessellateCertified(test.ctx, test.options)
		if !errors.Is(err, test.want) || got.Distance != nil || len(got.Mesh.Vertices)+len(got.Rim) != 0 {
			t.Fatal("failed or cancelled display returned partial geometry", got, err, test.want)
		}
	}
}
