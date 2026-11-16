package step

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"testing"

	"plainsolid/brep/parametric"
	"plainsolid/brep/trimmed"
	"plainsolid/geom"
)

func TestReadConicalRimFilletBoreSTEP(t *testing.T) {
	ctx := context.Background()
	raw := conicalRimFilletBoreFixture(t, "exact")
	p, err := ReadConicalRimFilletBore(ctx, bytes.NewReader(raw), Limits{}, geom.SurfacePatchOptions{})
	if err != nil {
		t.Fatal(err)
	}
	top := p.Solid.Topology()
	if p.LengthUnit != .001 || top.Vertices != 24 || top.Edges != 28 || top.Faces != 6 ||
		len(top.Shells) != 1 || top.InnerWires != 2 || top.EulerCharacteristic != 0 {
		t.Fatalf("wrong original unit or native topology: %g %v", p.LengthUnit, top)
	}
	if p.SourceBodyID != 15 || p.SourceShellID != 16 ||
		!reflect.DeepEqual(p.NativeFaceSourceIDs, []int64{17, 47, 99, 73, 119, 139}) {
		t.Fatalf("lost original face owners: %v", p.NativeFaceSourceIDs)
	}
	vertices := p.Solid.Vertices()
	original := make(map[[3]float64]bool)
	for i, id := range p.NativeVertexSourceIDs {
		if id == 0 {
			continue
		}
		rec, err := simple(p.Source, id, "VERTEX_POINT", 2)
		if err != nil {
			t.Fatal(err)
		}
		source, err := mappedExactCoordinate(p.Source, rec.Args[1].Ref, "CARTESIAN_POINT", 3)
		if err != nil || vertices[i] != source {
			t.Fatalf("original vertex #%d moved from %v to %v: %v", id, source, vertices[i], err)
		}
		original[[3]float64{source.X, source.Y, source.Z}] = true
	}
	uses := map[int64]int{}
	for _, id := range p.NativeEdgeSourceIDs {
		uses[id]++
	}
	if len(original) != 6 || len(uses) != 10 {
		t.Fatalf("lost original vertex or edge ownership: %v %v", original, uses)
	}
	for _, id := range []int64{28, 36, 62, 88, 106, 126} {
		if uses[id] != 4 {
			t.Fatalf("source full rim #%d lost its four quadrants", id)
		}
	}
	for _, id := range []int64{21, 51, 77, 144} {
		if uses[id] != 1 {
			t.Fatalf("original source seam #%d did not retain one native edge", id)
		}
	}
	mass, err := p.Solid.MassProperties(ctx, parametric.IntegrationOptions{RelativeTolerance: 1e-8})
	if err != nil || math.Abs(mass.Volume-2282.4014009763955) > 1e-7 || math.Abs(mass.Area-1182.6440976399522) > 1e-7 {
		t.Fatalf("native body differs from independent author: %v %v", mass, err)
	}
	var first, second bytes.Buffer
	if err := trimmed.WriteJSON(&first, p.Solid); err != nil {
		t.Fatal(err)
	}
	restored, err := trimmed.ReadJSONContext(ctx, bytes.NewReader(first.Bytes()), trimmed.JSONLimits{})
	if err != nil {
		t.Fatal(err)
	}
	if err := trimmed.WriteJSON(&second, restored); err != nil || !bytes.Equal(first.Bytes(), second.Bytes()) {
		t.Fatalf("native topology did not replay deterministically: %v", err)
	}
	if oracle := os.Getenv("PLAINSOLID_OCCT_CONICAL_RIM_BORE_STEP_READER"); oracle != "" {
		verifyConicalRimFilletBoreComparator(t, oracle, raw, original, mass)
	}
}

func TestReadConicalRimFilletBoreSTEPReject(t *testing.T) {
	ctx, o := context.Background(), geom.SurfacePatchOptions{}
	exact := conicalRimFilletBoreFixture(t, "exact")
	rounded := conicalRimFilletBoreFixture(t, "rounded")
	if _, err := ReadWrappedMaterials(bytes.NewReader(rounded), Limits{}, WrappedSurfaceOptions{}); err == nil {
		t.Fatal("existing wrapped reader unexpectedly accepted the actual default source")
	}
	for _, tc := range []struct {
		name, before, after string
		source              []byte
		limits              Limits
	}{
		{name: "actual default export", source: rounded},
		{name: "source face limit", source: exact, limits: Limits{MaxFaces: 5}},
		{name: "source vertex limit", source: exact, limits: Limits{MaxVertices: 5}},
		{name: "source edge-use limit", source: exact, limits: Limits{MaxEdgeUses: 19}},
		{name: "original incorrect lower bound", before: "#48 = FACE_BOUND('',#49,.T.);", after: "#48 = FACE_BOUND('',#49,.F.);"},
		{name: "torus leaves cone tangency", before: "#68 = TOROIDAL_SURFACE('',#69,7.6875,0.9375);", after: "#68 = TOROIDAL_SURFACE('',#69,7.75,0.9375);"},
		{name: "model arc leaves source torus", before: "#54 = CIRCLE('',#55,0.9375);", after: "#54 = CIRCLE('',#55,0.875);"},
		{name: "complementary obtuse upper arc", before: "#83 = DIRECTION('',(0.,-1.,0.));", after: "#83 = DIRECTION('',(0.,1.,0.));"},
		{name: "bore support off original cap holes", before: "CARTESIAN_POINT('',(1.5,4.,-1.));", after: "CARTESIAN_POINT('',(1.75,4.,-1.));"},
		{name: "unowned exact point", before: "ENDSEC;\nEND-ISO-10303-21;", after: "#303 = CARTESIAN_POINT('',(0.,0.,0.));\nENDSEC;\nEND-ISO-10303-21;"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			source := tc.source
			if source == nil {
				if bytes.Count(exact, []byte(tc.before)) != 1 {
					t.Fatalf("mutation did not identify one source entity: %s", tc.before)
				}
				source = bytes.Replace(exact, []byte(tc.before), []byte(tc.after), 1)
			}
			p, err := ReadConicalRimFilletBore(ctx, bytes.NewReader(source), tc.limits, o)
			if err == nil || p.Source != nil || p.Solid.Topology().Faces != 0 {
				t.Fatalf("accepted an unsupported source or returned partial material: %v", err)
			}
		})
	}
	canceled, stop := context.WithCancel(ctx)
	stop()
	if p, err := ReadConicalRimFilletBore(canceled, bytes.NewReader(exact), Limits{}, o); !errors.Is(err, context.Canceled) || p.Source != nil {
		t.Fatalf("canceled read returned material: %v", err)
	}
}

func TestConicalRimFilletBorePinnedSTEPAuthor(t *testing.T) {
	author := os.Getenv("PLAINSOLID_OCCT_CONICAL_RIM_BORE_STEP_AUTHOR")
	if author == "" {
		t.Skip("pinned OCCT conical rim author not configured")
	}
	dir := t.TempDir()
	exact, rounded := filepath.Join(dir, "exact.step"), filepath.Join(dir, "rounded.step")
	if output, err := exec.Command(author, exact, rounded).CombinedOutput(); err != nil {
		t.Fatalf("pinned author: %v\n%s", err, output)
	}
	for kind, path := range map[string]string{"exact": exact, "rounded": rounded} {
		generated, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		current := conicalRimFilletBoreFixture(t, kind)
		i, j := bytes.Index(generated, []byte("\nDATA;\n")), bytes.Index(current, []byte("\nDATA;\n"))
		if i < 0 || j < 0 || !bytes.Equal(generated[i:], current[j:]) {
			t.Fatalf("pinned author changed the %s DATA section", kind)
		}
	}
	// The exact carrier and the default export must use the same physical
	// model vertices, even though OCCT renumbers their directory IDs.
	vertices := func(path string) map[geom.Vec3]bool {
		t.Helper()
		generated, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		f, err := Parse(bytes.NewReader(generated), Limits{})
		if err != nil {
			t.Fatal(err)
		}
		points := map[geom.Vec3]bool{}
		for _, id := range f.Order {
			rec, err := simple(f, id, "VERTEX_POINT", 2)
			if err != nil {
				continue
			}
			point, err := mappedExactCoordinate(f, rec.Args[1].Ref, "CARTESIAN_POINT", 3)
			if err != nil {
				t.Fatal(err)
			}
			points[point] = true
		}
		return points
	}
	exactVertices, roundedVertices := vertices(exact), vertices(rounded)
	if len(exactVertices) != 6 || !reflect.DeepEqual(exactVertices, roundedVertices) {
		t.Fatalf("exact STEP carrier moved a source vertex: %v %v", exactVertices, roundedVertices)
	}
}

func conicalRimFilletBoreFixture(t *testing.T, kind string) []byte {
	t.Helper()
	data, err := os.ReadFile("testdata/occt-conical-double-rim-eccentric-bore-" + kind + ".step")
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func verifyConicalRimFilletBoreComparator(t *testing.T, oracle string, raw []byte, original map[[3]float64]bool, mass parametric.Properties) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "conical-rim-fillet.step")
	if err := os.WriteFile(path, raw, 0600); err != nil {
		t.Fatal(err)
	}
	data, err := exec.Command(oracle, path).CombinedOutput()
	var hit struct {
		Volume, Area float64
		Vertices     [][3]float64
		Edges        [3]int
		Faces        [][]float64
	}
	start := bytes.Index(data, []byte(`{"volume":`))
	if err != nil || start < 0 || json.Unmarshal(data[start:], &hit) != nil || hit.Edges != [3]int{6, 2, 2} ||
		hit.Volume <= 0 || math.Abs(hit.Volume-mass.Volume) > 1e-8*mass.Volume || math.Abs(hit.Area-mass.Area) > 1e-8*mass.Area ||
		len(hit.Vertices) != len(original) || len(hit.Faces) != 6 {
		t.Fatalf("pinned comparator failed signed solid transfer: %v\n%s", err, data)
	}
	for _, v := range hit.Vertices {
		if !original[v] {
			t.Fatalf("pinned STEP reopen changed an original vertex: %v", v)
		}
		delete(original, v)
	}
	want := [][]float64{{3, 0, 2, .1875, 8.25, 2.625, 11.5},
		{2, 0, 2, .9375, 7.6875, .9375, 0, .1875}, {2, 0, 2, 4.125, 10.375, 1.875, 2.625, 6},
		{0, 0, 2}, {0, 6, 2}, {1, 1.5, 4, 1, 0, 6}}
	for i, support := range want {
		if len(hit.Faces[i]) != len(support) {
			t.Fatalf("pinned STEP reopen changed source support %d: %v", i, hit.Faces[i])
		}
		for j, scalar := range support {
			if math.Abs(hit.Faces[i][j]-scalar) > 1e-10 {
				t.Fatalf("pinned STEP reopen changed source support %d: %v", i, hit.Faces[i])
			}
		}
	}
}
