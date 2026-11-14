package document

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
	"strings"
	"testing"

	"plainsolid/exchange/step"
	"plainsolid/geom"
)

func TestAffinePCurveHierarchyDocumentRetainsAuthoring(t *testing.T) {
	ctx := context.Background()
	for _, tc := range []struct {
		name string
		read func(context.Context, io.Reader, STEPAssemblyOptions, float64) (*RationalSphericalHierarchyDocument, error)
	}{
		{"rational-spherical-hierarchy-affine-pcurve-holed.step", ReadAffinePCurveRationalHierarchyDocumentSTEP},
		{"occt-rational-spherical-hierarchy-affine-pcurve-holed.step", ReadOCCTAffinePCurveRationalHierarchyDocumentSTEP},
	} {
		t.Run(tc.name, func(t *testing.T) {
			raw, err := os.ReadFile("../exchange/step/testdata/" + tc.name)
			if err != nil {
				t.Fatal(err)
			}
			d, err := tc.read(ctx, bytes.NewReader(raw), STEPAssemblyOptions{}, 1e-9)
			if err != nil || len(d.AffinePCurves()) != 16 {
				t.Fatal("source affidavit", err)
			}
			var source, native bytes.Buffer
			if err := d.WriteSTEP(&source); err != nil || !bytes.Equal(raw, source.Bytes()) {
				t.Fatal("original STEP (including pcurve records) must survive byte for byte", err)
			}
			if err := d.Write(&native); err != nil {
				t.Fatal(err)
			}
			back, err := ReadAffinePCurveRationalHierarchyDocumentNative(ctx, &native, step.IndependentMaterialJSONLimits{}, STEPAssemblyOptions{})
			if err != nil || !reflect.DeepEqual(d.Occurrences(), back.Occurrences()) || !reflect.DeepEqual(d.AffinePCurves(), back.AffinePCurves()) {
				t.Fatal("native reload and original identities", err)
			}
			var relation int64
			for _, u := range back.Occurrences() {
				if u.Family == "planar" {
					relation = u.Path[len(u.Path)-1]
				}
			}
			if relation == 0 {
				t.Fatal("missing independent planar use")
			}
			var edit step.RationalSphericalHierarchyEdit
			for _, r := range back.Relationships() {
				if r.NAUOID == relation {
					edit = step.RationalSphericalHierarchyEdit{NAUOID: relation, Matrix: r.Matrix()}
				}
			}
			edit.Matrix[0][3].Add(edit.Matrix[0][3], big.NewRat(13, 10))
			edited, err := back.EditRelationships(ctx, []step.RationalSphericalHierarchyEdit{edit}, STEPAssemblyOptions{})
			if err != nil || !reflect.DeepEqual(d.AffinePCurves(), edited.AffinePCurves()) {
				t.Fatal("exact settlement of the original affine source", err)
			}
			before, after := back.Occurrences(), edited.Occurrences()
			for i, a := range after {
				b := before[i]
				changed := !reflect.DeepEqual(a.World, b.World)
				if changed != (b.Path[len(b.Path)-1] == relation) {
					t.Fatal("incorrect affected occurrence set", b.Path, changed)
				}
				a.World = b.World
				if !reflect.DeepEqual(a, b) {
					t.Fatal("original identities changed", a, b)
				}
			}
			bad := edit
			bad.Matrix[0][3] = big.NewRat(1, 3)
			if out, err := back.EditRelationships(ctx, []step.RationalSphericalHierarchyEdit{bad}, STEPAssemblyOptions{}); out != nil || !errors.Is(err, geom.ErrUnrepresentable) {
				t.Fatal("nonterminating placement should fail before publishing", out, err)
			}
			if oracle := os.Getenv("PLAINSOLID_OCCT_INDEPENDENT_TORUS_ASSEMBLY"); oracle != "" {
				var input bytes.Buffer
				if err := edited.WriteSTEP(&input); err != nil {
					t.Fatal(err)
				}
				path := filepath.Join(t.TempDir(), "edited-affine-pcurve.step")
				if err := os.WriteFile(path, input.Bytes(), 0600); err != nil {
					t.Fatal(err)
				}
				data, err := exec.CommandContext(ctx, oracle, "read", path).CombinedOutput()
				var props struct {
					Volume float64
					Bounds [6]float64
				}
				at := strings.LastIndexByte(string(data), '{')
				if err != nil || at < 0 || json.Unmarshal(data[at:], &props) != nil || math.Abs(props.Volume-(48+12*math.Pi*math.Pi+136*math.Pi/3)) > 1e-7 {
					t.Fatalf("OCCT original pcurve readback: %v %s", err, data)
				}
				bounds, err := edited.Bounds(ctx, step.RationalHierarchyBoundsOptions{})
				if err != nil {
					t.Fatal(err)
				}
				for i, b := range [2][3]*big.Rat{bounds.Aggregate.Min(), bounds.Aggregate.Max()} {
					for j, q := range b {
						want, _ := q.Float64()
						if math.Abs(props.Bounds[3*i+j]-want) > 2e-7 {
							t.Fatal("OCCT edited placement differs", props.Bounds, want)
						}
					}
				}
			}
			mass, err := back.MassFromOrientedBoundary(ctx, step.GeneralRationalHierarchyMassOptions{})
			if err != nil {
				t.Fatal(err)
			}
			area, err := edited.BoundaryArea(ctx, step.RationalHierarchyAreaOptions{})
			if err != nil || area.Sum.Coefficients()[0].Cmp(big.NewRat(120, 1)) != 0 {
				t.Fatal("edited Boolean prism retains original planar area", err)
			}
			for _, p := range mass.Products {
				if p.Family == "planar" {
					if c := p.Moments.Volume().Coefficients()[0]; c.Cmp(big.NewRat(48, 1)) != 0 || len(p.SourceFaceIDs) != 10 {
						t.Fatal("signed original loop moment and native faces", c, p.SourceFaceIDs)
					}
					for _, use := range edited.Occurrences() {
						if use.SourceBodyID != p.SourceBodyID {
							continue
						}
						// Local (-9,-2,0) is on the open planar hole wall.
						var point [3]*big.Rat
						for i, row := range use.World {
							point[i] = new(big.Rat).Add(row[3], new(big.Rat).Mul(row[0], big.NewRat(-9, 1)))
							point[i].Add(point[i], new(big.Rat).Mul(row[1], big.NewRat(-2, 1)))
						}
						out, err := edited.BoundaryNormals(ctx, point, step.RationalHierarchyNormalOptions{})
						if err != nil {
							t.Fatal(err)
						}
						for _, leaf := range out.Leaves {
							if !reflect.DeepEqual(leaf.Path, use.Path) {
								continue
							}
							if len(leaf.Normals) != 1 || leaf.Location != step.AnalyticPointBoundary || leaf.Normals[0].SourceFaceID == 0 {
								t.Fatal("moved hole wall must retain its exact original face normal", leaf)
							}
							for i, v := range leaf.Normals[0].Vector() {
								if v.Cmp(use.World[i][0]) != 0 {
									t.Fatal("wrong outward world normal into the hole", v)
								}
							}
							return
						}
					}
					t.Fatal("missing moved hole use")
				}
			}
			t.Fatal("missing affine planar product")
		})
	}
}
