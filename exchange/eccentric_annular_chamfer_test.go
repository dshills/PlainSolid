package exchange_test

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
	"plainsolid/internal/slowtest"
	"slices"
	"testing"

	"plainsolid/brep/parametric"
	"plainsolid/brep/trimmed"
	"plainsolid/csg/fillet"
	"plainsolid/exchange/iges"
	"plainsolid/exchange/step"
	"plainsolid/geom"
)

func TestEccentricAnnularChamferNeutralTransfer(t *testing.T) {
	slowtest.Skip(t)
	ctx := context.Background()
	o := geom.SurfacePatchOptions{}
	wall := func(center geom.Vec3, lower, upper float64) *geom.WrappedSurfacePatch {
		t.Helper()
		w, err := geom.NewCanonicalWrappedCylinder(ctx, center, lower, 8, o)
		if err != nil {
			t.Fatal(err)
		}
		w, _, err = geom.DraftWrappedFrustum(ctx, w, 0, upper-lower, o)
		if err != nil {
			t.Fatal(err)
		}
		return w
	}
	outer := wall(geom.Vec3{Z: 2}, 5, 7)
	picks := fillet.CircularTubeChamfers{
		Outer: [2]fillet.CircularRimChamfer{{Radial: .25, Axial: 2}, {Radial: .5, Axial: 1}},
		Inner: [2]fillet.CircularRimChamfer{{Radial: .1875, Axial: 1}, {Radial: .375, Axial: 2}},
	}
	formats := []struct {
		suffix string
		write  func(io.Writer, trimmed.Solid) error
	}{
		{"step", func(w io.Writer, s trimmed.Solid) error {
			return step.WriteEccentricAnnularChamfer(ctx, w, s, "eccentric four rim chamfers", .001, o)
		}},
		{"igs", func(w io.Writer, s trimmed.Solid) error { return iges.WriteEccentricAnnularChamfer(ctx, w, s, .001, o) }},
	}
	for _, c := range []struct {
		name string
		axis geom.Vec2
		args []string
	}{
		{"aligned", geom.Vec2{X: 2.5}, nil},
		{"diagonal", geom.Vec2{X: 1.5, Y: 2}, []string{"diagonal"}},
	} {
		t.Run(c.name, func(t *testing.T) {
			source, _, err := trimmed.NewWrappedEccentricCircularFrustumBore(ctx, outer, wall(geom.Vec3{X: c.axis.X, Y: c.axis.Y, Z: 2}, 2, 3), 1e-9, o)
			if err != nil {
				t.Fatal(err)
			}
			result, err := fillet.ChamferWrappedEccentricFrustumTubeRims(ctx, source, picks, o)
			if err != nil {
				t.Fatal(err)
			}
			var persisted bytes.Buffer
			if err := trimmed.WriteJSON(&persisted, result.Solid); err != nil {
				t.Fatal(err)
			}
			native, err := trimmed.ReadJSONContext(ctx, &persisted, trimmed.JSONLimits{})
			if err != nil {
				t.Fatal(err)
			}
			mass, err := native.MassProperties(ctx, parametric.IntegrationOptions{RelativeTolerance: 1e-8})
			if err != nil {
				t.Fatal(err)
			}
			var rims [][4]float64
			if author := os.Getenv("PLAINSOLID_OCCT_ECCENTRIC_RIM_CHAMFER_AUTHOR"); author != "" {
				data, err := exec.CommandContext(ctx, author, c.args...).CombinedOutput()
				var got struct {
					Solids, Faces int
					Volume, Area  float64
					Rims          [][4]float64
				}
				if err != nil || json.Unmarshal(data, &got) != nil || got.Solids != 1 || got.Faces != 8 || len(got.Rims) != 8 ||
					math.Abs(got.Volume-mass.Volume) > 1e-7 || math.Abs(got.Area-mass.Area) > 1e-7 {
					t.Fatalf("independent OCCT BRepFilletAPI chamfer: %v\n%s", err, data)
				}
				rims = got.Rims
			}
			if c.name == "diagonal" {
				// An independently constructed, valid result with only three
				// sloped rims remains outside the four-chamfer exchange proof.
				partial := picks
				partial.Inner[0] = fillet.CircularRimChamfer{}
				other, err := fillet.ChamferWrappedEccentricFrustumTubeRims(ctx, source, partial, o)
				if err != nil {
					t.Fatal(err)
				}
				for _, f := range formats {
					var out bytes.Buffer
					if err := f.write(&out, other.Solid); !errors.Is(err, geom.ErrConvergence) || out.Len() != 0 {
						t.Fatalf("writer %s exposed a partial unsupported result: %v", f.suffix, err)
					}
				}
			}
			for _, f := range formats {
				t.Run(f.suffix, func(t *testing.T) {
					var out, repeated bytes.Buffer
					if err := f.write(&out, result.Solid); err != nil {
						t.Fatal(err)
					}
					if err := f.write(&repeated, native); err != nil || !bytes.Equal(out.Bytes(), repeated.Bytes()) {
						t.Fatalf("native replay changed exact neutral bytes: %v", err)
					}
					if f.suffix == "step" && (bytes.Count(out.Bytes(), []byte("SURFACE_OF_REVOLUTION(")) != 10 ||
						bytes.Contains(out.Bytes(), []byte("CONICAL_SURFACE("))) {
						t.Fatal("did not transfer all original generators without angle rounding")
					}
					if oracle := os.Getenv("PLAINSOLID_OCCT_ECCENTRIC_ANNULAR_TRANSFER"); oracle != "" {
						checkEccentricAnnularOCCTTransfer(t, ctx, oracle, f.suffix, out.Bytes(), native, c.axis, rims, mass)
					}
				})
			}
		})
	}
}

func checkEccentricAnnularOCCTTransfer(t *testing.T, ctx context.Context, oracle, ext string, data []byte, native trimmed.Solid,
	inner geom.Vec2, original [][4]float64, mass parametric.Properties) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "eccentric-annulus."+ext)
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	raw, err := exec.CommandContext(ctx, oracle, path).CombinedOutput()
	var hit struct {
		Solids, Faces int
		Volume, Area  float64
		Vertices      [][3]float64
		Edges         [][3]int
		Caps          [][2]float64
		Bands         [][6]float64
	}
	from := bytes.Index(raw, []byte(`{"Solids":`))
	if err != nil || from < 0 || json.Unmarshal(raw[from:], &hit) != nil {
		t.Fatalf("pinned OCCT annular chamfer transfer: %v\n%s", err, raw)
	}
	top := native.Topology()
	if hit.Solids != 1 || hit.Faces != top.Faces || hit.Faces != 12 || len(hit.Vertices) != top.Vertices ||
		len(hit.Edges) != top.Edges || hit.Volume <= 0 || math.Abs(hit.Volume-mass.Volume) > 1e-8*mass.Volume ||
		math.Abs(hit.Area-mass.Area) > 1e-8*mass.Area {
		t.Fatalf("invalid signed material or changed native topology: %s", raw)
	}
	v := native.Vertices()
	ids := make(map[[3]float64]int, len(v))
	for i, p := range v {
		ids[[3]float64{p.X, p.Y, p.Z}] = i
	}
	mapped := make([]int, len(hit.Vertices))
	for i, p := range hit.Vertices {
		id, ok := ids[p]
		if !ok {
			t.Fatalf("pinned reader created or moved a source vertex: %v", p)
		}
		mapped[i] = id
		delete(ids, p)
	}
	pairs := map[[2]int]int{}
	for _, edge := range native.Edges() {
		kind := 0
		if v[edge.Start].Z == v[edge.End].Z {
			kind = 1
		}
		pairs[[2]int{min(edge.Start, edge.End), max(edge.Start, edge.End)}] = kind
	}
	for _, edge := range hit.Edges {
		if edge[0] < 0 || edge[1] < 0 || edge[0] >= len(mapped) || edge[1] >= len(mapped) {
			t.Fatal("an original edge lost a model vertex")
		}
		a, b := mapped[edge[0]], mapped[edge[1]]
		key := [2]int{min(a, b), max(a, b)}
		if k, ok := pairs[key]; !ok || k != edge[2] {
			t.Fatalf("changed analytic circle or generator incidence: %v", edge)
		}
		delete(pairs, key)
	}
	if len(hit.Caps) != 2 || !slices.Contains(hit.Caps, [2]float64{2, 2}) || !slices.Contains(hit.Caps, [2]float64{10, 2}) {
		t.Fatalf("annular cap wires changed: %v", hit.Caps)
	}
	stations := [6]float64{2, 3, 4, 8, 9, 10}
	radii := [2][6]float64{{4.75, 5.125, 5.5, 6.5, 6.75, 6.5}, {2.1875, 2.125, 2.25, 2.75, 3.0625, 3.375}}
	remaining := make([][6]float64, 0, 10)
	for side, axis := range []geom.Vec2{{}, inner} {
		for i := range 5 {
			remaining = append(remaining, [6]float64{axis.X, axis.Y, stations[i], radii[side][i], stations[i+1], radii[side][i+1]})
		}
	}
	match := func(a, b [6]float64) bool {
		for i := range a {
			if math.Abs(a[i]-b[i]) > 1e-10 {
				return false
			}
		}
		return true
	}
	for _, b := range hit.Bands {
		i := slices.IndexFunc(remaining, func(a [6]float64) bool { return match(a, b) })
		if i < 0 {
			t.Fatalf("pinned reader changed a native axis, band or trim: %v", b)
		}
		remaining = slices.Delete(remaining, i, i+1)
	}
	if len(remaining) != 0 {
		t.Fatalf("pinned transfer lost original wall partitions: %v", remaining)
	}
	for _, r := range original {
		found := false
		for _, b := range hit.Bands {
			for j := 2; j <= 4; j += 2 {
				found = found || math.Abs(r[0]-b[0])+math.Abs(r[1]-b[1])+math.Abs(r[2]-b[j])+math.Abs(r[3]-b[j+1]) < 1e-10
			}
		}
		if !found {
			t.Fatalf("pinned retransfer lost one of the eight independent original rim circles: %v", r)
		}
	}
	t.Logf("preserved both axes, all ten wall partitions, 48 vertices, 58 edges and positive signed volume %.16g", hit.Volume)
}
