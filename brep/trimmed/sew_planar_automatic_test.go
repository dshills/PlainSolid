//go:build research

package trimmed

import (
	"context"
	"errors"
	"math/big"
	"plainsolid/brep"
	"plainsolid/geom"
	"strings"
	"testing"
)

func TestSewPlanarAutomaticallyWithinTolerance(t *testing.T) {
	ctx := context.Background()
	box, err := brep.Box(geom.Vec3{}, geom.Vec3{X: 3, Y: 2, Z: 1})
	if err != nil {
		t.Fatal(err)
	}
	planar, err := FromPlanar(box, 1e-9)
	if err != nil {
		t.Fatal(err)
	}
	faces, authored := planarSewToleranceFixture(t, planar)
	const shift = 1.0 / 1024
	for i := range faces[1].Outer {
		faces[1].Outer[i].Y += shift
	}
	base := PlanarToleranceSewOptions{Tolerance: shift}
	got, report, err := SewPlanarAutomaticallyWithinTolerance(ctx, faces, AutomaticPlanarSewOptions{PlanarToleranceSewOptions: base})
	if err != nil {
		t.Fatal(err)
	}
	if got.Topology().EulerCharacteristic != 2 || len(report.Seams) != len(authored) || report.PairTests <= 0 ||
		report.MovedVertices != 4 || report.MaxSquaredDisplacement.Cmp(big.NewRat(1, 1048576)) != 0 || report.ExactVolume.Cmp(big.NewRat(6, 1)) != 0 {
		t.Fatal("automatic seams failed to reproduce the original closed box", got.Topology(), report)
	}
	for _, got := range report.Seams {
		found := false
		for _, expected := range authored {
			if got.Reversed == expected.Reversed && (got.First == expected.First && got.Second == expected.Second || got.First == expected.Second && got.Second == expected.First) {
				found = true
			}
		}
		if !found {
			t.Fatal("geometric discovery paired different original use IDs", got)
		}
	}
	if _, _, err := SewPlanarAutomaticallyWithinTolerance(ctx, faces, AutomaticPlanarSewOptions{base, report.PairTests}); err != nil {
		t.Fatal("reported pair quota did not replay", err)
	}
	for _, test := range []struct {
		options AutomaticPlanarSewOptions
		reason  string
	}{
		{AutomaticPlanarSewOptions{base, report.PairTests - 1}, ""},
		{AutomaticPlanarSewOptions{PlanarToleranceSewOptions: PlanarToleranceSewOptions{Tolerance: shift / 2}}, "exceeds tolerance"},
		{AutomaticPlanarSewOptions{PlanarToleranceSewOptions: PlanarToleranceSewOptions{Tolerance: shift / 4}}, "no planar border match"},
		{AutomaticPlanarSewOptions{PlanarToleranceSewOptions: PlanarToleranceSewOptions{Tolerance: 4}}, "ambiguous planar border pair"},
	} {
		bad, partial, err := SewPlanarAutomaticallyWithinTolerance(ctx, faces, test.options)
		if !errors.Is(err, geom.ErrConvergence) || !strings.Contains(err.Error(), test.reason) || len(bad.faces)+len(partial.Seams)+len(partial.Vertices) != 0 {
			t.Fatal("automatic sewing exposed incomplete or ambiguous topology", test.reason, bad.Topology(), partial.PairTests, err)
		}
	}
}
