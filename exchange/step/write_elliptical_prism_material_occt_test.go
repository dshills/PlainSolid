package step

import (
	"bytes"
	"context"
	"encoding/json"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"plainsolid/brep/parametric"
	"plainsolid/geom"
	"testing"
	"time"
)

func TestOCCTWriteEllipticalPrismMaterialSTEP(t *testing.T) {
	bin := os.Getenv("PLAINSOLID_OCCT_ELLIPTICAL_PRISM")
	if bin == "" {
		t.Skip("optional pinned OCCT")
	}
	m := nativeEllipsePrism(t)
	var native bytes.Buffer
	if e := WriteEllipticalPrismMaterialJSON(context.Background(), &native, m); e != nil {
		t.Fatal(e)
	}
	m, e := ReadEllipticalPrismMaterialJSON(context.Background(), &native, Limits{})
	if e != nil {
		t.Fatal(e)
	}
	p, e := m.Solid.MassProperties(context.Background(), parametric.IntegrationOptions{RelativeTolerance: 1e-9})
	if e != nil {
		t.Fatal(e)
	}
	for _, o := range []EllipticalPrismSTEPOptions{{}, {Name: "Translated native prism", Translation: geom.Vec3{X: 8, Y: -2, Z: 4}}} {
		var out bytes.Buffer
		if e = WriteEllipticalPrismMaterialSTEP(context.Background(), &out, m, o); e != nil {
			t.Fatal(e)
		}
		path := filepath.Join(t.TempDir(), "native.step")
		if e = os.WriteFile(path, out.Bytes(), 0600); e != nil {
			t.Fatal(e)
		}
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		b, e := exec.CommandContext(ctx, bin, "read", path).CombinedOutput()
		cancel()
		if e != nil {
			t.Fatal(e, string(b))
		}
		at := bytes.LastIndex(b, []byte(`{"Area"`))
		if at < 0 {
			t.Fatal(string(b))
		}
		var q struct {
			Area, Volume, LibraryVolume float64
			Faces, Solids, EdgeUses     int
			SurfaceCenter               [3]float64
		}
		if e = json.Unmarshal(b[at:], &q); e != nil {
			t.Fatal(e)
		}
		center := geom.Vec3{X: q.SurfaceCenter[0], Y: q.SurfaceCenter[1], Z: q.SurfaceCenter[2]}
		if q.Faces != 3 || q.Solids != 1 || q.EdgeUses != 6 || math.Abs(q.Area-p.Area) > 1e-7 || math.Abs(q.Volume-p.Volume) > 1e-7 || math.Abs(q.LibraryVolume-p.Volume) > 1e-7 || center.Distance(p.Centroid.Add(o.Translation)) > 1e-8 {
			t.Fatal(q, p)
		}
		t.Logf("native generated STEP: V %.17g A %.17g center %v", q.Volume, q.Area, q.SurfaceCenter)
	}
}
