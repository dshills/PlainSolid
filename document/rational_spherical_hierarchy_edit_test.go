package document

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
	"strings"
	"testing"

	"plainsolid/exchange/step"
	"plainsolid/geom"
)

func TestRationalSphericalHierarchyDocumentEdit(t *testing.T) {
	ctx := context.Background()
	for _, tc := range []struct {
		file   string
		vendor bool
	}{{"rational-spherical-hierarchy.step", false}, {"occt-rational-spherical-hierarchy.step", true}} {
		t.Run(tc.file, func(t *testing.T) {
			raw, err := os.ReadFile("../exchange/step/testdata/" + tc.file)
			if err != nil {
				t.Fatal(err)
			}
			read := ReadRationalSphericalHierarchyDocumentSTEP
			if tc.vendor {
				read = ReadOCCTRationalSphericalHierarchyDocumentSTEP
			}
			d, err := read(ctx, bytes.NewReader(raw), STEPAssemblyOptions{}, 1e-9)
			if err != nil {
				t.Fatal(err)
			}
			before := d.Occurrences()
			var relation step.ExactNAUOOccurrence
			for _, u := range before {
				if u.Family != "cylinder" || len(u.Path) < 2 {
					continue
				}
				for _, r := range d.Relationships() {
					if r.NAUOID == u.Path[len(u.Path)-1] {
						relation = r
					}
				}
				break
			}
			if relation.NAUOID == 0 {
				t.Fatal("missing original shared cylinder relationship")
			}
			r := big.NewRat
			changed := [3][4]*big.Rat{{r(0, 1), r(-1, 1), r(0, 1), r(19, 10)}, {r(1, 1), r(0, 1), r(0, 1), r(21, 5)}, {r(0, 1), r(0, 1), r(1, 1), r(1, 5)}}
			name := "exact cylinder's shared use"
			edit := step.RationalSphericalHierarchyEdit{NAUOID: relation.NAUOID, Matrix: changed, Name: &name}
			edited, err := d.EditRelationships(ctx, []step.RationalSphericalHierarchyEdit{edit}, STEPAssemblyOptions{})
			if err != nil {
				t.Fatal(err)
			}
			seen := 0
			for i, a := range edited.Occurrences() {
				b := before[i]
				if a.Key != b.Key || a.DefinitionID != b.DefinitionID || a.SourceBodyID != b.SourceBodyID || !reflect.DeepEqual(a.Path, b.Path) || !reflect.DeepEqual(a.SourceFaceIDs, b.SourceFaceIDs) {
					t.Fatal("edit changed original material or relationship IDs", a, b)
				}
				if a.Path[len(a.Path)-1] == relation.NAUOID {
					seen++
					if reflect.DeepEqual(a.World, b.World) {
						t.Fatal("shared edit did not reach expanded material path", a.Path)
					}
				} else if !reflect.DeepEqual(a.World, b.World) {
					t.Fatal("unselected relationship placement changed", a.Path)
				}
			}
			if seen != 2 || !reflect.DeepEqual(d.Occurrences(), before) {
				t.Fatal("expected both paths and unchanged original", seen)
			}
			for _, q := range edited.Relationships() {
				if q.NAUOID == relation.NAUOID {
					if q.FromPlacementID != relation.FromPlacementID || q.TransformationID != relation.TransformationID ||
						q.RelationshipID != relation.RelationshipID || q.CDSRID != relation.CDSRID || q.Name != name || !reflect.DeepEqual(q.Matrix(), changed) {
						t.Fatal("reopened exact local edit or source identity changed", q)
					}
				}
			}
			bad := edit
			bad.Matrix[2][2] = r(-1, 1)
			if v, err := d.EditRelationships(ctx, []step.RationalSphericalHierarchyEdit{bad}, STEPAssemblyOptions{}); v != nil || !errors.Is(err, geom.ErrUnrepresentable) {
				t.Fatal("an improper rational reflection must reject atomically", v, err)
			}
			bad = edit
			bad.Matrix[0][3] = r(1, 3)
			if v, err := d.EditRelationships(ctx, []step.RationalSphericalHierarchyEdit{bad}, STEPAssemblyOptions{}); v != nil || !errors.Is(err, geom.ErrUnrepresentable) {
				t.Fatal("nonterminating STEP translation must reject atomically", v, err)
			}
			if oracle := os.Getenv("PLAINSOLID_OCCT_INDEPENDENT_TORUS_ASSEMBLY"); oracle != "" {
				var staged bytes.Buffer
				if err := edited.WriteSTEP(&staged); err != nil {
					t.Fatal(err)
				}
				path := filepath.Join(t.TempDir(), "edited-rational.step")
				if err := os.WriteFile(path, staged.Bytes(), 0600); err != nil {
					t.Fatal(err)
				}
				data, err := exec.CommandContext(ctx, oracle, "read", path).CombinedOutput()
				var props struct {
					Volume                  float64
					Solids, Faces, EdgeUses int
					Bounds                  [6]float64
				}
				at := strings.LastIndexByte(string(data), '{')
				if err != nil || at < 0 || json.Unmarshal(data[at:], &props) != nil {
					t.Fatalf("OCCT readback: %v %s", err, data)
				}
				if props.Solids != 7 || props.Faces != 22 || props.EdgeUses != 88 || math.Abs(props.Volume-(24+12*math.Pi*math.Pi+136*math.Pi/3)) > 1e-7 {
					t.Fatal("independent OCCT edit changed material", props)
				}
				bounds, err := edited.Bounds(ctx, step.RationalHierarchyBoundsOptions{})
				if err != nil {
					t.Fatal(err)
				}
				for i, b := range [2][3]*big.Rat{bounds.Aggregate.Min(), bounds.Aggregate.Max()} {
					for j, q := range b {
						want, _ := q.Float64()
						if math.Abs(props.Bounds[3*i+j]-want) > 2e-7 {
							t.Fatal("independent OCCT placement differs", props.Bounds, want)
						}
					}
				}
			}
		})
	}
}
