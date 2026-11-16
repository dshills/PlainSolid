package step

import (
	"bytes"
	"context"
	"encoding/json"
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

func TestReadEccentricAnnularChamferSTEP(t *testing.T) {
	ctx := context.Background()
	options := geom.SurfacePatchOptions{}
	data := eccentricAnnularChamferSTEPFixture(t, "exact")
	p, err := ReadEccentricAnnularChamfer(ctx, bytes.NewReader(data), Limits{}, options)
	if err != nil {
		t.Fatal(err)
	}
	top := p.Solid.Topology()
	if p.LengthUnit != .001 || top.Vertices != 32 || top.Edges != 38 || top.Faces != 8 ||
		top.InnerWires != 2 || len(top.Shells) != 1 || top.EulerCharacteristic != 0 {
		t.Fatalf("wrong original unit or reconstructed topology: %g %v", p.LengthUnit, top)
	}
	if p.SourceBodyID != 15 || p.SourceShellID != 16 ||
		!reflect.DeepEqual(p.NativeFaceSourceIDs, []int64{47, 17, 69, 91, 111, 131, 175, 153}) {
		t.Fatalf("lost original face owners: %v", p.NativeFaceSourceIDs)
	}
	vertices := p.Solid.Vertices()
	var originalVertices []geom.Vec3
	for i, id := range p.NativeVertexSourceIDs {
		if id == 0 {
			continue
		}
		point, err := simple(p.Source, id, "VERTEX_POINT", 2)
		if err != nil {
			t.Fatal(err)
		}
		q, err := mappedExactCoordinate(p.Source, point.Args[1].Ref, "CARTESIAN_POINT", 3)
		if err != nil || q != vertices[i] {
			t.Fatalf("moved original vertex #%d: %v %v", id, q, vertices[i])
		}
		originalVertices = append(originalVertices, q)
	}
	counts := map[int64]int{}
	for _, id := range p.NativeEdgeSourceIDs {
		counts[id]++
	}
	if len(originalVertices) != 8 || len(counts) != 14 {
		t.Fatalf("lost original vertices or edges: %v %v", originalVertices, counts)
	}
	for _, id := range []int64{28, 36, 56, 78, 98, 118, 142, 164} {
		if counts[id] != 4 {
			t.Fatalf("source rim #%d did not supply four exact quadrants", id)
		}
	}
	for _, id := range []int64{21, 51, 73, 135, 157, 179} {
		if counts[id] != 1 {
			t.Fatalf("source generator #%d did not retain a single native seam", id)
		}
	}
	mass, err := p.Solid.MassProperties(ctx, parametric.IntegrationOptions{RelativeTolerance: 1e-8})
	if err != nil || math.Abs(mass.Volume-726.9882656101975) > 1e-7 || math.Abs(mass.Area-593.897326906695) > 1e-7 {
		t.Fatalf("signed material differs from independent author: %v, %v", mass, err)
	}
	var saved, canonical, repeated bytes.Buffer
	if err := trimmed.WriteJSON(&saved, p.Solid); err != nil {
		t.Fatal(err)
	}
	restored, err := trimmed.ReadJSONContext(ctx, &saved, trimmed.JSONLimits{})
	if err != nil {
		t.Fatal(err)
	}
	if err := WriteEccentricAnnularChamfer(ctx, &canonical, p.Solid, p.Name, p.LengthUnit, options); err != nil {
		t.Fatal(err)
	}
	if err := WriteEccentricAnnularChamfer(ctx, &repeated, restored, p.Name, p.LengthUnit, options); err != nil ||
		!bytes.Equal(canonical.Bytes(), repeated.Bytes()) || bytes.Count(canonical.Bytes(), []byte("SURFACE_OF_REVOLUTION(")) != 6 {
		t.Fatalf("native replay lost the exact six-band canonical output: %v", err)
	}
	if oracle := os.Getenv("PLAINSOLID_OCCT_ECCENTRIC_ANNULAR_IMPORT_READER"); oracle != "" {
		verifyEccentricAnnularChamferSTEPConsumer(t, oracle, data, originalVertices, 14, mass, "--full-rims")
		verifyEccentricAnnularChamferSTEPConsumer(t, oracle, canonical.Bytes(), vertices, 38, mass)
	}
}

func TestReadEccentricAnnularChamferSTEPReject(t *testing.T) {
	ctx := context.Background()
	exact := eccentricAnnularChamferSTEPFixture(t, "exact")
	rounded := eccentricAnnularChamferSTEPFixture(t, "rounded")
	for _, tc := range []struct {
		name, before, after string
		source              []byte
		limits              Limits
	}{
		{name: "default rounded OCCT", source: rounded},
		{name: "too few faces", source: exact, limits: Limits{MaxFaces: 7}},
		{name: "too few vertices", source: exact, limits: Limits{MaxVertices: 7}},
		{name: "too few original edge uses", source: exact, limits: Limits{MaxEdgeUses: 27}},
		{name: "reversed shell plane", before: "#106,.F.);", after: "#106,.T.);"},
		{name: "one seam use reversed", before: "#34 = ORIENTED_EDGE('',*,*,#21,.F.);", after: "#34 = ORIENTED_EDGE('',*,*,#21,.T.);"},
		{name: "wrong seam generator", before: "#22,#24,#26,.T.);", after: "#22,#24,#54,.T.);"},
		{name: "off support source rim radius", before: "#29 = CIRCLE('',#30,6.75);", after: "#29 = CIRCLE('',#30,6.75000095367431640625);"},
		{name: "unequal generator radius", before: "#43 = B_SPLINE_CURVE_WITH_KNOTS('',1,(#23,#25)", after: "#43 = B_SPLINE_CURVE_WITH_KNOTS('',1,(#75,#25)"},
		{name: "off-axis revolution", before: "#45 = CARTESIAN_POINT('',(0.,0.,2.));", after: "#45 = CARTESIAN_POINT('',(0.0625,0.,2.));"},
		{name: "unowned entity", before: "ENDSEC;\nEND-ISO-10303-21;", after: "#300 = CARTESIAN_POINT('',(0.,0.,0.));\nENDSEC;\nEND-ISO-10303-21;"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			data := tc.source
			if data == nil {
				if bytes.Count(exact, []byte(tc.before)) != 1 {
					t.Fatalf("mutation no longer targets a unique source entity: %s", tc.before)
				}
				data = bytes.Replace(exact, []byte(tc.before), []byte(tc.after), 1)
			}
			p, err := ReadEccentricAnnularChamfer(ctx, bytes.NewReader(data), tc.limits, geom.SurfacePatchOptions{})
			if err == nil || p.Source != nil || p.Solid.Topology().Faces != 0 {
				t.Fatalf("accepted unsupported source or exposed partial native material: %v", err)
			}
		})
	}
}

func TestEccentricAnnularChamferPinnedSTEPAuthor(t *testing.T) {
	author := os.Getenv("PLAINSOLID_OCCT_ECCENTRIC_ANNULAR_EXACT_AUTHOR")
	if author == "" {
		t.Skip("pinned OCCT exact STEP author not configured")
	}
	dir := t.TempDir()
	exact, rounded := filepath.Join(dir, "exact.step"), filepath.Join(dir, "rounded.step")
	if out, err := exec.Command(author, exact, rounded).CombinedOutput(); err != nil {
		t.Fatalf("pinned OCCT author: %v\n%s", err, out)
	}
	for name, path := range map[string]string{"exact": exact, "rounded": rounded} {
		got, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		want := eccentricAnnularChamferSTEPFixture(t, name)
		// FILE_NAME captures the writer's current time. The entire DATA
		// section must reproduce byte for byte from the pinned author.
		i, j := bytes.Index(got, []byte("\nDATA;\n")), bytes.Index(want, []byte("\nDATA;\n"))
		if i < 0 || j < 0 || !bytes.Equal(got[i:], want[j:]) {
			t.Fatalf("pinned %s author generated a different source entity graph", name)
		}
	}
}

func eccentricAnnularChamferSTEPFixture(t *testing.T, kind string) []byte {
	t.Helper()
	data, err := os.ReadFile("testdata/occt-eccentric-annular-chamfer-" + kind + ".step")
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func verifyEccentricAnnularChamferSTEPConsumer(t *testing.T, oracle string, data []byte, vertices []geom.Vec3,
	edgeCount int, mass parametric.Properties, options ...string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "annular-chamfer.step")
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	raw, err := exec.Command(oracle, append([]string{path}, options...)...).CombinedOutput()
	var hit struct {
		Solids, Faces int
		Volume, Area  float64
		Vertices      [][3]float64
		Edges         [][3]int
		Caps          [][2]float64
		Bands         [][6]float64
	}
	first := bytes.Index(raw, []byte(`{"Solids":`))
	if err != nil || first < 0 || json.Unmarshal(raw[first:], &hit) != nil || hit.Solids != 1 || hit.Faces != 8 ||
		len(hit.Vertices) != len(vertices) || len(hit.Edges) != edgeCount || hit.Volume <= 0 ||
		math.Abs(hit.Volume-mass.Volume) > 1e-8*mass.Volume || math.Abs(hit.Area-mass.Area) > 1e-8*mass.Area {
		t.Fatalf("pinned OCCT consumer did not retain signed material: %v\n%s", err, raw)
	}
	exactVertices := map[[3]float64]bool{}
	for _, p := range vertices {
		exactVertices[[3]float64{p.X, p.Y, p.Z}] = true
	}
	for _, p := range hit.Vertices {
		if !exactVertices[p] {
			t.Fatalf("pinned consumer lost exact vertex %v", p)
		}
		delete(exactVertices, p)
	}
	if !reflect.DeepEqual(hit.Caps, [][2]float64{{2, 2}, {10, 2}}) || len(hit.Bands) != 6 {
		t.Fatalf("pinned consumer changed original cap or wall partition: %v", hit)
	}
	remaining := [][6]float64{{0, 0, 2, 4.75, 4, 5.5}, {0, 0, 4, 5.5, 9, 6.75}, {0, 0, 9, 6.75, 10, 6.5},
		{1.5, 2, 2, 2.1875, 3, 2.125}, {1.5, 2, 3, 2.125, 8, 2.75}, {1.5, 2, 8, 2.75, 10, 3.375}}
	for _, got := range hit.Bands {
		match := -1
		for i, want := range remaining {
			ok := true
			for j := range want {
				ok = ok && math.Abs(got[j]-want[j]) < 1e-10
			}
			if ok {
				match = i
				break
			}
		}
		if match < 0 {
			t.Fatalf("changed bounded conical support: %v", got)
		}
		remaining = append(remaining[:match], remaining[match+1:]...)
	}
}
