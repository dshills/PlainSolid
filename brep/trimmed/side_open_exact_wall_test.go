package trimmed

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"math"
	"math/big"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"plainsolid/brep/edgecheck"
	"plainsolid/brep/parametric"
	"plainsolid/brep/trimvalidate"
	"plainsolid/geom"
)

func TestGeneralSideOpenNonbinaryWallsSewAndPersist(t *testing.T) {
	ctx := context.Background()
	frame, err := geom.NewFrame(geom.Vec3{}, geom.Vec3{X: 1}, geom.Vec3{Z: 1})
	if err != nil {
		t.Fatal(err)
	}
	discs := [2]ExtrusionCircle{{Center: geom.Vec2{X: 20}, Radius: 13}, {Center: geom.Vec2{X: 24}, Radius: 15}}
	min, max := geom.Vec2{}, geom.Vec2{X: 50, Y: 22}
	for _, depth := range []float64{3, 6} {
		if _, e := CutGeneralSideOpenDualDisks(ctx, frame, min, max, discs, 6, depth, 1e-10, trimvalidate.Options{}); !errors.Is(e, geom.ErrUnrepresentable) {
			t.Fatal("ordinary constructor silently approximated the nonbinary wall", e)
		}
		cut, e := CutGeneralSideOpenDualDisksWithExactWalls(ctx, frame, min, max, discs, 6, depth, 1e-10, trimvalidate.Options{})
		if e != nil {
			t.Fatal("exact-wall stock", depth, e)
		}
		proof, e := cut.Solid.ValidateEdgeAgreement(ctx, edgecheck.Options{})
		if e != nil || !proof.Complete || len(cut.Solid.Topology().Shells) != 1 {
			t.Fatal("a closed solid needs continuous proof on every original use", proof.Complete, e)
		}
		var native bytes.Buffer
		if e := WriteJSON(&native, cut.Solid); e != nil {
			t.Fatal(e)
		}
		if !bytes.Contains(native.Bytes(), []byte("26/3")) {
			t.Fatal("nonbinary coefficient missing from native solid")
		}
		raw := append([]byte(nil), native.Bytes()...)
		if !bytes.Contains(raw, []byte(`"version":7`)) {
			t.Fatal("exact surface support must select native version 7")
		}
		downgraded := bytes.Replace(raw, []byte(`"version":7`), []byte(`"version":1`), 1)
		if s, e := ReadJSONContext(ctx, bytes.NewReader(downgraded), JSONLimits{}); e == nil || len(s.Faces()) != 0 {
			t.Fatal("older native version must not bypass exact support proof", e)
		}
		reloaded, e := ReadJSONContext(ctx, &native, JSONLimits{})
		if e != nil {
			t.Fatal("native continuous edge reproof", e)
		}
		for i, ids := range cut.WallFaces {
			id := ids[0]
			original, ok := cut.Solid.Faces()[id].Surface.(*geom.ExactBezierSurface)
			restored, ok2 := reloaded.Faces()[id].Surface.(*geom.ExactBezierSurface)
			if !ok || !ok2 {
				t.Fatal("wall did not retain exact surface kind", id)
			}
			for _, u := range []*big.Rat{big.NewRat(1, 3), big.NewRat(6, 7)} {
				a, e := original.ExactPatch().EvaluateRational(ctx, u, big.NewRat(1, 3), geom.SurfacePatchEvaluationOptions{})
				b, e2 := restored.ExactPatch().EvaluateRational(ctx, u, big.NewRat(1, 3), geom.SurfacePatchEvaluationOptions{})
				if e != nil || e2 != nil || !sameRationalVertex(a.Point, b.Point) || !sameRationalVertex(a.Du, b.Du) || !sameRationalVertex(a.Dv, b.Dv) {
					t.Fatal("wall point or derivative changed after reload", i, e, e2)
				}
				x := new(big.Rat).Sub(b.Point[0], ratInset(discs[i].Center.X))
				y := b.Point[1]
				if new(big.Rat).Add(new(big.Rat).Mul(x, x), new(big.Rat).Mul(y, y)).Cmp(ratInset(discs[i].Radius*discs[i].Radius)) != 0 {
					t.Fatal("reloaded chart left original circle")
				}
			}
		}
		bad := bytes.Replace(raw, []byte(`"26/3"`), []byte(`"26000000001/3000000000"`), 1)
		if s, e := ReadJSONContext(ctx, bytes.NewReader(bad), JSONLimits{}); e == nil || len(s.Faces()) != 0 {
			t.Fatal("small exact support edit must fail instead of accepting sample tolerance", e)
		}
		measured, e := reloaded.MassProperties(ctx, parametric.IntegrationOptions{RelativeTolerance: 1e-7})
		formula, e2 := cut.Measures()
		if e != nil || e2 != nil || math.Abs(measured.Volume-formula.Volume) > 1e-4 || math.Abs(measured.Area-formula.Area) > 1e-4 || measured.Centroid.Sub(formula.Centroid).Length() > 1e-6 {
			t.Fatal("native reloaded integration disagrees with analytic material", measured, formula, e, e2)
		}
		if oracle := os.Getenv("PLAINSOLID_OCCT_EXACT_SIDE_WALLS"); oracle != "" {
			mode := "exact-wall-side"
			if depth == 6 {
				mode = "exact-wall-through"
			}
			deadline, cancel := context.WithTimeout(ctx, 30*time.Second)
			log, e := exec.CommandContext(deadline, oracle, mode, filepath.Join(t.TempDir(), "oracle.step")).CombinedOutput()
			cancel()
			start := bytes.LastIndex(log, []byte("\n{"))
			if e != nil || start < 0 {
				t.Fatal("OCCT independent Boolean", e, string(log))
			}
			var reference struct {
				Volume, Area float64
				Center       [3]float64
			}
			if e = json.Unmarshal(log[start+1:], &reference); e != nil {
				t.Fatal(e)
			}
			center := geom.Vec3{X: reference.Center[0], Y: reference.Center[1], Z: 6 - reference.Center[2]}
			if math.Abs(formula.Volume-reference.Volume) > 1e-7 || math.Abs(formula.Area-reference.Area) > 1e-7 || formula.Centroid.Sub(center).Length() > 1e-8 {
				t.Fatal("exact-wall BRep vs pinned OCCT Boolean", depth, formula, reference)
			}
			t.Logf("depth %g: OCCT V %.12g A %.12g", depth, reference.Volume, reference.Area)
		}
	}
}
