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
	"plainsolid/brep/trimmed"
	"plainsolid/geom"
	"testing"
	"time"
)

func TestOCCTPythagoreanEllipticalPrism(t *testing.T) {
	t.Parallel()
	bin := os.Getenv("PLAINSOLID_OCCT_PYTHAGOREAN_ELLIPTICAL_PRISM")
	if bin == "" {
		t.Skip("optional pinned OCCT")
	}
	raw := pythagoreanPrismFixture(t)
	m, e := ReadPythagoreanEllipticalPrism(context.Background(), bytes.NewReader(raw), Limits{}, 1e-9)
	if e != nil {
		t.Fatal(e)
	}
	var native bytes.Buffer
	if e = trimmed.WriteJSON(&native, m.Solid); e != nil {
		t.Fatal(e)
	}
	s, e := trimmed.ReadJSON(&native, trimmed.JSONLimits{})
	if e != nil {
		t.Fatal(e)
	}
	props, e := s.MassProperties(context.Background(), parametric.IntegrationOptions{RelativeTolerance: 1e-9})
	if e != nil {
		t.Fatal(e)
	}
	var authored, source bytes.Buffer
	if e = WritePythagoreanEllipticalPrismSTEP(context.Background(), &authored, PythagoreanEllipticalPrismDefinition{Name: "Native 3:4 prism", Origin: geom.Vec3{X: 1, Y: 2, Z: 3}, Reference: geom.Vec3{X: 3, Y: 4}, Axis: geom.Vec3{Z: 1}, MajorRadius: 10, MinorRadius: 5, Height: 3}, 1e-9); e != nil {
		t.Fatal(e)
	}
	if e = WritePythagoreanEllipticalPrismSource(context.Background(), &source, m.Source, Limits{}, 1e-9); e != nil {
		t.Fatal(e)
	}
	for _, raw := range [][]byte{raw, source.Bytes(), authored.Bytes()} {
		path := filepath.Join(t.TempDir(), "prism.step")
		if e = os.WriteFile(path, raw, 0600); e != nil {
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
			Faces, Solids               int
			SurfaceCenter               [3]float64
		}
		if e = json.Unmarshal(b[at:], &q); e != nil {
			t.Fatal(e)
		}
		center := geom.Vec3{X: q.SurfaceCenter[0], Y: q.SurfaceCenter[1], Z: q.SurfaceCenter[2]}
		if q.Faces != 3 || q.Solids != 1 || math.Abs(q.Area-props.Area) > 1e-6 || math.Abs(q.Volume-props.Volume) > 1e-6 || math.Abs(q.LibraryVolume-props.Volume) > 1e-6 || center.Distance(props.Centroid) > 1e-7 {
			t.Fatal(q, props)
		}
		t.Logf("native/OCCT V %.17g/%.17g A %.17g/%.17g", props.Volume, q.Volume, props.Area, q.Area)
	}
}
