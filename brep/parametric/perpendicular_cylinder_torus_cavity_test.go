package parametric

import (
	"bytes"
	"context"
	"math"
	"os"
	"os/exec"
	"plainsolid/collision"
	"plainsolid/geom"
	"strconv"
	"strings"
	"testing"
)

func TestPerpendicularCylinderTorusCavity(t *testing.T) {
	ctx := context.Background()
	s, e := NewPerpendicularCylinderTorusCavitySet(ctx, 3, 1, 6, -5, 5, geom.Vec3{X: 1}, 1e-9)
	if e != nil {
		t.Fatal(e)
	}
	b, _ := s.Solid()
	owners, _ := s.SourceFaces()
	if len(b.faces) != 4 || !b.faces[3].Reversed || owners[0][0].Face != 3 {
		t.Fatal("cavity ownership")
	}
	p, e := b.MassProperties(ctx, IntegrationOptions{RelativeTolerance: 1e-9})
	v := 360*math.Pi - 6*math.Pi*math.Pi
	a := 192*math.Pi + 12*math.Pi*math.Pi
	if e != nil || math.Abs(p.Volume-v) > 1e-6 || math.Abs(p.Area-a) > 1e-6 || math.Abs(p.Centroid.Z) > 1e-8 {
		t.Fatal(p, e)
	}
	if math.Abs(p.Centroid.X+6*math.Pi*math.Pi/v) > 1e-8 || math.Abs(p.Centroid.Y) > 1e-8 {
		t.Fatal("shifted first moment", p)
	}
	if oracle := os.Getenv("PLAINSOLID_OCCT_PERPENDICULAR_CAVITY"); oracle != "" {
		raw, e := exec.Command(oracle).CombinedOutput()
		if e != nil {
			t.Fatal(e, string(raw))
		}
		fields := strings.Fields(string(raw))
		if len(fields) != 5 {
			t.Fatal(fields)
		}
		for i, want := range []float64{p.Volume, p.Area, p.Centroid.X, p.Centroid.Y, p.Centroid.Z} {
			got, e := strconv.ParseFloat(fields[i], 64)
			if e != nil || math.Abs(got-want) > 1e-6 {
				t.Fatal("independent", i, got, want, e)
			}
		}
	}
	var data bytes.Buffer
	if e = WriteJSON(&data, b); e != nil {
		t.Fatal(e)
	}
	reload, e := ReadJSON(bytes.NewReader(data.Bytes()), JSONLimits{})
	if e != nil {
		t.Fatal(e)
	}
	var data2 bytes.Buffer
	WriteJSON(&data2, reload)
	if !bytes.Equal(data.Bytes(), data2.Bytes()) {
		t.Fatal("reload")
	}
	mesh, e := reload.Tessellate(ctx, TessellationOptions{Deflection: .5, AnalyticBound: true})
	if e != nil {
		t.Fatal(e)
	}
	if e = mesh.ValidateSolid(); e != nil {
		t.Fatal(e)
	}
	contacts, e := collision.SelfIntersectionsBVHContext(ctx, mesh, collision.Options{})
	if e != nil || !contacts.Complete || len(contacts.Contacts) != 0 || mesh.SignedVolume() <= 0 {
		t.Fatal("contacts", e)
	}
}
