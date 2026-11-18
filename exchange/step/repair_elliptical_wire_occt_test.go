package step

import (
	"bytes"
	"context"
	"encoding/json"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"plainsolid/geom"
	"testing"
	"time"
)

func TestOCCTRepairRoundedEllipticalWire(t *testing.T) {
	t.Parallel()
	bin := os.Getenv("PLAINSOLID_OCCT_ELLIPSE_WIRE")
	if bin == "" {
		t.Skip("optional pinned OCCT")
	}
	raw := roundedEllipseFixture(t)
	repaired, e := RepairRoundedEllipticalWire(bytes.NewReader(raw), Limits{}, EllipticalWireRepairOptions{MaxDistance: 1e-11})
	m := repaired.Natural
	if e != nil {
		t.Fatal(e)
	}
	var out bytes.Buffer
	if e = WriteEllipticalWireSource(&out, m.Source, Limits{}); e != nil {
		t.Fatal(e)
	}
	for _, b := range [][]byte{raw, out.Bytes()} {
		path := filepath.Join(t.TempDir(), "ellipse.step")
		if e = os.WriteFile(path, b, 0600); e != nil {
			t.Fatal(e)
		}
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		r, e := exec.CommandContext(ctx, bin, "read", path).CombinedOutput()
		cancel()
		if e != nil {
			t.Fatal(e, string(r))
		}
		at := bytes.LastIndex(r, []byte(`{"Edges"`))
		if at < 0 {
			t.Fatal(string(r))
		}
		var q struct {
			Edges             int
			Length            float64
			Point, Derivative [3]float64
		}
		if e = json.Unmarshal(r[at:], &q); e != nil {
			t.Fatal(e)
		}
		p, _ := m.Geometry.Eval(.375)
		d, _ := m.Geometry.Derivative(.375)
		vec := func(a [3]float64) geom.Vec3 { return geom.Vec3{X: a[0], Y: a[1], Z: a[2]} }
		// Independent composite Simpson integral of the analytic speed, not a Go codec helper.
		const n = 16384
		h := 2 * math.Pi / n
		sum := 0.
		for i := 0; i <= n; i++ {
			u := float64(i) * h
			v := math.Hypot(4*math.Sin(u), 2*math.Cos(u))
			weight := 2.
			if i == 0 || i == n {
				weight = 1
			} else if i%2 == 1 {
				weight = 4
			}
			sum += weight * v
		}
		length := sum * h / 3
		if q.Edges != 1 || math.Abs(q.Length-length) > 1e-9 || vec(q.Point).Distance(p) > 1e-13 || vec(q.Derivative).Distance(d) > 1e-13 {
			t.Fatal(q, length, p, d)
		}
	}
}
