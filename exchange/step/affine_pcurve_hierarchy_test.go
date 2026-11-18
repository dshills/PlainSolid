package step

import (
	"bytes"
	"context"
	"encoding/json"
	"math"
	"math/big"
	"os"
	"os/exec"
	"path/filepath"
	"plainsolid/geom"
	"strconv"
	"testing"
)

func TestAffinePCurveHierarchyOriginal(t *testing.T) {
	t.Parallel()
	for _, vendor := range []bool{false, true} {
		name := "rational-spherical-hierarchy-affine-pcurve-holed.step"
		if vendor {
			name = "occt-" + name
		}
		b, e := os.ReadFile("testdata/" + name)
		if e != nil {
			t.Fatal(e)
		}
		if _, e = readRationalSphericalHierarchy(context.Background(), bytes.NewReader(b), Limits{}, 1e-9, vendor); e == nil {
			t.Fatal("old API admitted spline pcurve")
		}
		m, e := readAffinePCurveHierarchy(context.Background(), bytes.NewReader(b), Limits{}, 1e-9, vendor)
		if e != nil {
			t.Fatal(name, e)
		}
		t.Log(name, len(m.evidence))
		if m.original != string(b) || len(m.evidence) == 0 {
			t.Fatal("source")
		}
	}
}

func TestAffinePCurveHierarchyPersistenceAndQueries(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	for _, vendor := range []bool{false, true} {
		name := "rational-spherical-hierarchy-affine-pcurve-holed.step"
		if vendor {
			name = "occt-" + name
		}
		raw, _ := os.ReadFile("testdata/" + name)
		m, e := readAffinePCurveHierarchy(ctx, bytes.NewReader(raw), Limits{}, 1e-9, vendor)
		if e != nil {
			t.Fatal(e)
		}
		var native bytes.Buffer
		if e = WriteAffinePCurveHierarchyJSON(ctx, &native, m); e != nil {
			t.Fatal(e)
		}
		again, e := ReadAffinePCurveHierarchyJSON(ctx, bytes.NewReader(native.Bytes()), IndependentMaterialJSONLimits{})
		if e != nil {
			t.Fatal(e)
		}
		var stable bytes.Buffer
		if e = WriteAffinePCurveHierarchyJSON(ctx, &stable, again); e != nil || !bytes.Equal(native.Bytes(), stable.Bytes()) {
			t.Fatal("unstable", e)
		}
		proof, e := again.ProofHierarchy(ctx)
		if e != nil {
			t.Fatal(e)
		}
		index := -1
		for k, u := range proof.Instances {
			if proof.Products[u.ProductID].Planar != nil {
				index = k
			}
		}
		if index < 0 {
			t.Fatal("planar")
		}
		u := proof.Instances[index]
		frame, _ := geom.NewFrame(geom.Vec3{}, geom.Vec3{X: 1}, geom.Vec3{Z: 1})
		origin, direction := hierarchyTestWorldLine(u, frame, [3]int64{-11, -2, -2}, [3]int64{1, 0, 0})
		line, e := again.ClipLine(ctx, origin, direction, big.NewRat(0, 1), big.NewRat(6, 1), RationalHierarchyLineOptions{})
		if e != nil {
			t.Fatal(e)
		}
		leaf := line.Leaves[index]
		if !leaf.Supported || len(leaf.BoundarySpans) != 2 || len(leaf.Spans) != 0 {
			t.Fatal("hole spans", leaf)
		}
		for i, s := range leaf.BoundarySpans {
			want := [][2]int64{{1, 2}, {4, 5}}[i]
			for j, root := range []*geom.ExactAlgebraicRoot{s.Lower, s.Upper} {
				r, _, _ := geom.NewExactRationalRoot(ctx, big.NewRat(want[j], 1), geom.ExactCurvePlaneEventOptions{})
				c, e := root.Compare(ctx, r, geom.ExactCurvePlaneEventOptions{})
				if e != nil || !c.Proven || c.Order != 0 {
					t.Fatal("span", e)
				}
			}
			if len(s.SourceFaceIDs) != 1 {
				t.Fatal("owner")
			}
		}
		point, _ := u.ApplyExact(geom.Vec3{X: -8, Y: -2, Z: 0})
		locations, e := again.LocatePoint(ctx, point, RationalHierarchyPointOptions{})
		if e != nil || locations.Leaves[index].Location != "outside" {
			t.Fatal("hole point", e)
		}
		evidence := again.AffinePCurves()
		evidence[0].CurveID = -1
		if again.AffinePCurves()[0].CurveID < 0 {
			t.Fatal("mutable evidence")
		}
		proof.Source.Entities = nil // returned proof owns all mutable data
		if _, e = again.LocatePoint(ctx, point, RationalHierarchyPointOptions{}); e != nil {
			t.Fatal("aliased proof", e)
		}
		for _, field := range []string{"evidence", "source", "native", "convention", "format"} {
			var q affinePCurveHierarchyEnvelope
			if e = json.Unmarshal(native.Bytes(), &q); e != nil {
				t.Fatal(e)
			}
			switch field {
			case "evidence":
				q.Evidence[0].Derivative[0] = "999"
			case "source":
				q.OriginalSTEP = "invalid"
			case "native":
				q.Proof.Instances[0].Matrix[0][0] = "999"
			case "convention":
				q.OCCTFrameConvention = !q.OCCTFrameConvention
			case "format":
				q.Format = rationalSphericalHierarchyFormat
			}
			b, _ := json.Marshal(q)
			if got, e := ReadAffinePCurveHierarchyJSON(ctx, bytes.NewReader(b), IndependentMaterialJSONLimits{}); e == nil || got.OriginalSTEP() != "" {
				t.Fatal("tamper", field, e)
			}
		}
		if _, e = ReadRationalSphericalHierarchyJSON(ctx, bytes.NewReader(native.Bytes()), IndependentMaterialJSONLimits{}); e == nil {
			t.Fatal("old tag")
		}
		if _, e = ReadAffinePCurveHierarchyJSON(ctx, bytes.NewReader(native.Bytes()), IndependentMaterialJSONLimits{MaxBytes: 10}); e == nil {
			t.Fatal("byte budget")
		}
		if bin := os.Getenv("PLAINSOLID_OCCT_RATIONAL_HIERARCHY_LINE"); bin != "" {
			args := []string{"testdata/" + name}
			for _, v := range [][3]*big.Rat{origin, direction} {
				for _, q := range v {
					f, _ := q.Float64()
					args = append(args, strconv.FormatFloat(f, 'g', 17, 64))
				}
			}
			args = append(args, "0", "6")
			output, e := exec.Command(bin, args...).Output()
			if e != nil {
				t.Fatal(e)
			}
			var hits []float64
			if e = json.Unmarshal(output, &hits); e != nil {
				t.Fatal(e)
			}
			if len(hits) != 4 {
				t.Fatal("OCCT hits", hits)
			}
			for i, v := range []float64{1, 2, 4, 5} {
				if math.Abs(hits[i]-v) > 1e-7 {
					t.Fatal("OCCT station", hits)
				}
			}
			f, e := Parse(bytes.NewBufferString(again.modelSourceForTest(t)), Limits{})
			if e != nil {
				t.Fatal(e)
			}
			b, _ := mappedPlanarSourceBytes(f)
			path := filepath.Join(t.TempDir(), "proof.step")
			if e = os.WriteFile(path, b, 0600); e != nil {
				t.Fatal(e)
			}
			args[0] = path
			output, e = exec.Command(bin, args...).Output()
			if e != nil {
				t.Fatal(e)
			}
			var normalized []float64
			if e = json.Unmarshal(output, &normalized); e != nil || len(normalized) != len(hits) {
				t.Fatal("normalized", e)
			}
			for i, v := range hits {
				if math.Abs(normalized[i]-v) > 1e-7 {
					t.Fatal("normalized OCCT", normalized)
				}
			}
		}
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if _, e := ReadAffinePCurveRationalHierarchy(canceled, bytes.NewReader(nil), Limits{}, 1e-9); e != context.Canceled {
		t.Fatal(e)
	}
}
func (m AffinePCurveHierarchy) modelSourceForTest(t *testing.T) string {
	t.Helper()
	b, e := mappedPlanarSourceBytes(m.model.Source)
	if e != nil {
		t.Fatal(e)
	}
	return string(b)
}

func TestAffinePCurveHierarchyReject(t *testing.T) {
	t.Parallel()
	raw, _ := os.ReadFile("testdata/rational-spherical-hierarchy-affine-pcurve-holed.step")
	m, e := ReadAffinePCurveRationalHierarchy(context.Background(), bytes.NewReader(raw), Limits{}, 1e-9)
	if e != nil {
		t.Fatal(e)
	}
	c := m.evidence[0]
	for _, test := range []string{"pole", "knot", "sense", "plane", "degree", "shared", "orphan", "exponent"} {
		t.Run(test, func(t *testing.T) {
			f, e := Parse(bytes.NewReader(raw), Limits{})
			if e != nil {
				t.Fatal(e)
			}
			id := c.CurveID
			en := f.Entities[id]
			switch test {
			case "pole":
				id = c.PoleIDs[0]
				en = f.Entities[id]
				en.Records[0].Args[1].Items[0].Text = "99"
			case "knot":
				en.Records[0].Args[7].Items[0].Text = "-9"
			case "sense":
				id = c.EdgeID
				en = f.Entities[id]
				en.Records[0].Args[4].Text = "F"
			case "plane":
				id = c.PCurveID
				en = f.Entities[id]
				en.Records[0].Args[1].Ref = c.EdgeID
			case "degree":
				en.Records[0].Args[1].Text = "2"
			case "shared":
				en.Records[0].Args[2].Items[1].Ref = c.PoleIDs[0]
			case "orphan":
				id = 90000
				en = Entity{ID: id, Records: []Record{{Type: "CARTESIAN_POINT", Args: []Value{{Kind: String}, {Kind: Aggregate, Items: []Value{{Kind: Number, Text: "0"}, {Kind: Number, Text: "0"}, {Kind: Number, Text: "0"}}}}}}}
				f.Order = append(f.Order, id)
			case "exponent":
				en.Records[0].Args[7].Items[0].Text = "1.E-999999999"
			}
			f.Entities[id] = en
			b, e := mappedPlanarSourceBytes(f)
			if e != nil {
				t.Fatal(e)
			}
			if got, e := ReadAffinePCurveRationalHierarchy(context.Background(), bytes.NewReader(b), Limits{}, 1e-9); e == nil || got.original != "" {
				t.Fatal("accepted", test)
			}
		})
	}
}

func TestAffinePCurveJointDirectedEdgeReversal(t *testing.T) {
	t.Parallel()
	raw, _ := os.ReadFile("testdata/rational-spherical-hierarchy-affine-pcurve-holed.step")
	ctx := context.Background()
	m, e := ReadAffinePCurveRationalHierarchy(ctx, bytes.NewReader(raw), Limits{}, 1e-9)
	if e != nil {
		t.Fatal(e)
	}
	id := m.evidence[0].EdgeID
	f, _ := Parse(bytes.NewReader(raw), Limits{})
	edge := f.Entities[id]
	a := edge.Records[0].Args
	a[1], a[2] = a[2], a[1]
	if a[4].Text == "T" {
		a[4].Text = "F"
	} else {
		a[4].Text = "T"
	}
	f.Entities[id] = edge
	count := 0
	for k, en := range f.Entities {
		if len(en.Records) == 1 && en.Records[0].Type == "ORIENTED_EDGE" && en.Records[0].Args[3].Ref == id {
			if en.Records[0].Args[4].Text == "T" {
				en.Records[0].Args[4].Text = "F"
			} else {
				en.Records[0].Args[4].Text = "T"
			}
			f.Entities[k] = en
			count++
		}
	}
	if count != 2 {
		t.Fatal(count)
	}
	b, _ := mappedPlanarSourceBytes(f)
	got, e := ReadAffinePCurveRationalHierarchy(ctx, bytes.NewReader(b), Limits{}, 1e-9)
	if e != nil {
		t.Fatal(e)
	}
	if len(got.evidence) != len(m.evidence) {
		t.Fatal("ancestry")
	}
}

func FuzzAffinePCurveHierarchy(f *testing.F) {
	b, e := os.ReadFile("testdata/rational-spherical-hierarchy-affine-pcurve-holed.step")
	if e != nil {
		f.Fatal(e)
	}
	f.Add(b)
	f.Add([]byte("invalid"))
	f.Fuzz(func(t *testing.T, b []byte) {
		if len(b) > 1<<17 {
			t.Skip()
		}
		ctx := context.Background()
		m, e := ReadAffinePCurveRationalHierarchy(ctx, bytes.NewReader(b), Limits{MaxBytes: 1 << 17, MaxEntities: 2048, MaxValues: 32768}, 1e-9)
		if e != nil {
			if m.original != "" {
				t.Fatal("partial result")
			}
			return
		}
		if m.original != string(b) || len(m.evidence) == 0 {
			t.Fatal("unbound")
		}
		var w bytes.Buffer
		if e = WriteAffinePCurveHierarchyJSON(ctx, &w, m); e != nil {
			t.Fatal(e)
		}
		if _, e = ReadAffinePCurveHierarchyJSON(ctx, bytes.NewReader(w.Bytes()), IndependentMaterialJSONLimits{}); e != nil {
			t.Fatal(e)
		}
	})
}
