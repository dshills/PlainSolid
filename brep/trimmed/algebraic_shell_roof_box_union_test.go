package trimmed

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"math/big"
	"os"
	"os/exec"
	"testing"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

func TestOrderedRoofBoxFuseNative(t *testing.T) {
	ctx := context.Background()
	a, _ := makeOpenSplineBox(t, [3]float64{0, 0, -1}, [3]float64{1, 1, 1}, 0)
	b := weightedIdentityRoofStock(t, false, 2)
	var ref struct {
		Volume, Area  float64
		Centroid      [3]float64
		Solids, Euler int
	}
	if oracle := os.Getenv("PLAINSOLID_OCCT_ORDERED_ROOF_COMMON"); oracle != "" {
		data, err := exec.CommandContext(ctx, oracle, "weighted-ordered-union").Output()
		if err != nil || json.Unmarshal(data, &ref) != nil || ref.Solids != 1 || ref.Euler != 2 {
			t.Fatal("independent OCCT rational Fuse", ref, err, string(data))
		}
	}
	for i, src := range [][2]Solid{{a, b}, {b, a}} {
		out, h, err := FuseOrderedRoofBoxes(ctx, src[0], src[1], MixedRoofQueryOptions{}, geom.SurfacePatchEvaluationOptions{})
		if err != nil || len(h.Faces) != 6 || len(h.Faces[0]) != 1 || h.Faces[0][0] != (MixedRoofFaceOwner{1 - i, 1}) || len(h.Faces[1]) != 1 || h.Faces[1][0] != (MixedRoofFaceOwner{i, 4}) {
			t.Fatal("native roof and floor ancestry", out.Topology(), h, err)
		}
		for _, f := range h.Faces[2:] {
			if len(f) != 2 || f[0].Operand != 0 || f[1].Operand != 1 {
				t.Fatal("merged wall lost its original region ancestry", f)
			}
		}
		if topo := out.Topology(); len(topo.Shells) != 1 || topo.EulerCharacteristic != 2 || topo.Edges != 12 {
			t.Fatal("one closed canonical output band", topo)
		}
		roof := out.Faces()[0].Surface.(*geom.ExactBezierSurface)
		p, err := roof.ExactPatch().EvaluateRational(ctx, big.NewRat(1, 2), big.NewRat(1, 2), geom.SurfacePatchEvaluationOptions{})
		if err != nil || p.Point[2].Cmp(big.NewRat(3, 2)) != 0 {
			t.Fatal("exact original rational roof", p, err)
		}
		mass, err := out.MassProperties(ctx, parametric.IntegrationOptions{})
		if err != nil || math.Abs(mass.Volume-(3*math.Log(2)+.5)) > 1e-8 {
			t.Fatal("independent complete band integral", mass, err)
		}
		if ref.Solids != 0 && (math.Abs(mass.Volume-ref.Volume) > 1e-8 || math.Abs(mass.Area-ref.Area) > 1e-8 || mass.Centroid.Sub(geom.Vec3{X: ref.Centroid[0], Y: ref.Centroid[1], Z: ref.Centroid[2]}).Length() > 1e-8) {
			t.Fatal("rational Fuse differs from pinned OCCT", mass, ref)
		}
	}
	_, ledger, err := CombineSplineShells(ctx, a, b, parametric.Union, SplineShellCombineOptions{OrderedRoofFuse: &SplineShellRoofOptions{}})
	if err != nil || ledger.Method != "ordered-roof-fuse" || len(ledger.FacesB[1]) != 1 || ledger.FacesB[1][0] != 0 || len(ledger.FacesA[4]) != 1 || ledger.FacesA[4][0] != 1 {
		t.Fatal("optional native Fuse dispatch and ancestry", ledger, err)
	}
}

func TestOrderedRoofBoxFuseContactAndGap(t *testing.T) {
	ctx := context.Background()
	a := orderedRoofTangentSource(t)
	b, _ := makeOpenSplineBox(t, [3]float64{0, -.125, 1}, [3]float64{1, 1.125, 2}, 0)
	out, h, err := FuseOrderedRoofBoxes(ctx, a, b, MixedRoofQueryOptions{}, geom.SurfacePatchEvaluationOptions{})
	if err != nil || out.Topology().Faces != 6 || len(h.Faces) != 6 {
		t.Fatal("tangential roof to floor meeting does not leave a gap", out.Topology(), h, err)
	}
	mass, err := out.MassProperties(ctx, parametric.IntegrationOptions{})
	if err != nil || math.Abs(mass.Volume-3.125) > 1e-10 {
		t.Fatal("the internal contact must disappear in regularized Fuse", mass, err)
	}
	for _, tc := range []struct {
		min, max [3]float64
	}{
		{[3]float64{0, -.125, 1.0625}, [3]float64{1, 1.125, 2}}, // gap on a positive-area subregion
		{[3]float64{0, -.125, 0}, [3]float64{1, 1.125, 1.125}},  // nonrepresentable roof switches
		{[3]float64{0, -.125, 0}, [3]float64{2, 1.125, 2}},      // different footprints
	} {
		other, _ := makeOpenSplineBox(t, tc.min, tc.max, 0)
		if s, h, err := FuseOrderedRoofBoxes(ctx, a, other, MixedRoofQueryOptions{}, geom.SurfacePatchEvaluationOptions{}); !errors.Is(err, geom.ErrUnrepresentable) || s.Topology().Faces != 0 || len(h.Faces) != 0 {
			t.Fatal("unqualified single-band result escaped", tc, s.Topology(), h, err)
		}
	}
	if s, h, err := FuseOrderedRoofBoxes(ctx, a, b, MixedRoofQueryOptions{}, geom.SurfacePatchEvaluationOptions{MaxOperations: 1}); !errors.Is(err, geom.ErrConvergence) || s.Topology().Faces != 0 || len(h.Faces) != 0 {
		t.Fatal("budget exhaustion leaked topology", s.Topology(), h, err)
	}
}

func TestOrderedRoofBoxFuseImprintsIsoCrossings(t *testing.T) {
	ctx := context.Background()
	curved := orderedRoofTangentSource(t)
	flat, _ := makeOpenSplineBox(t, [3]float64{0, -.125, 0}, [3]float64{1, 1.125, 17. / 16}, 0)
	for i, pair := range [][2]Solid{{curved, flat}, {flat, curved}} {
		s, h, err := FuseOrderedRoofBoxes(ctx, pair[0], pair[1], MixedRoofQueryOptions{}, geom.SurfacePatchEvaluationOptions{})
		if err != nil || len(h.Faces) != 14 || s.Topology().EulerCharacteristic != 2 || len(s.Topology().Shells) != 1 {
			t.Fatal("two exact roof switches must retain a single solid", s.Topology(), h, err)
		}
		for j, resultFace := range []int{0, 5, 9} {
			owner := i
			if j == 1 {
				owner = 1 - i
			}
			if len(h.Faces[resultFace]) != 1 || h.Faces[resultFace][0].Operand != owner {
				t.Fatal("wrong source on the selected top fragment", j, h.Faces)
			}
		}
		if len(h.Faces[2]) != 2 || h.Faces[2][0].Operand != 0 || h.Faces[2][1].Operand != 1 {
			t.Fatal("merged wall must carry both original faces", h.Faces[2])
		}
		mass, err := s.MassProperties(ctx, parametric.IntegrationOptions{})
		// Subtract the exact Common from the two source volumes. The
		// parabolic stock includes an extra half unit below the flat floor.
		want := 1.25*(1.5+1./12+17./16) - 125./96
		if err != nil || math.Abs(mass.Volume-want) > 1e-12 {
			t.Fatal("independent maximum envelope integral", mass, err)
		}
	}
	creased := weightedIdentityRoofStockMode(t, false, 2, false, true)
	inferior, _ := makeOpenSplineBox(t, [3]float64{0, 0, -.5}, [3]float64{1, 1, 1}, 0)
	s, h, err := FuseOrderedRoofBoxes(ctx, creased, inferior, MixedRoofQueryOptions{}, geom.SurfacePatchEvaluationOptions{})
	if err != nil || len(h.Faces) != 10 || s.Topology().EulerCharacteristic != 2 || h.Faces[0][0] != (MixedRoofFaceOwner{0, 1}) || h.Faces[5][0] != (MixedRoofFaceOwner{0, 1}) {
		t.Fatal("a genuine crease retains its two distinct original fragments", s.Topology(), h, err)
	}
	flat, _ = makeOpenSplineBox(t, [3]float64{0, 0, 0}, [3]float64{1, 1, 2.25}, 0)
	s, h, err = FuseOrderedRoofBoxes(ctx, creased, flat, MixedRoofQueryOptions{}, geom.SurfacePatchEvaluationOptions{})
	if err != nil || len(h.Faces) != 18 || s.Topology().EulerCharacteristic != 2 || len(s.Topology().Shells) != 1 {
		t.Fatal("the original crease and both new intersections must be sewn", s.Topology(), h, err)
	}
	checkOrderedRoofCreaseCrossingMass(t, ctx, s, "piecewise-crossing-ordered-fuse", 37./16)
}

func TestOrderedRoofBoxFuseStrictlySeparateBands(t *testing.T) {
	ctx := context.Background()
	a := weightedIdentityRoofStock(t, false, 2)
	b, _ := makeOpenSplineBox(t, [3]float64{0, 0, 2.125}, [3]float64{1, 1, 3}, 0)
	for _, sources := range [][2]Solid{{a, b}, {b, a}} {
		s, h, err := FuseOrderedRoofBoxes(ctx, sources[0], sources[1], MixedRoofQueryOptions{}, geom.SurfacePatchEvaluationOptions{})
		if err != nil || len(s.Topology().Shells) != 2 || s.Topology().EulerCharacteristic != 4 || len(h.Faces) != 12 {
			t.Fatal("strict separation retains both original components", s.Topology(), h, err)
		}
		for i, owners := range h.Faces {
			if len(owners) != 1 || owners[0] != (MixedRoofFaceOwner{i / 6, i % 6}) || s.faces[i].Surface != sources[i/6].faces[i%6].Surface {
				t.Fatal("separate union must carry each original support and face unchanged", i, owners)
			}
		}
		mass, err := s.MassProperties(ctx, parametric.IntegrationOptions{})
		if err != nil || math.Abs(mass.Volume-(3*math.Log(2)+.375)) > 1e-8 {
			t.Fatal("independent two-band volume", mass, err)
		}
	}
	touch, _ := makeOpenSplineBox(t, [3]float64{0, 0, 2}, [3]float64{1, 1, 3}, 0)
	if s, h, err := FuseOrderedRoofBoxes(ctx, a, touch, MixedRoofQueryOptions{}, geom.SurfacePatchEvaluationOptions{}); !errors.Is(err, geom.ErrUnrepresentable) || s.Topology().Faces != 0 || len(h.Faces) != 0 {
		t.Fatal("edge contact cannot be promoted to strictly separate shell material", s.Topology(), h, err)
	}
}

func TestOrderedRoofBoxFuseOriginalFootprintContainment(t *testing.T) {
	ctx := context.Background()
	outer, inner := weightedIdentityRoofStock(t, false, 2), weightedIdentityRoofStock(t, true, 2)
	for i, sources := range [][2]Solid{{outer, inner}, {inner, outer}} {
		s, h, err := FuseOrderedRoofBoxes(ctx, sources[0], sources[1], MixedRoofQueryOptions{}, geom.SurfacePatchEvaluationOptions{})
		if err != nil || len(h.Faces) != 6 || s.Topology().EulerCharacteristic != 2 {
			t.Fatal("contained footprint and heights retain the entire original outer", s.Topology(), h, err)
		}
		for f, owners := range h.Faces {
			if len(owners) != 1 || owners[0] != (MixedRoofFaceOwner{i, f}) || s.faces[f].Surface != outer.faces[f].Surface {
				t.Fatal("original unmodified container ancestry", f, owners)
			}
		}
	}
	lateral, _ := makeOpenSplineBox(t, [3]float64{1.125, 0, 0}, [3]float64{2, 1, 1}, 0)
	if s, h, err := FuseOrderedRoofBoxes(ctx, outer, lateral, MixedRoofQueryOptions{}, geom.SurfacePatchEvaluationOptions{}); err != nil || len(s.Topology().Shells) != 2 || len(h.Faces) != 12 {
		t.Fatal("strict projected separation", s.Topology(), h, err)
	}
	partial, _ := makeOpenSplineBox(t, [3]float64{.5, 0, 0}, [3]float64{2, 1, 1}, 0)
	if s, h, err := FuseOrderedRoofBoxes(ctx, outer, partial, MixedRoofQueryOptions{}, geom.SurfacePatchEvaluationOptions{}); !errors.Is(err, geom.ErrUnrepresentable) || s.Topology().Faces != 0 || len(h.Faces) != 0 {
		t.Fatal("partially overlapping footprints need a different proof", s.Topology(), h, err)
	}
}

func TestOrderedRoofBoxFuseCreasedContainmentAndSeparation(t *testing.T) {
	ctx := context.Background()
	creased := weightedIdentityRoofStockMode(t, false, 2, false, true)
	inside, _ := makeOpenSplineBox(t, [3]float64{.25, .25, .5}, [3]float64{.75, .75, 2.25}, 0)
	high, _ := makeOpenSplineBox(t, [3]float64{0, 0, 2.75}, [3]float64{1, 1, 3.75}, 0)
	overlapHigh, _ := makeOpenSplineBox(t, [3]float64{.125, .25, 2.25}, [3]float64{.1875, 1.25, 3}, 0)
	for _, tc := range []struct {
		name   string
		other  Solid
		shells int
		volume float64
	}{
		{"contained", inside, 1, 2.25},
		{"full footprint separation", high, 2, 3.25},
		{"separation localized to one side of the crease", overlapHigh, 2, 2.25 + .75/16},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for i, sources := range [][2]Solid{{creased, tc.other}, {tc.other, creased}} {
				out, h, err := FuseOrderedRoofBoxes(ctx, sources[0], sources[1], MixedRoofQueryOptions{}, geom.SurfacePatchEvaluationOptions{})
				top := out.Topology()
				if err != nil || len(top.Shells) != tc.shells || top.EulerCharacteristic != 2*tc.shells || len(h.Faces) != 6*tc.shells {
					t.Fatal("retain full original crease supports", top, h, err)
				}
				for face, owners := range h.Faces {
					want := MixedRoofFaceOwner{face / 6, face % 6}
					if tc.shells == 1 {
						want.Operand = i
					}
					if len(owners) != 1 || owners[0] != want || out.faces[face].Surface != sources[want.Operand].faces[want.Face].Surface {
						t.Fatal("full original ancestry", h)
					}
				}
				mass, err := out.MassProperties(ctx, parametric.IntegrationOptions{})
				if err != nil || math.Abs(mass.Volume-tc.volume) > 1e-11 {
					t.Fatal("independent contained/separated volume", mass, tc.volume, err)
				}
			}
		})
	}
	touch, _ := makeOpenSplineBox(t, [3]float64{0, 0, 2.5}, [3]float64{1, 1, 3.75}, 0)
	if out, h, err := FuseOrderedRoofBoxes(ctx, creased, touch, MixedRoofQueryOptions{}, geom.SurfacePatchEvaluationOptions{}); !errors.Is(err, geom.ErrUnrepresentable) || out.Topology().Faces != 0 || len(h.Faces) != 0 {
		t.Fatal("crease contact must not be promoted to separate volume or a manifold merge", out.Topology(), h, err)
	}
}

func TestOrderedRoofBoxFuseOnlyOverlapSeparated(t *testing.T) {
	ctx := context.Background()
	low := orderedRoofTangentSource(t) // whole roof reaches 1.25
	high, _ := makeOpenSplineBox(t, [3]float64{.375, 0, 1.125}, [3]float64{.625, 1.25, 2}, 0)
	// The high body's footprint crosses the original Y boundary, and its
	// floor is below the low body's global roof maximum. On their actual
	// shared footprint the low roof never exceeds 1+1/64, proving a gap.
	s, h, err := FuseOrderedRoofBoxes(ctx, low, high, MixedRoofQueryOptions{}, geom.SurfacePatchEvaluationOptions{})
	if err != nil || len(s.Topology().Shells) != 2 || len(h.Faces) != 12 || s.Topology().EulerCharacteristic != 4 {
		t.Fatal("the exact overlap rather than the global height hull separates original shells", s.Topology(), h, err)
	}
	near, _ := makeOpenSplineBox(t, [3]float64{.375, 0, 1.015625}, [3]float64{.625, 1.25, 2}, 0)
	if s, h, err := FuseOrderedRoofBoxes(ctx, low, near, MixedRoofQueryOptions{}, geom.SurfacePatchEvaluationOptions{}); !errors.Is(err, geom.ErrUnrepresentable) || s.Topology().Faces != 0 || len(h.Faces) != 0 {
		t.Fatal("touching over the projected edge must not pass strict separation", s.Topology(), h, err)
	}
}

func TestOrderedRoofBoxFuseStackedPartialCap(t *testing.T) {
	ctx := context.Background()
	high := orderedRoofParabolaSource(t, 1, .5)
	// Move the lower stock's top to the original curved source's floor,
	// so the upper rectangle reaches past two ends of the shared cap.
	low, _ := makeOpenSplineBox(t, [3]float64{.125, .1875, -1.5}, [3]float64{.875, .8125, -.5}, 0)
	var ref struct {
		Volume, Area  float64
		Centroid      [3]float64
		Solids, Euler int
	}
	if oracle := os.Getenv("PLAINSOLID_OCCT_ORDERED_ROOF_COMMON"); oracle != "" {
		data, err := exec.CommandContext(ctx, oracle, "ordered-stacked-partial-cap").Output()
		if err != nil || json.Unmarshal(data, &ref) != nil || ref.Solids != 1 || ref.Euler != 2 {
			t.Fatal("independent OCCT partial planar cap", ref, err, string(data))
		}
	}
	for i, pair := range [][2]Solid{{low, high}, {high, low}} {
		s, h, err := FuseOrderedRoofBoxes(ctx, pair[0], pair[1], MixedRoofQueryOptions{}, geom.SurfacePatchEvaluationOptions{})
		if err != nil || len(s.Topology().Shells) != 1 || s.Topology().EulerCharacteristic != 2 || len(h.Faces) != s.Topology().Faces {
			t.Fatal("certified partial cap removes the interior interface", s.Topology(), h, err)
		}
		for _, f := range h.Faces {
			if len(f) != 1 || f[0] == (MixedRoofFaceOwner{i, 5}) {
				t.Fatal("consumed lower roof must have no retained fragment", f)
			}
		}
		mass, err := s.MassProperties(ctx, parametric.IntegrationOptions{})
		if err != nil || math.Abs(mass.Volume-(.75*.625+1.25*(1.5+1./12))) > 1e-9 {
			t.Fatal("analytic sum of source volumes with zero overlap", mass, err)
		}
		if ref.Solids != 0 && (math.Abs(mass.Volume-ref.Volume) > 1e-8 || math.Abs(mass.Area-ref.Area) > 1e-7 || mass.Centroid.Sub(geom.Vec3{X: ref.Centroid[0], Y: ref.Centroid[1], Z: ref.Centroid[2]}).Length() > 1e-8) {
			t.Fatal("stacked union differs from pinned OCCT", mass, ref)
		}
	}
}
