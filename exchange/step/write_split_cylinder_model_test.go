package step

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"plainsolid/geom"
	"reflect"
	"testing"
	"time"
)

func splitWriterFixture(t *testing.T) SplitCylinderDescriptor {
	t.Helper()
	f, e := geom.NewFrame(geom.Vec3{X: 10, Y: -4, Z: 7}, geom.Vec3{Z: -1}, geom.Vec3{X: -1})
	if e != nil {
		t.Fatal(e)
	}
	return SplitCylinderDescriptor{Name: "split 'posed'", Frame: f, Radius: 2, Height: 3, LengthUnit: .001}
}
func TestWriteSplitCylinderModel(t *testing.T) {
	d := splitWriterFixture(t)
	for _, unit := range []float64{.001, 1} {
		d.LengthUnit = unit
		var b bytes.Buffer
		if e := WriteSplitCylinderModel(&b, d); e != nil {
			t.Fatal(e)
		}
		raw := append([]byte(nil), b.Bytes()...)
		m, e := ReadSplitCylinderModel(&b, Limits{})
		if e != nil || m.Name != d.Name || m.Frame != d.Frame || m.Radius != d.Radius || m.Height != d.Height || m.LengthUnit != unit {
			t.Fatal(m, e)
		}
		if e = WriteSplitCylinderModel(&b, d); e != nil || !bytes.Equal(raw, b.Bytes()) {
			t.Fatal("unstable", e)
		}
		n, e := ReadSplitCylinderModel(&b, Limits{})
		if e != nil || !reflect.DeepEqual(n.Faces, m.Faces) || !reflect.DeepEqual(n.Edges, m.Edges) {
			t.Fatal("ancestry", e)
		}
	}
}

type splitShortWriter struct{}

func (splitShortWriter) Write(b []byte) (int, error) { return len(b) - 1, nil }
func TestWriteSplitCylinderModelReject(t *testing.T) {
	good := splitWriterFixture(t)
	bad := good
	bad.Radius = -1
	cases := []SplitCylinderDescriptor{bad}
	bad = good
	bad.Frame = geom.Frame{}
	cases = append(cases, bad)
	bad = good
	bad.LengthUnit = .3048
	cases = append(cases, bad)
	bad = good
	bad.Frame, _ = geom.NewFrame(geom.Vec3{X: math.Ldexp(1, -60)}, geom.Vec3{X: 1}, geom.Vec3{Z: 1})
	cases = append(cases, bad)
	for _, d := range cases {
		var b bytes.Buffer
		if e := WriteSplitCylinderModel(&b, d); e == nil || b.Len() != 0 {
			t.Fatal("accepted invalid", d, e)
		}
	}
	if e := WriteSplitCylinderModel(splitShortWriter{}, good); !errors.Is(e, io.ErrShortWrite) {
		t.Fatal(e)
	}
}
func TestWriteSplitCylinderModelOCCT(t *testing.T) {
	bin := os.Getenv("PLAINSOLID_OCCT_SPLIT_CYLINDER")
	if bin == "" {
		t.Skip("optional independent helper")
	}
	d := splitWriterFixture(t)
	var b bytes.Buffer
	if e := WriteSplitCylinderModel(&b, d); e != nil {
		t.Fatal(e)
	}
	p := filepath.Join(t.TempDir(), "model.step")
	if e := os.WriteFile(p, b.Bytes(), 0600); e != nil {
		t.Fatal(e)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	raw, e := exec.CommandContext(ctx, bin, p).CombinedOutput()
	if e != nil {
		t.Fatal(e, string(raw))
	}
	var v struct {
		Faces        int
		Volume, Area float64
		Center       [3]float64
	}
	if e = json.Unmarshal(raw, &v); e != nil {
		t.Fatal(e, string(raw))
	}
	if v.Faces != 4 || math.Abs(v.Volume-12*math.Pi) > 1e-10 || math.Abs(v.Area-20*math.Pi) > 1e-10 {
		t.Fatal(v)
	}
	for i, x := range []float64{8.5, -4, 7} {
		if math.Abs(v.Center[i]-x) > 1e-10 {
			t.Fatal(v)
		}
	}
}

func TestWriteSplitCylinderExactFractionalDimensions(t *testing.T) {
	d := splitWriterFixture(t)
	d.Radius = 2.125
	d.Height = 3.25
	var b bytes.Buffer
	if e := WriteSplitCylinderModel(&b, d); e != nil {
		t.Fatal(e)
	}
	m, e := ReadSplitCylinderModel(&b, Limits{})
	if e != nil || m.Radius != d.Radius || m.Height != d.Height || m.Frame != d.Frame {
		t.Fatal(m, e)
	}
}
