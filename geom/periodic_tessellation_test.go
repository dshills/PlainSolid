package geom

import (
	"context"
	"errors"
	"math/big"
	"testing"
)

func TestPeriodicCertifiedTessellationWindow(t *testing.T) {
	poles := []Vec3{{1, 2, 0}, {3, -1, 1}, {4, 0, 3}, {-1, 3, -2}, {-2, 0, 1}}
	weights := []float64{1, .5, .25, .75, .125}
	curve, err := NewPeriodicBSpline(3, poles, weights, 1.25, 1.5)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	start, end := big.NewRat(11, 10), big.NewRat(57, 20)
	o := PeriodicTessellationOptions{CertifiedTessellationOptions: CertifiedTessellationOptions{Deflection: .02}}
	mesh, err := curve.TessellateCertifiedWindow(ctx, start, end, o)
	if err != nil || !mesh.Complete || len(mesh.Segments) <= 7 {
		t.Fatalf("received %d segments with complete=%t and error %v", len(mesh.Segments), mesh.Complete, err)
	}
	previous := start
	owners := 0
	width := big.NewRat(3, 10)
	for i, segment := range mesh.Segments {
		if previous.Cmp(segment.Domain.Lower) != 0 || segment.Domain.Lower.Cmp(segment.Domain.Upper) >= 0 || segment.ErrorUpper > o.Deflection {
			t.Fatalf("segment %d: %+v, previous endpoint %v", i, segment, previous)
		}
		previous = segment.Domain.Upper
		if i == 0 || mesh.Segments[i-1].Span != segment.Span {
			owners++
		}
		if i != 0 && mesh.Segments[i-1].End != segment.Start {
			t.Fatalf("rounded chord endpoint mismatch at %s", segment.Domain.Lower)
		}
		origin := new(big.Rat).Add(big.NewRat(5, 4), new(big.Rat).Mul(new(big.Rat).SetInt(segment.Cycle), big.NewRat(3, 2)))
		origin.Add(origin, new(big.Rat).Mul(width, big.NewRat(int64(segment.Span), 1)))
		cut := new(big.Rat).Add(origin, width)
		if segment.Domain.Lower.Cmp(origin) < 0 || segment.Domain.Upper.Cmp(cut) > 0 {
			t.Fatalf("chord crosses original span %d/%s: %+v", segment.Span, segment.Cycle, segment.Domain)
		}
		for _, station := range []*big.Rat{new(big.Rat), big.NewRat(1, 3), big.NewRat(2, 3), big.NewRat(1, 1)} {
			inverse := new(big.Rat).Sub(big.NewRat(1, 1), station)
			parameter := new(big.Rat).Add(new(big.Rat).Mul(segment.Domain.Lower, inverse), new(big.Rat).Mul(segment.Domain.Upper, station))
			local := new(big.Rat).Quo(new(big.Rat).Sub(parameter, origin), width)
			exact := evaluateCyclicCubicBasis(poles, weights, segment.Span, local)
			distance := new(big.Rat)
			for axis := 0; axis < 3; axis++ {
				chord := new(big.Rat).Mul(inverse, new(big.Rat).SetFloat64(coordinate(segment.Start, axis)))
				chord.Add(chord, new(big.Rat).Mul(station, new(big.Rat).SetFloat64(coordinate(segment.End, axis))))
				delta := new(big.Rat).Sub(new(big.Rat).Quo(exact[axis], exact[3]), chord)
				distance.Add(distance, new(big.Rat).Mul(delta, delta))
			}
			bound := new(big.Rat).SetFloat64(segment.ErrorUpper)
			if distance.Cmp(new(big.Rat).Mul(bound, bound)) > 0 {
				t.Fatalf("chord bound violated in segment %d at %s", i, parameter)
			}
		}
	}
	if previous.Cmp(end) != 0 || owners != 7 {
		t.Fatalf("last parameter %s, covered %d owners", previous, owners)
	}
	o.MaxOperations = mesh.ExactOperations
	full, err := curve.TessellateCertifiedWindow(ctx, start, end, o)
	if err != nil || !full.Complete || full.ExactOperations != mesh.ExactOperations {
		t.Fatalf("exact limit: complete=%t, work=%d, error=%v", full.Complete, full.ExactOperations, err)
	}
	o.MaxOperations--
	prefix, err := curve.TessellateCertifiedWindow(ctx, start, end, o)
	if !errors.Is(err, ErrConvergence) || prefix.Complete || len(prefix.Segments) != len(mesh.Segments)-1 || prefix.Segments[len(prefix.Segments)-1].Domain.Upper.Cmp(mesh.Segments[len(mesh.Segments)-1].Domain.Lower) != 0 {
		t.Fatalf("one operation short: complete=%t, segments=%d, error=%v", prefix.Complete, len(prefix.Segments), err)
	}
}
