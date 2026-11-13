package occtcheck

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"plainsolid/brep"
	"plainsolid/csg/draft"
	"plainsolid/geom"
	"strings"
	"testing"
	"time"
)

// The reference independently intersects six OCCT halfspaces. It never reads
// native vertices, tessellation or STEP output from the operation under test.
func TestOCCTPlanarDraft(t *testing.T) {
	program := os.Getenv("PLAINSOLID_OCCT_ORACLE")
	if program == "" {
		t.Skip("set PLAINSOLID_OCCT_ORACLE to the optional OCCT executable")
	}
	path := filepath.Join(t.TempDir(), "draft.jsonl")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if b, e := exec.CommandContext(ctx, program, path, "draft").CombinedOutput(); e != nil {
		t.Fatalf("%v: %s", e, b)
	}
	data, e := os.ReadFile(path)
	if e != nil {
		t.Fatal(e)
	}
	type draftRecord struct {
		record
		Vertices, Edges, Faces, Solids int
	}
	refs := map[string]draftRecord{}
	for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		var r draftRecord
		if e = json.Unmarshal([]byte(line), &r); e != nil {
			t.Fatal(e)
		}
		if _, ok := refs[r.Name]; ok {
			t.Fatal("duplicate fixture")
		}
		refs[r.Name] = r
	}
	if len(refs) != 8 {
		t.Fatal("expected eight reference cases")
	}
	for _, name := range []string{"single", "four", "collapse", "oblique", "chamfer-two", "chamfer-four"} {
		t.Run(name, func(t *testing.T) {
			s, e := brep.Box(geom.Vec3{}, geom.Vec3{X: 1, Y: 1, Z: 1})
			if e != nil {
				t.Fatal(e)
			}
			n, e := geom.NewPlane(geom.Vec3{}, geom.Vec3{Z: 1})
			if e != nil {
				t.Fatal(e)
			}
			changes := []draft.FaceShear{{Face: 3, Displacement: geom.Vec3{X: .5}}}
			if name == "four" || name == "collapse" {
				k := .25
				if name == "collapse" {
					k = -.75
				}
				changes = []draft.FaceShear{{Face: 5, Displacement: geom.Vec3{X: -k}}, {Face: 3, Displacement: geom.Vec3{X: k}}, {Face: 2, Displacement: geom.Vec3{Y: -k}}, {Face: 4, Displacement: geom.Vec3{Y: k}}}
			}
			if name == "oblique" {
				n, e = geom.NewPlane(geom.Vec3{Z: .5}, geom.Vec3{X: 1, Z: 1})
				if e != nil {
					t.Fatal(e)
				}
				changes[0].Displacement.X = .25
			}
			result, e := draft.Shear(ctx, s, n, changes, draft.Options{})
			if strings.HasPrefix(name, "chamfer-") {
				var cuts []draft.EdgeChamfer
				vertices := s.Vertices()
				for i, edge := range s.Edges() {
					a, b := vertices[edge.Start], vertices[edge.End]
					if a.X != b.X || a.Y != b.Y {
						continue
					}
					if name == "chamfer-two" {
						if a.X == 1 && a.Y == 1 {
							cuts = append(cuts, draft.EdgeChamfer{Edge: i, Face: 3, Setback: .25, OtherSetback: .5})
						}
					} else {
						face := -1
						for j, f := range s.Faces() {
							for _, u := range f.Boundary {
								if u.Edge == i {
									face = j
								}
							}
						}
						cuts = append(cuts, draft.EdgeChamfer{Edge: i, Face: face, Setback: .125, OtherSetback: .125})
					}
				}
				var r draft.ChamferResult
				r, e = draft.Chamfer(ctx, s, cuts, false, draft.Options{})
				result = r.Result
			}

			if e != nil {
				t.Fatal(e)
			}
			m, e := result.Solid.MassProperties()
			if e != nil {
				t.Fatal(e)
			}
			ref, ok := refs[name]
			if !ok {
				t.Fatal("missing reference")
			}
			compare(t, ref.record, m.SurfaceArea, m.Volume, m.Centroid)
			top := result.Solid.Topology()
			if top.Vertices != ref.Vertices || top.Edges != ref.Edges || top.Faces != ref.Faces || top.Shells != ref.Solids || top.EulerCharacteristic != 2 {
				t.Fatalf("native topology %+v reference %+v", top, ref)
			}
		})
	}
}
