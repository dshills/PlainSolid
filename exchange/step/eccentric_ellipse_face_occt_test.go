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

func TestOCCTEccentricEllipticalFace(t *testing.T) {
	t.Parallel()
	bin := os.Getenv("PLAINSOLID_OCCT_ECCENTRIC_ELLIPSE_FACE")
	if bin == "" {
		t.Skip("optional pinned OCCT")
	}
	raw := eccentricEllipseFixture(t)
	m, e := ReadEccentricEllipticalFace(bytes.NewReader(raw), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	var native bytes.Buffer
	if e = WriteEccentricEllipticalFaceJSON(&native, m); e != nil {
		t.Fatal(e)
	}
	m, e = ReadEccentricEllipticalFaceJSON(&native, Limits{})
	if e != nil {
		t.Fatal(e)
	}

	var out bytes.Buffer
	if e = WriteEccentricEllipticalFaceSource(&out, m.Source, Limits{}); e != nil {
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
		at := bytes.LastIndex(r, []byte(`{"Area"`))
		if at < 0 {
			t.Fatal(string(r))
		}
		var q struct {
			Faces             int
			Normal            [3]float64
			Area              float64
			Center            [3]float64
			Edges             int
			Length            float64
			Point, Derivative [3]float64
		}
		if e = json.Unmarshal(r[at:], &q); e != nil {
			t.Fatal(e)
		}
		p, _ := m.InnerBoundary.Eval(.375)
		d, _ := m.InnerBoundary.Derivative(.375)
		vec := func(a [3]float64) geom.Vec3 { return geom.Vec3{X: a[0], Y: a[1], Z: a[2]} }
		// Independent composite Simpson integral of the analytic speed, not a Go codec helper.
		const n = 16384
		h := 2 * math.Pi / n
		sum := 0.
		for i := 0; i <= n; i++ {
			u := float64(i) * h
			v := math.Hypot(8*math.Sin(u), 5*math.Cos(u)) + math.Hypot(2*math.Sin(u), math.Cos(u))
			weight := 2.
			if i == 0 || i == n {
				weight = 1
			} else if i%2 == 1 {
				weight = 4
			}
			sum += weight * v
		}
		length := sum * h / 3
		if q.Faces != 1 || vec(q.Normal) != (geom.Vec3{Y: 1}) || math.Abs(q.Area-38*math.Pi) > 1e-10 || vec(q.Center).Distance(geom.Vec3{X: 20. / 19, Y: 2, Z: 59. / 19}) > 1e-10 || q.Edges != 2 || math.Abs(q.Length-length) > 1e-9 || vec(q.Point).Distance(p) > 1e-13 || vec(q.Derivative).Distance(d) > 1e-13 {
			t.Fatal(q, length, p, d)
		}
	}
}
