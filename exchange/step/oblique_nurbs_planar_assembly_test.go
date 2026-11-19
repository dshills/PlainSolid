package step

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"math"
	"math/big"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"plainsolid/geom"
)

func obliqueNURBSFixture(t testing.TB) []byte {
	t.Helper()
	b, e := os.ReadFile("testdata/occt-ap242-oblique-nurbs-planar-assembly.step")
	if e != nil {
		t.Fatal(e)
	}
	return b
}
func TestObliqueNURBSPlanarAssembly(t *testing.T) {
	t.Parallel()
	raw := obliqueNURBSFixture(t)
	ctx := context.Background()
	m, e := ReadObliqueNURBSPlanarAssembly(ctx, bytes.NewReader(raw), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	if len(m.ProductFrames()) != 2 || len(m.SourcePoints()) != 112 || len(m.Geometry.Assembly.Occurrences) != 3 {
		t.Fatal("missing source ownership")
	}
	if _, e = ReadScaledNURBSPlanarAssembly(ctx, bytes.NewReader(raw), Limits{}); e == nil {
		t.Fatal("scaled-cardinal reader widened")
	}
	f, e := Parse(bytes.NewReader(raw), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	if !reflect.DeepEqual(f, m.Geometry.Assembly.Source) {
		t.Fatal("native source is not original")
	}
	ratio := false
	frames := map[int64]ObliqueNURBSProductFrame{}
	rat := func(s string) *big.Rat {
		q, ok := new(big.Rat).SetString(s)
		if !ok {
			t.Fatal(s)
		}
		return q
	}
	for _, b := range m.ProductFrames() {
		frames[b.DefinitionID] = b
		var x, y, z [3]*big.Rat
		for j := 0; j < 3; j++ {
			x[j], y[j], z[j] = rat(b.Basis[0][j]), rat(b.Basis[1][j]), rat(b.Basis[2][j])
			if b.Basis[1][j] == "4/5" || b.Basis[2][j] == "3/5" {
				ratio = true
			}
		}
		if mappedRationalDot(x, x).Cmp(big.NewRat(1, 1)) != 0 || mappedRationalDot(y, y).Cmp(big.NewRat(1, 1)) != 0 || mappedRationalDot(x, y).Sign() != 0 || mappedRationalDot(mappedRationalCross(x, y), z).Cmp(big.NewRat(1, 1)) != 0 {
			t.Fatal("not exact proper rigid basis")
		}
	}
	if !ratio {
		t.Fatal("fixture did not exercise nonbinary frame")
	}
	for _, p := range m.SourcePoints() {
		b := frames[p.DefinitionID]
		for j := 0; j < 3; j++ {
			want := rat(b.Origin[j])
			for k := 0; k < 3; k++ {
				want.Add(want, new(big.Rat).Mul(rat(b.Basis[k][j]), rat(p.Proof[k])))
			}
			if want.Cmp(rat(p.Source[j])) != 0 {
				t.Fatal("not exact inverse", p.PointID)
			}
		}
	}
	seen := map[float64]bool{}
	for _, bs := range m.Geometry.ProductBodies {
		for _, b := range bs {
			mass, e := b.Body.Solid.MassProperties()
			if e != nil {
				t.Fatal(e)
			}
			seen[mass.Volume] = true
			switch mass.Volume {
			case 750:
				if mass.Centroid != (geom.Vec3{X: -2.5, Y: 5, Z: 7.5}) || mass.SurfaceArea != 550 {
					t.Fatal(mass)
				}
			case 250:
				if mass.Centroid != (geom.Vec3{X: -.5, Y: 3.5, Z: 5}) || mass.SurfaceArea != 250 {
					t.Fatal(mass)
				}
			default:
				t.Fatal(mass)
			}
		}
	}
	if !seen[750] || !seen[250] {
		t.Fatal(seen)
	}
	c := m.ProductFrames()[0]
	copies := m.ProductFrames()
	copies[0].Basis[0][0] = "forged"
	if m.ProductFrames()[0] != c {
		t.Fatal("mutable frame evidence")
	}
	var out bytes.Buffer
	if e = WriteObliqueNURBSPlanarAssemblySource(ctx, &out, m, Limits{}); e != nil || !bytes.Equal(raw, out.Bytes()) {
		t.Fatal("source loss", e)
	}
	n, e := ReadObliqueNURBSPlanarAssembly(ctx, bytes.NewReader(out.Bytes()), Limits{})
	if e != nil || !reflect.DeepEqual(n.frames, m.frames) || !reflect.DeepEqual(n.points, m.points) {
		t.Fatal("source ancestry loss", e)
	}
	for id, bs := range m.Geometry.ProductBodies {
		for i, b := range bs {
			got := n.Geometry.ProductBodies[id][i].Body.Solid
			if !reflect.DeepEqual(b.Body.Solid.Vertices(), got.Vertices()) || !reflect.DeepEqual(b.Body.Solid.Edges(), got.Edges()) || !reflect.DeepEqual(b.Body.Solid.Faces(), got.Faces()) {
				t.Fatal("native exact vertex/topology loss")
			}
		}
	}
}
func TestObliqueNURBSPlanarReject(t *testing.T) {
	t.Parallel()
	raw := obliqueNURBSFixture(t)
	ctx := context.Background()
	m, e := ReadObliqueNURBSPlanarAssembly(ctx, bytes.NewReader(raw), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	b := m.ProductFrames()[0]
	mutations := map[string]func(*File){
		"rounded pole":      func(f *File) { f.Entities[b.PoleIDs[0]].Records[0].Args[1].Items[0].Text = "-8.00000000000000000001" },
		"nonaffine support": func(f *File) { f.Entities[b.PoleIDs[3]].Records[0].Args[1].Items[0].Text = "1." },
		"irrational norm": func(f *File) {
			r := f.Entities[b.PoleIDs[2]].Records[0]
			r.Args[1].Items[0].Text = "-5."
			r.Args[1].Items[1].Text = "10."
		},
		"nonorthogonal support": func(f *File) { f.Entities[b.PoleIDs[2]].Records[0].Args[1].Items[0].Text = "-7." },
		"cross product geometry": func(f *File) {
			face, _ := simple(f, b.FaceID, "ADVANCED_FACE", 4)
			face.Args[2].Ref = m.ProductFrames()[1].SurfaceID
		},
		"geometry borrowed by pose": func(f *File) {
			for _, id := range f.Order {
				r, e := simple(f, id, "ITEM_DEFINED_TRANSFORMATION", 4)
				if e == nil {
					pl, _ := simple(f, r.Args[3].Ref, "AXIS2_PLACEMENT_3D", 4)
					pl.Args[1].Ref = b.PoleIDs[0]
					return
				}
			}
		},
		"underflow": func(f *File) { f.Entities[b.PoleIDs[0]].Records[0].Args[1].Items[0].Text = "1.E-999999999" },
	}
	for name, mutate := range mutations {
		t.Run(name, func(t *testing.T) {
			f, e := Parse(bytes.NewReader(raw), Limits{})
			if e != nil {
				t.Fatal(e)
			}
			mutate(f)
			data, e := mappedPlanarSourceBytes(f)
			if e != nil {
				t.Fatal(e)
			}
			got, e := ReadObliqueNURBSPlanarAssembly(ctx, bytes.NewReader(data), Limits{})
			if e == nil || got.original != "" || len(got.Geometry.ProductBodies) != 0 {
				t.Fatal("accepted invalid source", e)
			}
		})
	}
	// A valid exact global shear is outside this rigid proof domain.
	f, e := Parse(bytes.NewReader(raw), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	for _, p := range m.SourcePoints() {
		r := f.Entities[p.PointID].Records[0]
		x, _ := liftSourceRat(r.Args[1].Items[0])
		z, _ := liftSourceRat(r.Args[1].Items[2])
		x.Add(x, z)
		text, e := extrusionDirectionDecimal(x)
		if e != nil {
			t.Fatal(e)
		}
		r.Args[1].Items[0].Text = text
	}
	data, e := mappedPlanarSourceBytes(f)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = ReadObliqueNURBSPlanarAssembly(ctx, bytes.NewReader(data), Limits{}); e == nil {
		t.Fatal("accepted shear as rigid")
	}
	// No role is inferred from entity order.
	f, e = Parse(bytes.NewReader(raw), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	for i, j := 0, len(f.Order)-1; i < j; i, j = i+1, j-1 {
		f.Order[i], f.Order[j] = f.Order[j], f.Order[i]
	}
	data, e = mappedPlanarSourceBytes(f)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = ReadObliqueNURBSPlanarAssembly(ctx, bytes.NewReader(data), Limits{}); e != nil {
		t.Fatal("reordered source", e)
	}
}
func TestObliqueNURBSPlanarLimits(t *testing.T) {
	t.Parallel()
	raw := obliqueNURBSFixture(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, e := ReadObliqueNURBSPlanarAssembly(ctx, bytes.NewReader(raw), Limits{}); !errors.Is(e, context.Canceled) {
		t.Fatal(e)
	}
	for _, l := range []Limits{{MaxBytes: int64(len(raw) - 1)}, {MaxEntities: 600}, {MaxSolids: 1}, {MaxDepth: 2}} {
		if m, e := ReadObliqueNURBSPlanarAssembly(context.Background(), bytes.NewReader(raw), l); e == nil || m.original != "" {
			t.Fatal("limit accepted", l)
		}
	}
	var out bytes.Buffer
	if e := WriteObliqueNURBSPlanarAssemblySource(context.Background(), &out, ObliqueNURBSPlanarAssembly{}, Limits{}); e == nil || out.Len() != 0 {
		t.Fatal("invalid write was not atomic")
	}
}
func TestObliqueNURBSPlanarLive(t *testing.T) {
	t.Parallel()
	raw := obliqueNURBSFixture(t)
	ctx := context.Background()
	m, e := ReadObliqueNURBSPlanarAssembly(ctx, bytes.NewReader(raw), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	var out bytes.Buffer
	if e = WriteObliqueNURBSPlanarAssemblySource(ctx, &out, m, Limits{}); e != nil {
		t.Fatal(e)
	}
	// Native body coordinates are restored before applying the unchanged uses.
	volume, area := 0., 0.
	moment := geom.Vec3{}
	for _, o := range m.Geometry.Assembly.Occurrences {
		bs := m.Geometry.ProductBodies[o.Child]
		if len(bs) != 1 {
			t.Fatal("unexpected fixture nesting")
		}
		p, e := bs[0].Body.Solid.MassProperties()
		if e != nil {
			t.Fatal(e)
		}
		volume += p.Volume
		area += p.SurfaceArea
		moment = moment.Add(o.Transform.Apply(p.Centroid).Scale(p.Volume))
	}
	center := moment.Scale(1 / volume)
	if volume != 1750 || area != 1350 || math.Abs(center.X-69.0/14) > 1e-12 || math.Abs(center.Y-66.0/7) > 1e-12 || math.Abs(center.Z-99.0/14) > 1e-12 {
		t.Fatal(volume, area, center)
	}
	bin := os.Getenv("PLAINSOLID_OCCT_AP242_OBLIQUE_NURBS_PLANAR")
	if bin == "" {
		t.Skip("set PLAINSOLID_OCCT_AP242_OBLIQUE_NURBS_PLANAR")
	}
	// A fresh independent shifted-chart source also exercises the private basis.
	shiftedPath := filepath.Join(t.TempDir(), "shifted.step")
	c, cancel := context.WithTimeout(ctx, 30*time.Second)
	auth, e := exec.CommandContext(c, bin, "write-shifted", shiftedPath).CombinedOutput()
	cancel()
	if e != nil {
		t.Fatalf("%v: %s", e, auth)
	}
	shifted, e := os.ReadFile(shiftedPath)
	if e != nil {
		t.Fatal(e)
	}
	sm, e := ReadObliqueNURBSPlanarAssembly(ctx, bytes.NewReader(shifted), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	var so bytes.Buffer
	if e = WriteObliqueNURBSPlanarAssemblySource(ctx, &so, sm, Limits{}); e != nil {
		t.Fatal(e)
	}
	for _, b := range [][]byte{raw, out.Bytes(), shifted, so.Bytes()} {
		path := filepath.Join(t.TempDir(), "assembly.step")
		if e = os.WriteFile(path, b, 0600); e != nil {
			t.Fatal(e)
		}
		c, cancel := context.WithTimeout(ctx, 30*time.Second)
		result, e := exec.CommandContext(c, bin, "read", path).CombinedOutput()
		cancel()
		if e != nil {
			t.Fatalf("%v: %s", e, result)
		}
		var q struct {
			Volume, Area  float64
			Solids, Faces int
			Centroid      [3]float64
			Colors        [][3]float64
		}
		at := bytes.IndexByte(result, '{')
		if at < 0 || json.Unmarshal(result[at:], &q) != nil {
			t.Fatal(string(result))
		}
		if math.Abs(q.Volume-volume) > 1e-9 || math.Abs(q.Area-area) > 1e-9 || q.Solids != 3 || q.Faces != 18 || math.Abs(q.Centroid[0]-center.X) > 1e-9 || math.Abs(q.Centroid[1]-center.Y) > 1e-9 || math.Abs(q.Centroid[2]-center.Z) > 1e-9 || len(q.Colors) != 2 {
			t.Fatal(q)
		}
	}
}
func FuzzObliqueNURBSPlanarAssembly(f *testing.F) {
	f.Add(obliqueNURBSFixture(f))
	f.Add([]byte("ISO-10303-21;"))
	f.Fuzz(func(t *testing.T, b []byte) {
		if len(b) > 1<<20 {
			t.Skip()
		}
		m, e := ReadObliqueNURBSPlanarAssembly(context.Background(), bytes.NewReader(b), Limits{MaxEntities: 1024})
		if e != nil {
			if m.original != "" || len(m.Geometry.ProductBodies) != 0 {
				t.Fatal("partial failure")
			}
			return
		}
		var out bytes.Buffer
		if e = WriteObliqueNURBSPlanarAssemblySource(context.Background(), &out, m, Limits{}); e != nil || !bytes.Equal(b, out.Bytes()) {
			t.Fatal("source instability", e)
		}
	})
}
