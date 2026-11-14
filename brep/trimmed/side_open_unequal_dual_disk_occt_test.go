package trimmed

import (
	"bytes"
	"context"
	"encoding/json"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"plainsolid/brep/parametric"
	"plainsolid/brep/trimvalidate"
	"plainsolid/geom"
)

func TestUnequalSideOpenDualDiskAgainstOCCT(t *testing.T) {
	bin := os.Getenv("PLAINSOLID_OCCT_OVERLAPPING_SIDE_POCKETS")
	if bin == "" {
		t.Skip("optional independent OCCT overlap oracle")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	ref, e := exec.CommandContext(ctx, bin, "unequal-side", filepath.Join(t.TempDir(), "side.step")).CombinedOutput()
	if e != nil {
		t.Fatal(e, string(ref))
	}
	start := bytes.LastIndex(ref, []byte("\n{"))
	if start < 0 {
		t.Fatal(string(ref))
	}
	var want struct {
		Volume, Area float64
		Center       [3]float64
	}
	if e = json.Unmarshal(ref[start+1:], &want); e != nil {
		t.Fatal(e)
	}
	// The oracle enters through the opposite cap. Reflect its centroid
	// in the stock midplane for this comparison; area and volume are unchanged.
	f, e := geom.NewFrame(geom.Vec3{}, geom.Vec3{X: 1}, geom.Vec3{Z: 1})
	if e != nil {
		t.Fatal(e)
	}
	discs := [2]ExtrusionCircle{{Center: geom.Vec2{X: 20}, Radius: 13}, {Center: geom.Vec2{X: 34}, Radius: 15}}
	r, e := CutUnequalSideOpenDualDisks(ctx, f, geom.Vec2{}, geom.Vec2{X: 60, Y: 22}, discs, 6, 3, 1e-10, trimvalidate.Options{})
	if e != nil {
		t.Fatal(e)
	}
	got, e := r.Solid.MassProperties(ctx, parametric.IntegrationOptions{RelativeTolerance: 1e-7})
	if e != nil {
		t.Fatal(e)
	}
	center := geom.Vec3{X: want.Center[0], Y: want.Center[1], Z: 6 - want.Center[2]}
	if math.Abs(got.Volume-want.Volume) > 1e-4 || math.Abs(got.Area-want.Area) > 1e-4 || got.Centroid.Sub(center).Length() > 1e-6 {
		t.Fatal("Go exact circular chart vs OCCT solid", got, want)
	}
	analytic, e := r.Measures()
	if e != nil || math.Abs(analytic.Volume-want.Volume) > 1e-7 || math.Abs(analytic.Area-want.Area) > 1e-7 || analytic.Centroid.Sub(center).Length() > 1e-8 {
		t.Fatal("independent unequal moments", analytic, want, e)
	}

}
