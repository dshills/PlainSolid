package occtcheck

import (
	"bufio"
	"context"
	"encoding/json"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"plainsolid/brep/parametric"
	"plainsolid/brep/trimmed"
	"plainsolid/csg/fillet"
	"plainsolid/geom"
)

func TestOCCTSelectiveUnequalFillet(t *testing.T) {
	program := os.Getenv("PLAINSOLID_OCCT_ORACLE")
	if program == "" {
		t.Skip("optional OCCT reference")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	path := filepath.Join(t.TempDir(), "fillet.jsonl")
	if out, e := exec.CommandContext(ctx, program, path, "selective-fillet").CombinedOutput(); e != nil {
		t.Fatal(e, string(out))
	}
	f, e := os.Open(path)
	if e != nil {
		t.Fatal(e)
	}
	defer f.Close()
	type result struct {
		record
		Faces, Edges, Vertices, Shells, Euler int
	}
	refs := map[string]result{}
	scan := bufio.NewScanner(f)
	for scan.Scan() {
		var r result
		if e = json.Unmarshal(scan.Bytes(), &r); e != nil {
			t.Fatal(e)
		}
		refs[r.Name] = r
	}
	if e = scan.Err(); e != nil {
		t.Fatal(e)
	}
	ref, ok := refs["selective-fillet"]
	top := refs["selective-fillet-topology"]
	if !ok || len(refs) != 2 || ref.Shells != 1 || top.Euler != 2 || top.Faces != 8 {
		t.Fatal(ref, top)
	}
	r, e := fillet.PrismCorners(ctx, []geom.Vec2{{}, {X: 4}, {X: 4, Y: 3}, {Y: 3}}, 0, 2, []fillet.CornerRadius{{Corner: 0, Radius: .25}, {Corner: 2, Radius: .5}}, fillet.Options{})
	if e != nil {
		t.Fatal(e)
	}
	p, e := r.Solid.MassProperties(ctx, parametric.IntegrationOptions{})
	if e != nil {
		t.Fatal(e)
	}
	area := 12 - (1-math.Pi/4)*(.25*.25+.5*.5)
	perimeter := 14 - (2-math.Pi/2)*(.25+.5)
	if math.Abs(ref.Volume-2*area) > 1e-8 || math.Abs(ref.Area-(2*area+2*perimeter)) > 1e-8 || math.Abs(p.Volume-ref.Volume) > 1e-8 || math.Abs(p.Area-ref.Area) > 1e-8 || p.Centroid.Sub(geom.Vec3{X: ref.Centroid[0], Y: ref.Centroid[1], Z: ref.Centroid[2]}).Length() > 1e-9 {
		t.Fatal("selective fillet differential", p, ref)
	}
	native := r.Solid.Topology()
	if native.Vertices != top.Vertices || native.Edges != top.Edges || native.Faces != top.Faces {
		t.Fatal("independent topology", native, top)
	}
}

func TestOCCTWrappedCircularRimFillet(t *testing.T) {
	ref := runOracle(t, "circular-rim-fillet")
	ctx := context.Background()
	o := geom.SurfacePatchOptions{}
	w, e := geom.NewCanonicalWrappedCylinder(ctx, geom.Vec3{}, 3, 5, o)
	if e != nil {
		t.Fatal(e)
	}
	stock, _, e := trimmed.NewWrappedCylinder(ctx, w, 1e-9, o)
	if e != nil {
		t.Fatal(e)
	}
	s, h, e := fillet.WrappedCylinderRims(ctx, stock, .5, 1, o)
	if e != nil || s.Topology().EulerCharacteristic != 2 || h.Blends[0] < 0 || h.Blends[1] < 0 {
		t.Fatal(h, e)
	}
	p, e := s.MassProperties(ctx, parametric.IntegrationOptions{RelativeTolerance: 1e-8})
	if e != nil {
		t.Fatal(e)
	}
	compare(t, ref["circular-rim-fillet"], p.Area, p.Volume, p.Centroid)
}
