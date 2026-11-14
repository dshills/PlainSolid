package trimmed

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"math"
	"os"
	"os/exec"
	"testing"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

func TestOrderedRoofBoxDifferenceRetainsOriginalCreases(t *testing.T) {
	ctx := context.Background()
	creased := weightedIdentityRoofStockMode(t, false, 2, false, true)
	flat := func(floor, roof float64) Solid {
		s, _ := makeOpenSplineBox(t, [3]float64{0, 0, floor}, [3]float64{1, 1, roof}, 0)
		return s
	}
	for _, tc := range []struct {
		name, oracle string
		stock, tool  Solid
		volume       float64
		shells       int
		cutterFaces  int
	}{
		{"creased stock above through tool", "piecewise-ordered-cut-upper", creased, flat(-1, 1.5), .75, 1, 2},
		{"creased floating cutter", "piecewise-ordered-cut-split", flat(-1, 4), creased, 2.75, 2, 4},
		{"tool above stock", "", creased, flat(3, 4), 2.25, 1, 0},
		{"stock covered", "", creased, flat(-1, 4), 0, 0, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			part, h, err := SubtractOrderedRoofBoxes(ctx, tc.stock, tc.tool, MixedRoofQueryOptions{}, geom.SurfacePatchEvaluationOptions{})
			top := part.Topology()
			if err != nil || len(top.Shells) != tc.shells || top.EulerCharacteristic != 2*tc.shells || len(h.Faces) != top.Faces || len(h.Reversed) != top.Faces {
				t.Fatal("incorrect retained band topology or face ledger", top, h, err)
			}
			polarity := 0
			for i, face := range h.Faces {
				if h.Reversed[i] != (face.Operand == 1) {
					t.Fatal("only original cutter faces reverse", i, h)
				}
				if h.Reversed[i] {
					polarity++
				}
			}
			if polarity != tc.cutterFaces {
				t.Fatal("incorrect number of retained reversed cutter fragments", polarity, h)
			}
			if tc.shells == 0 {
				return
			}
			if tc.cutterFaces == 0 {
				if top.Faces != len(tc.stock.Faces()) {
					t.Fatal("disjoint cutter must retain the original native faces", top)
				}
				return
			}
			if top.Faces != 10*tc.shells {
				t.Fatal("original knot may split caps and side walls without adding interior walls", top)
			}
			mass, err := part.MassProperties(ctx, parametric.IntegrationOptions{})
			if err != nil || math.Abs(mass.Volume-tc.volume) > 1e-12 {
				t.Fatal("independent piecewise band integral", tc.volume, mass, err)
			}
			if oracle := os.Getenv("PLAINSOLID_OCCT_ORDERED_ROOF_COMMON"); oracle != "" {
				data, err := exec.CommandContext(ctx, oracle, tc.oracle).Output()
				var ref struct {
					Volume, Area  float64
					Solids, Euler int
				}
				if err != nil || json.Unmarshal(data, &ref) != nil || ref.Solids != tc.shells || ref.Euler != 2*tc.shells || math.Abs(ref.Volume-mass.Volume) > 1e-8 || math.Abs(ref.Area-mass.Area) > 1e-8 {
					t.Fatal("pinned OCCT creased cut", mass, ref, err, string(data))
				}
			}
			var native bytes.Buffer
			if err := WriteJSON(&native, part); err != nil {
				t.Fatal(err)
			}
			if restored, err := ReadJSONContext(ctx, &native, JSONLimits{}); err != nil || restored.Topology().Faces != top.Faces {
				t.Fatal("native reload must validate the sewn knot edges", restored.Topology(), err)
			}
		})
	}
	part, h, err := SubtractOrderedRoofBoxes(ctx, creased, flat(-1, 2.25), MixedRoofQueryOptions{}, geom.SurfacePatchEvaluationOptions{})
	if !errors.Is(err, geom.ErrUnrepresentable) || part.Topology().Faces != 0 || len(h.Faces) != 0 {
		t.Fatal("a terminating band must require its own topology proof", part.Topology(), h, err)
	}
}

func TestOrderedRoofBoxDifferenceNative(t *testing.T) {
	ctx := context.Background()
	stock, _ := makeOpenSplineBox(t, [3]float64{}, [3]float64{1, 1, 4}, 0)
	cutter := weightedIdentityRoofStock(t, false, 2)
	part, h, err := SubtractOrderedRoofBoxes(ctx, stock, cutter, MixedRoofQueryOptions{}, geom.SurfacePatchEvaluationOptions{})
	if err != nil || part.Topology().Faces != 6 || part.Topology().EulerCharacteristic != 2 || len(h.Faces) != 6 || h.Faces[1] != (MixedRoofFaceOwner{1, 1}) || !h.Reversed[1] {
		t.Fatal("native roof-band difference and reversed original cutter", part.Topology(), h, err)
	}
	for i, owner := range h.Faces {
		if i != 1 && (h.Reversed[i] || owner.Operand != 0) {
			t.Fatal("incorrect unchanged stock side ledger", i, owner)
		}
	}
	mass, err := part.MassProperties(ctx, parametric.IntegrationOptions{})
	want := 4.5 - 3*math.Log(2)
	if err != nil || math.Abs(mass.Volume-want) > 1e-8 {
		t.Fatal("independent exact band integral", mass, want, err)
	}
	if oracle := os.Getenv("PLAINSOLID_OCCT_ORDERED_ROOF_COMMON"); oracle != "" {
		data, err := exec.CommandContext(ctx, oracle, "weighted-ordered-difference").Output()
		var ref struct {
			Volume, Area  float64
			Solids, Euler int
		}
		if err != nil || json.Unmarshal(data, &ref) != nil || ref.Solids != 1 || ref.Euler != 2 || math.Abs(ref.Volume-mass.Volume) > 1e-8 || math.Abs(ref.Area-mass.Area) > 1e-8 {
			t.Fatal("independent pinned OCCT weighted Boolean Cut", mass, ref, err, string(data))
		}
	}
	for _, tc := range []struct {
		max, min  float64
		wantFaces int
	}{{5, -1, 0}, {0, -2, 6}} {
		other, _ := makeOpenSplineBox(t, [3]float64{-1, -1, tc.min}, [3]float64{2, 2, tc.max}, 0)
		out, h, err := SubtractOrderedRoofBoxes(ctx, stock, other, MixedRoofQueryOptions{}, geom.SurfacePatchEvaluationOptions{})
		if err != nil || len(h.Faces) != tc.wantFaces || out.Topology().Faces != tc.wantFaces {
			t.Fatal("covered or boundary-only stock result", tc, out.Topology(), h, err)
		}
	}
	bad, _ := makeOpenSplineBox(t, [3]float64{.125, 0, 1}, [3]float64{1, 1, 2}, 0)
	if out, h, err := SubtractOrderedRoofBoxes(ctx, stock, bad, MixedRoofQueryOptions{}, geom.SurfacePatchEvaluationOptions{}); !errors.Is(err, geom.ErrUnrepresentable) || out.Topology().Faces != 0 || len(h.Faces) != 0 {
		t.Fatal("unqualified two-band result leaked output", out.Topology(), h, err)
	}
}

func TestOrderedRoofBoxDifferenceEnclosedCavity(t *testing.T) {
	ctx := context.Background()
	stock, cutter := weightedIdentityRoofStock(t, false, 2), weightedIdentityRoofStock(t, true, 2)
	s, h, err := SubtractOrderedRoofBoxes(ctx, stock, cutter, MixedRoofQueryOptions{}, geom.SurfacePatchEvaluationOptions{})
	if err != nil || len(h.Faces) != 12 || len(h.Reversed) != 12 || len(s.Topology().Shells) != 2 {
		t.Fatal("original stock plus inward closed cavity", s.Topology(), h, err)
	}
	for i, owner := range h.Faces {
		src := [2]Solid{stock, cutter}[i/6].faces[i%6]
		if owner != (MixedRoofFaceOwner{i / 6, i % 6}) || h.Reversed[i] != (i >= 6) || s.faces[i].Surface != src.Surface || (s.faces[i].Reversed != src.Reversed) != (i >= 6) {
			t.Fatal("original supports and inward polarity", i, owner)
		}
	}
	mass, err := s.MassProperties(ctx, parametric.IntegrationOptions{})
	// Cutter height is 7/8+X/4-1/4 on a square with side 3/4; the
	// linear height at its center is 3/4. Its whole volume is 27/64.
	want := 3*math.Log(2) - .5 - 27./64
	if err != nil || math.Abs(mass.Volume-want) > 1e-8 {
		t.Fatal("independent difference of the volumes", mass, want, err)
	}
	if oracle := os.Getenv("PLAINSOLID_OCCT_ORDERED_ROOF_COMMON"); oracle != "" {
		data, err := exec.CommandContext(ctx, oracle, "weighted-ordered-enclosed").Output()
		var ref struct {
			Volume, Area  float64
			Centroid      [3]float64
			Solids, Euler int
		}
		if err != nil || json.Unmarshal(data, &ref) != nil || ref.Solids != 1 || ref.Euler != 4 || math.Abs(ref.Volume-mass.Volume) > 1e-8 || math.Abs(ref.Area-mass.Area) > 1e-8 || mass.Centroid.Sub(geom.Vec3{X: ref.Centroid[0], Y: ref.Centroid[1], Z: ref.Centroid[2]}).Length() > 1e-8 {
			t.Fatal("independent pinned OCCT weighted cavity", mass, ref, err, string(data))
		}
	}
	touch, _ := makeOpenSplineBox(t, [3]float64{.125, .125, 0}, [3]float64{.875, .875, 1.25}, 0)
	if s, h, err := SubtractOrderedRoofBoxes(ctx, stock, touch, MixedRoofQueryOptions{}, geom.SurfacePatchEvaluationOptions{}); !errors.Is(err, geom.ErrUnrepresentable) || s.Topology().Faces != 0 || len(h.Faces) != 0 {
		t.Fatal("open cavity must not be represented as enclosed", s.Topology(), h, err)
	}
}

func TestOrderedRoofBoxDifferenceEnclosedCreasedCavity(t *testing.T) {
	ctx := context.Background()
	creased := weightedIdentityRoofStockMode(t, false, 2, false, true)
	inner, _ := makeOpenSplineBox(t, [3]float64{.25, .25, .5}, [3]float64{.75, .75, 1.5}, 0)
	outer, _ := makeOpenSplineBox(t, [3]float64{-.25, -.25, -.5}, [3]float64{1.25, 1.25, 3}, 0)
	for _, tc := range []struct {
		name, oracle  string
		stock, cutter Solid
		volume        float64
	}{
		{"cavity inside original creased stock", "piecewise-enclosed-stock", creased, inner, 2},
		{"original creased cavity", "piecewise-enclosed-cutter", outer, creased, 45. / 8},
	} {
		t.Run(tc.name, func(t *testing.T) {
			out, h, err := SubtractOrderedRoofBoxes(ctx, tc.stock, tc.cutter, MixedRoofQueryOptions{}, geom.SurfacePatchEvaluationOptions{})
			top := out.Topology()
			if err != nil || len(top.Shells) != 2 || top.EulerCharacteristic != 4 || top.Faces != 12 || len(h.Faces) != 12 {
				t.Fatal("keep both original shells and the creased support", top, h, err)
			}
			for i, face := range h.Faces {
				owner := [2]Solid{tc.stock, tc.cutter}[i/6].faces[i%6]
				if face != (MixedRoofFaceOwner{i / 6, i % 6}) || h.Reversed[i] != (i >= 6) || out.faces[i].Surface != owner.Surface {
					t.Fatal("incorrect original shell ancestry", i, h, face)
				}
			}
			mass, err := out.MassProperties(ctx, parametric.IntegrationOptions{})
			if err != nil || math.Abs(mass.Volume-tc.volume) > 1e-11 {
				t.Fatal("independent stock minus cavity volume", mass, tc.volume, err)
			}
			if oracle := os.Getenv("PLAINSOLID_OCCT_ORDERED_ROOF_COMMON"); oracle != "" {
				data, err := exec.CommandContext(ctx, oracle, tc.oracle).Output()
				var ref struct {
					Volume, Area  float64
					Solids, Euler int
				}
				if err != nil || json.Unmarshal(data, &ref) != nil || ref.Solids != 1 || ref.Euler != 4 || math.Abs(ref.Volume-mass.Volume) > 1e-8 || math.Abs(ref.Area-mass.Area) > 1e-8 {
					t.Fatal("pinned OCCT enclosed creased cut", mass, ref, err, string(data))
				}
			}
		})
	}
	touch, _ := makeOpenSplineBox(t, [3]float64{.25, .25, .5}, [3]float64{.75, .75, 2.25}, 0)
	if out, h, err := SubtractOrderedRoofBoxes(ctx, creased, touch, MixedRoofQueryOptions{}, geom.SurfacePatchEvaluationOptions{}); !errors.Is(err, geom.ErrUnrepresentable) || out.Topology().Faces != 0 || len(h.Faces) != 0 {
		t.Fatal("contact at the cavity boundary requires a separate topology proof", out.Topology(), h, err)
	}
}

func TestOrderedRoofBoxDifferenceFloatingCutter(t *testing.T) {
	ctx := context.Background()
	stock, _ := makeOpenSplineBox(t, [3]float64{0, 0, -1}, [3]float64{1, 1, 4}, 0)
	cutter := weightedIdentityRoofStock(t, false, 2)
	out, h, err := SubtractOrderedRoofBoxes(ctx, stock, cutter, MixedRoofQueryOptions{}, geom.SurfacePatchEvaluationOptions{})
	if err != nil || len(h.Faces) != 12 || len(h.Reversed) != 12 || h.Faces[0] != (MixedRoofFaceOwner{1, 0}) || h.Faces[7] != (MixedRoofFaceOwner{1, 1}) {
		t.Fatal("original cutter floor and roof ownership", h, err)
	}
	for i, reversed := range h.Reversed {
		if reversed != (i == 0 || i == 7) {
			t.Fatal("only the two retained cutter caps reverse", h)
		}
	}
	if top := out.Topology(); len(top.Shells) != 2 || top.Faces != 12 || top.Edges != 24 || top.EulerCharacteristic != 4 {
		t.Fatal("separate closed components on either side of the cutter", top)
	}
	mass, err := out.MassProperties(ctx, parametric.IntegrationOptions{})
	want := 5.5 - 3*math.Log(2)
	if err != nil || math.Abs(mass.Volume-want) > 1e-8 {
		t.Fatal("independent volume of both retained bands", mass, want, err)
	}
	if oracle := os.Getenv("PLAINSOLID_OCCT_ORDERED_ROOF_COMMON"); oracle != "" {
		data, err := exec.CommandContext(ctx, oracle, "weighted-ordered-split").Output()
		var ref struct {
			Volume, Area  float64
			Centroid      [3]float64
			Solids, Euler int
		}
		if err != nil || json.Unmarshal(data, &ref) != nil || ref.Solids != 2 || ref.Euler != 4 || math.Abs(ref.Volume-mass.Volume) > 1e-8 || math.Abs(ref.Area-mass.Area) > 1e-8 || mass.Centroid.Sub(geom.Vec3{X: ref.Centroid[0], Y: ref.Centroid[1], Z: ref.Centroid[2]}).Length() > 1e-8 {
			t.Fatal("independent pinned OCCT weighted split", mass, ref, err, string(data))
		}
	}
	_, ledger, err := CombineSplineShells(ctx, stock, cutter, parametric.Difference, SplineShellCombineOptions{OrderedRoofCut: &SplineShellRoofOptions{}})
	if err != nil || ledger.Method != "ordered-roof-cut" || len(ledger.FacesA[0]) != 2 || len(ledger.FacesB[0]) != 1 || ledger.FacesB[0][0] != 0 || len(ledger.FacesB[1]) != 1 || ledger.FacesB[1][0] != 7 || len(ledger.ReversedFaces) != 2 || ledger.ReversedFaces[0] != 0 || ledger.ReversedFaces[1] != 7 {
		t.Fatal("split ancestry survives opt-in dispatcher", ledger, err)
	}
	if s, h, err := SubtractOrderedRoofBoxes(ctx, stock, cutter, MixedRoofQueryOptions{}, geom.SurfacePatchEvaluationOptions{MaxOperations: 1}); !errors.Is(err, geom.ErrConvergence) || s.Topology().Faces != 0 || len(h.Faces) != 0 {
		t.Fatal("failure never returns half of the split", s.Topology(), h, err)
	}
}

func TestOrderedRoofBoxDifferenceTopRemoval(t *testing.T) {
	ctx := context.Background()
	stock := weightedIdentityRoofStock(t, false, 2)
	cut, _ := makeOpenSplineBox(t, [3]float64{-1, -1, .5}, [3]float64{2, 2, 3}, 0)
	out, h, err := SubtractOrderedRoofBoxes(ctx, stock, cut, MixedRoofQueryOptions{}, geom.SurfacePatchEvaluationOptions{})
	if err != nil || out.Topology().Faces != 6 || out.Topology().EulerCharacteristic != 2 || len(h.Faces) != 6 || h.Faces[0] != (MixedRoofFaceOwner{1, 4}) || !h.Reversed[0] {
		t.Fatal("upper cut leaves the original cutter floor as the reversed roof", out.Topology(), h, err)
	}
	mass, err := out.MassProperties(ctx, parametric.IntegrationOptions{})
	if err != nil || math.Abs(mass.Volume-.5) > 1e-12 || math.Abs(mass.Area-4) > 1e-12 {
		t.Fatal("independent remaining unit-footprint prism", mass, err)
	}
	if oracle := os.Getenv("PLAINSOLID_OCCT_ORDERED_ROOF_COMMON"); oracle != "" {
		data, err := exec.CommandContext(ctx, oracle, "weighted-ordered-top-removal").Output()
		var ref struct {
			Volume, Area  float64
			Solids, Euler int
		}
		if err != nil || json.Unmarshal(data, &ref) != nil || ref.Solids != 1 || ref.Euler != 2 || math.Abs(ref.Volume-mass.Volume) > 1e-8 || math.Abs(ref.Area-mass.Area) > 1e-8 {
			t.Fatal("independent pinned OCCT top removal", mass, ref, err, string(data))
		}
	}
	above, _ := makeOpenSplineBox(t, [3]float64{0, 0, 2}, [3]float64{1, 1, 3}, 0)
	unchanged, h, err := SubtractOrderedRoofBoxes(ctx, stock, above, MixedRoofQueryOptions{}, geom.SurfacePatchEvaluationOptions{})
	if err != nil || len(h.Faces) != 6 || unchanged.faces[1].Surface != stock.faces[1].Surface {
		t.Fatal("a cutter touching only the stock roof retains original stock", h, err)
	}
	// The cutter floor crosses inside the footprint; a constant upper
	// plane cannot represent this result without imprint and trim.
	partial, _ := makeOpenSplineBox(t, [3]float64{0, 0, 1.5625}, [3]float64{1, 1, 3}, 0)
	if out, h, err := SubtractOrderedRoofBoxes(ctx, stock, partial, MixedRoofQueryOptions{}, geom.SurfacePatchEvaluationOptions{}); err == nil || out.Topology().Faces != 0 || len(h.Faces) != 0 {
		t.Fatal("crossing cutter floor remains bounded and atomic", out.Topology(), h, err)
	}
}
