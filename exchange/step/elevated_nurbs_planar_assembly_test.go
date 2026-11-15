package step

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"math"
	"math/big"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strconv"
	"testing"
	"time"
)

func elevatedNURBSFixture(t testing.TB) []byte {
	t.Helper()
	b, e := os.ReadFile("testdata/occt-ap242-elevated-nurbs-planar.step")
	if e != nil {
		t.Fatal(e)
	}
	return b
}
func TestElevatedNURBSPlanarAssembly(t *testing.T) {
	ctx := context.Background()
	raw := elevatedNURBSFixture(t)
	m, e := ReadElevatedNURBSPlanarAssembly(ctx, bytes.NewReader(raw), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	if len(m.Surfaces()) != 12 || len(m.PCurves()) != 48 || len(m.Geometry.Assembly.Occurrences) != 3 {
		t.Fatal("source roles")
	}
	f, e := Parse(bytes.NewReader(raw), Limits{})
	if e != nil || !reflect.DeepEqual(f, m.Geometry.Assembly.Source) {
		t.Fatal("original source", e)
	}
	if _, e = ReadObliqueNURBSPlanarAssembly(ctx, bytes.NewReader(raw), Limits{}); e == nil {
		t.Fatal("old reader widened")
	}
	if _, e = ReadQuadraticNURBSPlanarAssembly(ctx, bytes.NewReader(raw), Limits{}); e == nil {
		t.Fatal("quadratic gate widened")
	}
	for _, s := range m.Surfaces() {
		if s.FaceID == 0 || s.SurfaceID == 0 || s.PoleIDs[1][1] == 0 {
			t.Fatal(s)
		}
		r, e := simple(f, s.FaceID, "ADVANCED_FACE", 4)
		if e != nil || r.Args[2].Ref != s.SurfaceID {
			t.Fatal("surface owner")
		}
	}
	for _, c := range m.PCurves() {
		r, e := simple(f, c.EdgeID, "EDGE_CURVE", 5)
		if e != nil || r.Args[3].Ref != c.SurfaceCurveID {
			t.Fatal("model owner")
		}
		p, e := simple(f, c.PCurveID, "PCURVE", 3)
		if e != nil || p.Args[1].Ref != c.SurfaceID {
			t.Fatal("pcurve owner")
		}
	}
	areas := map[float64]float64{4050: 1710, 2700: 1440}
	for _, bodies := range m.Geometry.ProductBodies {
		for _, b := range bodies {
			mass, e := b.Body.Solid.MassProperties()
			if e != nil || areas[mass.Volume] != mass.SurfaceArea {
				t.Fatal(mass, e)
			}
		}
	}
	surfaces := m.Surfaces()
	surfaces[0].Poles[1][1][0] = "forged"
	if m.Surfaces()[0].Poles[1][1][0] == "forged" {
		t.Fatal("mutable evidence")
	}
	var source bytes.Buffer
	if e = WriteElevatedNURBSPlanarAssemblySource(ctx, &source, m, Limits{}); e != nil || !bytes.Equal(source.Bytes(), raw) {
		t.Fatal("source identity", e)
	}
	for i, j := 0, len(f.Order)-1; i < j; i, j = i+1, j-1 {
		f.Order[i], f.Order[j] = f.Order[j], f.Order[i]
	}
	reordered, e := mappedPlanarSourceBytes(f)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = ReadElevatedNURBSPlanarAssembly(ctx, bytes.NewReader(reordered), Limits{}); e != nil {
		t.Fatal("DE order", e)
	}
}
func TestElevatedNURBSPlanarReject(t *testing.T) {
	ctx := context.Background()
	raw := elevatedNURBSFixture(t)
	m, e := ReadElevatedNURBSPlanarAssembly(ctx, bytes.NewReader(raw), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	s, c := m.Surfaces()[0], m.PCurves()[0]
	for name, edit := range map[string]func(*File){
		"interior": func(f *File) { f.Entities[s.PoleIDs[1][1]].Records[0].Args[1].Items[0].Text = "99." },
		"near-affine": func(f *File) {
			p := &f.Entities[s.PoleIDs[1][1]].Records[0].Args[1].Items[0]
			q, _ := liftSourceRat(*p)
			q.Add(q, big.NewRat(1, 1000000000000000000))
			p.Text = q.FloatString(18)
		},
		"pcurve-midpoint": func(f *File) {
			p := &f.Entities[c.PoleIDs[1]].Records[0].Args[1].Items[0]
			q, _ := liftSourceRat(*p)
			q.Add(q, big.NewRat(1, 1000000000000000000))
			p.Text = q.FloatString(18)
		},
		"degree-decimal": func(f *File) { f.Entities[s.SurfaceID].Records[0].Args[1].Text = "3.00000000000000001" },
		"knot-domain":    func(f *File) { f.Entities[c.CurveID].Records[0].Args[7].Items[1].Text = "99." },
		"wrong-owner":    func(f *File) { f.Entities[c.PCurveID].Records[0].Args[1].Ref = m.Surfaces()[3].SurfaceID },
		"pole-sharing":   func(f *File) { f.Entities[c.CurveID].Records[0].Args[2].Items[1].Ref = c.PoleIDs[0] },
		"underflow":      func(f *File) { f.Entities[s.PoleIDs[1][1]].Records[0].Args[1].Items[0].Text = "1.E-999999999" },
	} {
		t.Run(name, func(t *testing.T) {
			f, e := Parse(bytes.NewReader(raw), Limits{})
			if e != nil {
				t.Fatal(e)
			}
			edit(f)
			b, e := mappedPlanarSourceBytes(f)
			if e != nil {
				t.Fatal(e)
			}
			q, e := ReadElevatedNURBSPlanarAssembly(ctx, bytes.NewReader(b), Limits{})
			if e == nil || q.original != "" || len(q.Geometry.ProductBodies) != 0 {
				t.Fatal("admitted", e)
			}
		})
	}
	for _, l := range []Limits{{MaxBytes: 10}, {MaxEntities: 10}, {MaxValues: 100}, {MaxDepth: 1}} {
		if _, e = ReadElevatedNURBSPlanarAssembly(ctx, bytes.NewReader(raw), l); e == nil {
			t.Fatal("limit")
		}
	}
	cancelctx, cancel := context.WithCancel(ctx)
	cancel()
	if _, e = ReadElevatedNURBSPlanarAssembly(cancelctx, bytes.NewReader(raw), Limits{}); e == nil {
		t.Fatal("cancel")
	}
}
func elevatedNURBSNativeFixture(t testing.TB) []byte {
	t.Helper()
	m, e := ReadElevatedNURBSPlanarAssembly(context.Background(), bytes.NewReader(elevatedNURBSFixture(t)), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	var out bytes.Buffer
	if e = WriteElevatedNURBSPlanarAssemblyJSON(context.Background(), &out, m); e != nil {
		t.Fatal(e)
	}
	return out.Bytes()
}
func TestElevatedNURBSNative(t *testing.T) {
	ctx := context.Background()
	raw := elevatedNURBSNativeFixture(t)
	m, e := ReadElevatedNURBSPlanarAssemblyJSON(ctx, bytes.NewReader(raw), IndependentMaterialJSONLimits{})
	if e != nil {
		t.Fatal(e)
	}
	var stable bytes.Buffer
	if e = WriteElevatedNURBSPlanarAssemblyJSON(ctx, &stable, m); e != nil || !bytes.Equal(raw, stable.Bytes()) {
		t.Fatal("stable", e)
	}
	if e = WriteElevatedNURBSPlanarAssemblyJSON(ctx, sideDiskShortWriter{}, m); !errors.Is(e, io.ErrShortWrite) {
		t.Fatal(e)
	}
	if e = WriteElevatedNURBSPlanarAssemblySource(ctx, sideDiskShortWriter{}, m, Limits{}); !errors.Is(e, io.ErrShortWrite) {
		t.Fatal(e)
	}
	if _, e = ReadElevatedNURBSPlanarAssemblyJSON(ctx, bytes.NewReader(append(append([]byte(nil), raw...), []byte("{}")...)), IndependentMaterialJSONLimits{}); e == nil {
		t.Fatal("trailing")
	}
	if _, e = ReadElevatedNURBSPlanarAssemblyJSON(ctx, bytes.NewReader(raw), IndependentMaterialJSONLimits{Source: Limits{MaxEntities: 10}}); e == nil {
		t.Fatal("source limit")
	}
	cc, cancel := context.WithCancel(ctx)
	cancel()
	if _, e = ReadElevatedNURBSPlanarAssemblyJSON(cc, bytes.NewReader(raw), IndependentMaterialJSONLimits{}); e == nil {
		t.Fatal("cancel")
	}
	for _, edit := range []func(*elevatedNURBSNativeEnvelope){func(q *elevatedNURBSNativeEnvelope) { q.Format = nurbsPlanarAssemblyNativeFormat }, func(q *elevatedNURBSNativeEnvelope) { q.Source = "ISO-10303-21;" }, func(q *elevatedNURBSNativeEnvelope) { q.Surfaces[0].PoleIDs[1][1]++ }, func(q *elevatedNURBSNativeEnvelope) { q.Curves[0].EdgeID++ }, func(q *elevatedNURBSNativeEnvelope) { q.Surfaces[0].Poles[1][1][0] = "0" }, func(q *elevatedNURBSNativeEnvelope) { q.Geometry.Occurrences[0].Transform[0][3]++ }, func(q *elevatedNURBSNativeEnvelope) {
		for i := range q.Geometry.Products {
			if len(q.Geometry.Products[i].Bodies) > 0 {
				q.Geometry.Products[i].Bodies[0].Vertices[0].X++
				return
			}
		}
	}} {
		var q elevatedNURBSNativeEnvelope
		if e = json.Unmarshal(raw, &q); e != nil {
			t.Fatal(e)
		}
		edit(&q)
		b, _ := json.Marshal(q)
		bad, e := ReadElevatedNURBSPlanarAssemblyJSON(ctx, bytes.NewReader(b), IndependentMaterialJSONLimits{})
		if e == nil || bad.original != "" {
			t.Fatal("forged", e)
		}
	}
	if _, e = ReadObliqueNURBSPlanarAssemblyJSON(ctx, bytes.NewReader(raw), IndependentMaterialJSONLimits{}); e == nil {
		t.Fatal("old tag")
	}
	if _, e = ReadElevatedNURBSPlanarAssemblyJSON(ctx, bytes.NewReader(raw), IndependentMaterialJSONLimits{MaxBytes: 10}); e == nil {
		t.Fatal("limit")
	}
	m.surfaces[0].FaceID++
	var out bytes.Buffer
	if e = WriteElevatedNURBSPlanarAssemblySource(ctx, &out, m, Limits{}); e == nil || out.Len() != 0 {
		t.Fatal("stale source")
	}
	if e = WriteElevatedNURBSPlanarAssemblyJSON(ctx, &out, m); e == nil || out.Len() != 0 {
		t.Fatal("stale native")
	}
}
func TestElevatedNURBSLive(t *testing.T) {
	bin := os.Getenv("PLAINSOLID_OCCT_ELEVATED_NURBS_PLANAR")
	if bin == "" {
		t.Skip("optional pinned OCCT")
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	for _, mode := range []string{"write", "write-shifted", "write-mixed"} {
		dir := t.TempDir()
		src := filepath.Join(dir, "source.step")
		if b, e := exec.CommandContext(ctx, bin, mode, src).CombinedOutput(); e != nil {
			t.Fatal(e, string(b))
		}
		raw, e := os.ReadFile(src)
		if e != nil {
			t.Fatal(e)
		}
		m, e := ReadElevatedNURBSPlanarAssembly(ctx, bytes.NewReader(raw), Limits{})
		if e != nil {
			t.Fatal(e)
		}
		var native bytes.Buffer
		if e = WriteElevatedNURBSPlanarAssemblyJSON(ctx, &native, m); e != nil {
			t.Fatal(e)
		}
		m, e = ReadElevatedNURBSPlanarAssemblyJSON(ctx, &native, IndependentMaterialJSONLimits{})
		if e != nil {
			t.Fatal(e)
		}
		var out bytes.Buffer
		if e = WriteElevatedNURBSPlanarAssemblySource(ctx, &out, m, Limits{}); e != nil {
			t.Fatal(e)
		}
		dst := filepath.Join(dir, "output.step")
		if e = os.WriteFile(dst, out.Bytes(), 0600); e != nil {
			t.Fatal(e)
		}
		for _, path := range []string{src, dst} {
			b, e := exec.CommandContext(ctx, bin, "read", path).Output()
			if e != nil {
				t.Fatal(e, string(b))
			}
			var q struct {
				Volume, Area  float64
				Solids, Faces int
				Centroid      [3]float64
				Colors        [][3]float64
			}
			if e = json.Unmarshal(b, &q); e != nil {
				t.Fatal(e, string(b))
			}
			if math.Abs(q.Volume-10800) > 1e-9 || math.Abs(q.Area-4860) > 1e-9 || q.Solids != 3 || q.Faces != 18 || len(q.Colors) != 2 {
				t.Fatal(q)
			}
			want := [3]float64{9.3125, 14.5, 12.0625}
			for i := range want {
				if math.Abs(q.Centroid[i]-want[i]) > 1e-12 {
					t.Fatal(q)
				}
			}
		}
	}
}
func FuzzElevatedNURBSPlanar(f *testing.F) {
	f.Add(elevatedNURBSFixture(f))
	mixed, e := os.ReadFile("testdata/occt-ap242-elevated-mixed-nurbs-planar.step")
	if e != nil {
		f.Fatal(e)
	}
	f.Add(mixed)
	f.Fuzz(func(t *testing.T, b []byte) {
		if len(b) > 1<<20 {
			t.Skip()
		}
		m, e := ReadElevatedNURBSPlanarAssembly(context.Background(), bytes.NewReader(b), Limits{MaxBytes: 1 << 20, MaxEntities: 4096, MaxDepth: 16})
		if e != nil && m.original != "" {
			t.Fatal("partial")
		}
	})
}

func TestElevatedNURBSMixedAndRoundedSource(t *testing.T) {
	raw, e := os.ReadFile("testdata/occt-ap242-elevated-mixed-nurbs-planar.step")
	if e != nil {
		t.Fatal(e)
	}
	m, e := ReadElevatedNURBSPlanarAssembly(context.Background(), bytes.NewReader(raw), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	for _, s := range m.Surfaces() {
		if s.UDegree != 2 || s.VDegree != 3 || s.UKnots != [2]string{"0", "3"} || s.VKnots != [2]string{"0", "6"} {
			t.Fatal(s)
		}
	}
	for _, c := range m.PCurves() {
		if c.Degree != 4 {
			t.Fatal(c)
		}
	}
	raw, e = os.ReadFile("testdata/occt-ap242-elevated-rounded-oblique.step")
	if e != nil {
		t.Fatal(e)
	}
	if m, e = ReadElevatedNURBSPlanarAssembly(context.Background(), bytes.NewReader(raw), Limits{}); e == nil || m.original != "" {
		t.Fatal("rounded original promoted", e)
	}
}

// Independently generate exact Bernstein controls from the preserved endpoints
// to exercise the admitted degree ceiling, including a degree-one tensor axis.
func TestElevatedNURBSMaximumDegree(t *testing.T) {
	ctx := context.Background()
	raw := elevatedNURBSFixture(t)
	source, e := ReadElevatedNURBSPlanarAssembly(ctx, bytes.NewReader(raw), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	for _, ud := range []int{1, 8} {
		f, e := Parse(bytes.NewReader(raw), Limits{})
		if e != nil {
			t.Fatal(e)
		}
		var next int64
		for id := range f.Entities {
			if id > next {
				next = id
			}
		}
		removed := map[int64]bool{}
		num := func(n int) Value { return Value{Kind: Number, Text: strconv.Itoa(n)} }
		point := func(a []*big.Rat) int64 {
			next++
			vals := make([]Value, len(a))
			for i, q := range a {
				s, e := extrusionDirectionDecimal(q)
				if e != nil {
					t.Fatal(e)
				}
				vals[i] = Value{Kind: Number, Text: s}
			}
			f.Entities[next] = Entity{ID: next, Records: []Record{{Type: "CARTESIAN_POINT", Args: []Value{{Kind: String}, {Kind: Aggregate, Items: vals}}}}}
			f.Order = append(f.Order, next)
			return next
		}
		rat := func(s string) *big.Rat {
			q, ok := new(big.Rat).SetString(s)
			if !ok {
				t.Fatal(s)
			}
			return q
		}
		for _, s := range source.Surfaces() {
			for i := 0; i <= s.UDegree; i++ {
				for j := 0; j <= s.VDegree; j++ {
					removed[s.PoleIDs[i][j]] = true
				}
			}
			r := f.Entities[s.SurfaceID].Records[0]
			rows := make([]Value, ud+1)
			for i := 0; i <= ud; i++ {
				row := make([]Value, 9)
				for j := 0; j <= 8; j++ {
					a := make([]*big.Rat, 3)
					for k := range a {
						origin := rat(s.Poles[0][0][k])
						u := new(big.Rat).Sub(rat(s.Poles[s.UDegree][0][k]), origin)
						v := new(big.Rat).Sub(rat(s.Poles[0][s.VDegree][k]), origin)
						a[k] = new(big.Rat).Add(origin, new(big.Rat).Add(new(big.Rat).Mul(big.NewRat(int64(i), int64(ud)), u), new(big.Rat).Mul(big.NewRat(int64(j), 8), v)))
					}
					row[j] = Value{Kind: Reference, Ref: point(a)}
				}
				rows[i] = Value{Kind: Aggregate, Items: row}
			}
			r.Args[1], r.Args[2], r.Args[3] = num(ud), num(8), Value{Kind: Aggregate, Items: rows}
			r.Args[8] = Value{Kind: Aggregate, Items: []Value{num(ud + 1), num(ud + 1)}}
			r.Args[9] = Value{Kind: Aggregate, Items: []Value{num(9), num(9)}}
			f.Entities[s.SurfaceID] = Entity{ID: s.SurfaceID, Records: []Record{r}}
		}
		for _, c := range source.PCurves() {
			for i := 0; i <= c.Degree; i++ {
				removed[c.PoleIDs[i]] = true
			}
			r := f.Entities[c.CurveID].Records[0]
			ids := make([]Value, 9)
			for i := 0; i <= 8; i++ {
				a := make([]*big.Rat, 2)
				for j := range a {
					start := rat(c.Poles[0][j])
					a[j] = new(big.Rat).Add(start, new(big.Rat).Mul(big.NewRat(int64(i), 8), new(big.Rat).Sub(rat(c.Poles[c.Degree][j]), start)))
				}
				ids[i] = Value{Kind: Reference, Ref: point(a)}
			}
			r.Args[1] = num(8)
			r.Args[2] = Value{Kind: Aggregate, Items: ids}
			r.Args[6] = Value{Kind: Aggregate, Items: []Value{num(9), num(9)}}
			f.Entities[c.CurveID] = Entity{ID: c.CurveID, Records: []Record{r}}
		}
		order := make([]int64, 0, len(f.Order))
		for _, id := range f.Order {
			if removed[id] {
				delete(f.Entities, id)
			} else {
				order = append(order, id)
			}
		}
		f.Order = order
		b, e := mappedPlanarSourceBytes(f)
		if e != nil {
			t.Fatal(e)
		}
		m, e := ReadElevatedNURBSPlanarAssembly(ctx, bytes.NewReader(b), Limits{})
		if e != nil {
			t.Fatal("degree ceiling", ud, e)
		}
		if m.Surfaces()[0].UDegree != ud || m.Surfaces()[0].VDegree != 8 || m.PCurves()[0].Degree != 8 {
			t.Fatal("degree evidence")
		}
		f.Entities[m.Surfaces()[0].SurfaceID].Records[0].Args[1] = num(9)
		b, e = mappedPlanarSourceBytes(f)
		if e != nil {
			t.Fatal(e)
		}
		if _, e = ReadElevatedNURBSPlanarAssembly(ctx, bytes.NewReader(b), Limits{}); e == nil {
			t.Fatal("over degree quota")
		}
	}
}
