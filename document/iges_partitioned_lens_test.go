package document

import (
	"bytes"
	"context"
	"encoding/json"
	"math"
	"math/big"
	"os"
	"os/exec"
	"path/filepath"
	"plainsolid/exchange/iges"
	"reflect"
	"strings"
	"testing"
	"time"
)

func partitionedIGESDocument(t *testing.T, name string) *Document {
	t.Helper()
	raw, e := os.ReadFile("../exchange/iges/testdata/" + name + ".igs")
	if e != nil {
		t.Fatal(e)
	}
	d, e := ReadPartitionedSplineLensIGES(bytes.NewReader(raw), iges.Limits{}, 2)
	if e != nil {
		t.Fatal(e)
	}
	return d
}
func TestPartitionedIGESDocumentChain(t *testing.T) {
	for _, name := range []string{"partitioned-lens", "concurrent-partitioned-lens", "sheared-partitioned-lens"} {
		t.Run(name, func(t *testing.T) {
			raw, e := os.ReadFile("../exchange/iges/testdata/" + name + ".igs")
			if e != nil {
				t.Fatal(e)
			}
			original, e := iges.ReadPartitionedRationalSplineLensSolid(bytes.NewReader(raw), iges.Limits{})
			if e != nil {
				t.Fatal(e)
			}
			d := partitionedIGESDocument(t, name)
			s, _ := d.Snapshot()
			recipe := s.Labels["iges:body-000001"].Attributes["iges.lens.partitioned.recipe"].Text
			s.LengthUnit = .0254
			d, e = New(s, 2)
			if e != nil {
				t.Fatal(e)
			}
			var native bytes.Buffer
			if e = d.Write(&native); e != nil {
				t.Fatal(e)
			}
			d, e = Read(&native, 0, 2)
			if e != nil {
				t.Fatal(e)
			}
			s, _ = d.Snapshot()
			if recipe != s.Labels["iges:body-000001"].Attributes["iges.lens.partitioned.recipe"].Text {
				t.Fatal("recipe lost")
			}
			var out bytes.Buffer
			if e = s.WritePartitionedSplineLensIGES(&out); e != nil {
				t.Fatal(e)
			}
			got, e := iges.ReadPartitionedRationalSplineLensSolid(bytes.NewReader(out.Bytes()), iges.Limits{})
			if e != nil {
				t.Fatal(e)
			}
			if got.Body.LengthUnit != .0254 || !reflect.DeepEqual(original.Curves, got.Curves) || !reflect.DeepEqual(original.Body.Upper.Data(), got.Body.Upper.Data()) || !reflect.DeepEqual(original.Body.Lower.Data(), got.Body.Lower.Data()) {
				t.Fatal("source basis changed")
			}
			for i, p := range original.Pieces {
				q := got.Pieces[i]
				if p.Quarter != q.Quarter || p.Start.Cmp(q.Start) != 0 || p.End.Cmp(q.End) != 0 || p.Cells != q.Cells {
					t.Fatal("partition changed")
				}
			}
			out.Reset()
			if e = s.WriteSplineLensIGES(&out); e == nil || out.Len() != 0 {
				t.Fatal("ordinary export lost partition")
			}
			if e = s.WriteRationalSplineLensIGES(&out); e == nil || out.Len() != 0 {
				t.Fatal("four-edge export lost partition")
			}
			d, e = FromPartitionedSplineLensIGES(got, 2)
			if e != nil {
				t.Fatal(e)
			}
			s, _ = d.Snapshot()
			if e = s.WritePartitionedSplineLensIGES(&out); e != nil {
				t.Fatal(e)
			}
		})
	}
}
func TestPartitionedIGESDocumentReject(t *testing.T) {
	for _, kind := range []string{"cut", "cell", "id", "model", "weight", "owner", "extra-cell", "extra-id", "extra-owner", "oversize", "huge-rational", "geometry", "placement", "instances"} {
		t.Run(kind, func(t *testing.T) {
			d := partitionedIGESDocument(t, "concurrent-partitioned-lens")
			s, _ := d.Snapshot()
			label := s.Labels["iges:body-000001"]
			var p partitionedIGESRecipe
			if e := json.Unmarshal([]byte(label.Attributes["iges.lens.partitioned.recipe"].Text), &p); e != nil {
				t.Fatal(e)
			}
			switch kind {
			case "cut":
				p.Pieces[0].End = big.NewRat(2, 3)
			case "cell":
				p.Pieces[0].Cells[0].UMax = .8
			case "id":
				p.Pieces[0].ParameterCurveIDs[0] = p.Pieces[0].ModelCurveID
			case "model":
				p.Curves[0][0].Poles[1].Z = math.Nextafter(2, 3)
			case "weight":
				p.Curves[0][0].Weights[0] = 0
			case "owner":
				p.Curves[0][1] = p.Curves[1][1]
			case "extra-cell":
				p.Pieces[0].Cells = append(p.Pieces[0].Cells, p.Pieces[0].Cells[0])
			case "extra-id":
				p.Pieces[0].ParameterCurveIDs = append(p.Pieces[0].ParameterCurveIDs, 999)
			case "extra-owner":
				p.Curves[0] = append(p.Curves[0], p.Curves[0][0])
			case "oversize":
				p.Geometry = strings.Repeat("x", maxPartitionedIGESRecipeBytes+1)
			case "huge-rational":
				p.Pieces[0].End = new(big.Rat).SetFrac(big.NewInt(1), new(big.Int).Lsh(big.NewInt(1), 9000))
			case "geometry":
				d2 := igesLensDocument(t, "sheared-spline-lens")
				s2, _ := d2.Snapshot()
				s.Trimmed["body-000001"] = s2.Trimmed["body-000001"]
			case "placement":
				n := s.Nodes["body-000001"]
				n.Transform[0][3] = 1
				s.Nodes["body-000001"] = n
			case "instances":
				s.Nodes["extra"] = s.Nodes["body-000001"]
			}
			raw, e := json.Marshal(p)
			if e != nil {
				t.Fatal(e)
			}
			label.Attributes["iges.lens.partitioned.recipe"] = Attribute{Kind: "string", Text: string(raw)}
			label.Attributes["iges.lens.partitioned.binding"] = Attribute{Kind: "string", Text: lensProvenanceDigest(raw)}
			s.Labels["iges:body-000001"] = label
			var out bytes.Buffer
			if e = s.WritePartitionedSplineLensIGES(&out); e == nil || out.Len() != 0 {
				t.Fatal("stale recipe emitted")
			}
		})
	}
}
func TestPartitionedIGESDocumentOCCT(t *testing.T) {
	oracle := os.Getenv("PLAINSOLID_OCCT_ORACLE")
	if oracle == "" || os.Getenv("PLAINSOLID_OCCT_IGES") == "" {
		t.Skip("optional OCCT")
	}
	for _, name := range []string{"partitioned-lens", "concurrent-partitioned-lens", "sheared-partitioned-lens"} {
		t.Run(name, func(t *testing.T) {
			d := partitionedIGESDocument(t, name)
			var native bytes.Buffer
			if e := d.Write(&native); e != nil {
				t.Fatal(e)
			}
			d, e := Read(&native, 0, 2)
			if e != nil {
				t.Fatal(e)
			}
			s, _ := d.Snapshot()
			var b bytes.Buffer
			if e = s.WritePartitionedSplineLensIGES(&b); e != nil {
				t.Fatal(e)
			}
			path := filepath.Join(t.TempDir(), "native.igs")
			if e = os.WriteFile(path, b.Bytes(), 0600); e != nil {
				t.Fatal(e)
			}
			out := filepath.Join(t.TempDir(), "mass.json")
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			if log, e := exec.CommandContext(ctx, oracle, out, "iges-solid", path).CombinedOutput(); e != nil {
				t.Fatal(e, string(log))
			}
			raw, e := os.ReadFile(out)
			if e != nil {
				t.Fatal(e)
			}
			var r struct {
				Solids, Faces, Edges, Vertices int
				Volume                         float64
			}
			n, radius := 6, 15./64
			if name == "concurrent-partitioned-lens" {
				n, radius = 7, 263340./4194304
			}
			if e = json.Unmarshal(raw, &r); e != nil || r.Solids != 1 || r.Faces != 2 || r.Edges != n || r.Vertices != n || math.Abs(r.Volume-math.Pi*math.Pow(radius, 4)/2) > 1e-10 {
				t.Fatal(r, e)
			}
		})
	}
}
