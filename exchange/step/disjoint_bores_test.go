package step

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"plainsolid/brep/parametric"
	"plainsolid/geom"
	"reflect"
	"regexp"
	"strings"
	"testing"
)

func checkDisjointBoresSTEP(t *testing.T, p DisjointBoresBody) {
	t.Helper()
	m := DisjointBoresModel{p, .001}
	var out bytes.Buffer
	if e := WriteDisjointBores(&out, m); e != nil {
		t.Fatal(e)
	}
	got, e := ReadDisjointBores(bytes.NewReader(out.Bytes()), Limits{})
	want := m
	want.Body.Bores = sortedDisjointBores(p.Bores)
	if e != nil || !reflect.DeepEqual(got, want) {
		t.Fatal(got, want, e)
	}
	reverse := m
	reverse.Body.Bores = append([]DisjointBore(nil), p.Bores...)
	for i, j := 0, len(p.Bores)-1; i < j; i, j = i+1, j-1 {
		reverse.Body.Bores[i], reverse.Body.Bores[j] = reverse.Body.Bores[j], reverse.Body.Bores[i]
	}
	var again bytes.Buffer
	if e = WriteDisjointBores(&again, reverse); e != nil || !bytes.Equal(out.Bytes(), again.Bytes()) {
		t.Fatal("order changed STEP", e)
	}
	d := p.Dimensions
	volume := d.X * d.Y * d.Z
	area := 2 * (d.X*d.Y + d.X*d.Z + d.Y*d.Z)
	moment := d.Scale(volume / 2)
	for _, b := range p.Bores {
		length := []float64{d.X, d.Y, d.Z}[b.Axis]
		v := math.Pi * b.Radius * b.Radius * length
		c := b.Center
		if b.Axis == 0 {
			c.X = length / 2
		} else if b.Axis == 1 {
			c.Y = length / 2
		} else {
			c.Z = length / 2
		}
		volume -= v
		area += 2*math.Pi*b.Radius*length - 2*math.Pi*b.Radius*b.Radius
		moment = moment.Sub(c.Scale(v))
	}
	native, e := p.Solid(context.Background())
	if e != nil {
		t.Fatal(e)
	}
	mass, e := native.MassProperties(context.Background(), parametric.IntegrationOptions{})
	center := p.Frame.ToWorld(moment.Scale(1 / volume))
	if e != nil || math.Abs(mass.Volume-volume) > 1e-7 || math.Abs(mass.Area-area) > 1e-7 || mass.Centroid.Distance(center) > 1e-7 {
		t.Fatal(mass, volume, area, center, e)
	}
	if os.Getenv("PLAINSOLID_OCCT_STEP") == "" {
		return
	}
	path, report := filepath.Join(t.TempDir(), "bores.step"), filepath.Join(t.TempDir(), "mass.json")
	if e = os.WriteFile(path, out.Bytes(), 0600); e != nil {
		t.Fatal(e)
	}
	stepOracle(t, report, "step-mass-precise", path)
	raw, e := os.ReadFile(report)
	if e != nil {
		t.Fatal(e)
	}
	var occt struct {
		Volume, Area float64
		Centroid     [3]float64
	}
	if e = json.Unmarshal(raw, &occt); e != nil {
		t.Fatal(e)
	}
	if math.Abs(occt.Volume-volume) > 1e-7 || math.Abs(occt.Area-area) > 1e-7 || center.Distance(geom.Vec3{X: occt.Centroid[0], Y: occt.Centroid[1], Z: occt.Centroid[2]}) > 1e-7 {
		t.Fatal(occt, volume, area, center)
	}
}
func TestDisjointBoresSTEP(t *testing.T) {
	for ni, n := range []geom.Vec3{{Z: 1}, {Z: -1}, {X: 1}, {X: -1}, {Y: 1}, {Y: -1}} {
		f, _ := geom.FrameFromNormal(geom.Vec3{X: 16, Y: 32, Z: 48}, n)
		for count := 2; count <= 3; count++ {
			t.Run(fmt.Sprintf("%d/%d", ni, count), func(t *testing.T) {
				p := DisjointBoresBody{Name: "disjoint cardinal bores", Frame: f, Dimensions: geom.Vec3{X: 24, Y: 24, Z: 24}, Bores: []DisjointBore{{0, geom.Vec3{Y: 4, Z: 4}, 1.5}, {1, geom.Vec3{X: 12, Z: 12}, 2}, {2, geom.Vec3{X: 20, Y: 20}, 1}}}
				p.Bores = p.Bores[:count]
				checkDisjointBoresSTEP(t, p)
			})
		}
	}
}
func TestDisjointBoresSTEPAtLimit(t *testing.T) {
	f, _ := geom.FrameFromNormal(geom.Vec3{}, geom.Vec3{Z: 1})
	p := DisjointBoresBody{Name: "32 cardinal bores", Frame: f, Dimensions: geom.Vec3{X: 20, Y: 160, Z: 20}}
	for i := 0; i < 31; i++ {
		p.Bores = append(p.Bores, DisjointBore{0, geom.Vec3{Y: 4 + 4*float64(i), Z: 4}, 1})
	}
	p.Bores = append(p.Bores, DisjointBore{2, geom.Vec3{X: 4, Y: 140}, 1})
	checkDisjointBoresSTEP(t, p)
	var out bytes.Buffer
	if e := WriteDisjointBores(&out, DisjointBoresModel{p, .001}); e != nil {
		t.Fatal(e)
	}
	if _, e := ReadDisjointBores(bytes.NewReader(out.Bytes()), Limits{MaxFaces: 64}); e == nil {
		t.Fatal("ignored caller face budget")
	}
}
func TestDisjointBoresSTEPRejects(t *testing.T) {
	f, _ := geom.FrameFromNormal(geom.Vec3{}, geom.Vec3{Z: 1})
	base := DisjointBoresBody{Name: "rejections", Frame: f, Dimensions: geom.Vec3{X: 24, Y: 24, Z: 24}, Bores: []DisjointBore{{0, geom.Vec3{Y: 4, Z: 4}, 1.5}, {1, geom.Vec3{X: 12, Z: 12}, 2}}}
	for _, cuts := range [][]DisjointBore{base.Bores[:1], {base.Bores[0], {0, geom.Vec3{Y: 12, Z: 12}, 1}}, {base.Bores[0], {1, geom.Vec3{X: 12, Z: 4}, 2}}, {base.Bores[0], {1, geom.Vec3{X: 12, Y: 1, Z: 12}, 2}}, make([]DisjointBore, 33)} {
		p := base
		p.Bores = cuts
		var out bytes.Buffer
		if e := WriteDisjointBores(&out, DisjointBoresModel{p, .001}); e == nil || out.Len() != 0 {
			t.Fatal("published invalid recipe", cuts, e)
		}
	}
	var out bytes.Buffer
	if e := WriteDisjointBores(&out, DisjointBoresModel{base, .001}); e != nil {
		t.Fatal(e)
	}
	source := out.String()
	radius := regexp.MustCompile(`CYLINDRICAL_SURFACE\('',#[0-9]+,[^)]+\)`).FindStringIndex(source)
	if radius == nil {
		t.Fatal("missing cylinder")
	}
	support := source[radius[0]:radius[1]]
	comma := strings.LastIndex(support, ",")
	changed := source[:radius[0]] + support[:comma] + ",1.25)" + source[radius[1]:]
	if _, e := ReadDisjointBores(strings.NewReader(changed), Limits{}); e == nil {
		t.Fatal("accepted support inconsistent with edges")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, e := base.Solid(ctx); e != context.Canceled {
		t.Fatal("cancel", e)
	}
	bad := strings.Replace(out.String(), "CYLINDRICAL_SURFACE(", "CONICAL_SURFACE(", 1)
	if _, e := ReadDisjointBores(strings.NewReader(bad), Limits{}); e == nil {
		t.Fatal("corrupt support accepted")
	}
}
