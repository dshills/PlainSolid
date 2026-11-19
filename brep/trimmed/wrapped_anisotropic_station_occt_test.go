//go:build research

package trimmed

import (
	"context"
	"encoding/json"
	"math"
	"math/big"
	"os"
	"os/exec"
	"path/filepath"
	"plainsolid/brep/parametric"
	"plainsolid/geom"
	"testing"
	"time"
)

// Optional independent authoring comparator: the executable constructs its
// own original OCCT NURBS and real Cut, never consumes Go geometry or a mesh.
func TestIndependentAnisotropicStationOCCT(t *testing.T) {
	oracle := os.Getenv("PLAINSOLID_OCCT_ANISOTROPIC_STATION_MATERIAL_AUTHOR")
	if oracle == "" {
		t.Skip("set PLAINSOLID_OCCT_ANISOTROPIC_STATION_MATERIAL_AUTHOR")
	}
	for _, mode := range []string{"filled", "bore"} {
		t.Run(mode, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
			defer cancel()
			dir := t.TempDir()
			stepPath, measurePath := filepath.Join(dir, "original.step"), filepath.Join(dir, "original.json")
			if output, e := exec.CommandContext(ctx, oracle, stepPath, measurePath, mode).CombinedOutput(); e != nil {
				t.Fatalf("independent OCCT author: %v\n%s", e, output)
			}
			raw, e := os.ReadFile(measurePath)
			if e != nil {
				t.Fatal(e)
			}
			var reference struct {
				OCCTVersion                             string
				Volume, Area, WholeFaceArea, AreaChange float64
				Centroid                                [3]float64
				Solids, Shells, Faces, Holes, Euler     int
				Samples                                 []struct {
					Side, Band       int
					U, V             float64
					Point            [3]float64
					BoundaryDistance float64
				}
			}
			if e = json.Unmarshal(raw, &reference); e != nil {
				t.Fatal(e)
			}
			if reference.OCCTVersion == "" || !finite(reference.AreaChange) || reference.AreaChange > 1e-9 || reference.Solids != 1 || reference.Shells != 1 {
				t.Fatal("independent topology/integration", reference)
			}
			if info, e := os.Stat(stepPath); e != nil || info.Size() == 0 {
				t.Fatal("missing independently authored STEP", e)
			}
			var s Solid
			var ids [2][]int
			var volume, area WrappedStationMassInterval
			var center [3]WrappedStationMassInterval
			if mode == "filled" {
				s = anisotropicMassFixture(t, true, geom.Vec3{Z: -4})
				p, e := s.ValidateWrappedAnisotropicStationLoft(ctx, geom.SurfacePatchOptions{})
				if e != nil {
					t.Fatal(e)
				}
				ids[0] = p.History.WallFaces
				mass, e := s.BoundWrappedAnisotropicStationMass(ctx, WrappedRationalStationMassOptions{VolumeTolerance: big.NewRat(1, 1000)})
				if e != nil {
					t.Fatal(e)
				}
				volume, center = mass.Volume, mass.Centroid
				surface, e := s.BoundWrappedAnisotropicStationArea(ctx, WrappedAnisotropicStationAreaOptions{AreaTolerance: big.NewRat(1, 2)})
				if e != nil {
					t.Fatal(e)
				}
				area = surface.Total
			} else {
				s = anisotropicMeasuredBore(t) // native face reindex and JSON reload
				p, e := s.ValidateWrappedAnisotropicStationBore(ctx, geom.SurfacePatchOptions{})
				if e != nil {
					t.Fatal(e)
				}
				ids = [2][]int{p.OuterWalls, p.InnerWalls}
				mass, e := s.BoundWrappedAnisotropicStationBoreMass(ctx, WrappedAnisotropicBoreMassOptions{VolumeTolerance: big.NewRat(1, 1000), InertiaTolerance: big.NewRat(1, 10)})
				if e != nil {
					t.Fatal(e)
				}
				volume, center = mass.Volume, mass.Centroid
				surface, e := s.BoundWrappedAnisotropicStationBoreArea(ctx, WrappedAnisotropicStationAreaOptions{AreaTolerance: big.NewRat(1, 2)})
				if e != nil {
					t.Fatal(e)
				}
				area = surface.Total
			}
			topology := s.Topology()
			if reference.Faces != len(s.faces) || reference.Holes != topology.InnerWires || reference.Euler != topology.EulerCharacteristic {
				t.Fatal("original material topology differs", reference.Faces, reference.Holes, reference.Euler, topology)
			}
			check := func(name string, q WrappedStationMassInterval, want, slack float64) {
				t.Helper()
				lo, _ := q.Lower.Float64()
				hi, _ := q.Upper.Float64()
				if !finite(want) || want < lo-slack || want > hi+slack {
					t.Fatalf("%s interval [%g,%g] misses OCCT %.17g", name, lo, hi, want)
				}
			}
			check("volume", volume, reference.Volume, 1e-7)
			check("area", area, reference.Area, 1e-7)
			for i, q := range center {
				check("centroid", q, reference.Centroid[i], 1e-7)
			}
			// The existing ordinary native boundary integrator is an additional
			// independent path; its numerical results are not certificate endpoints.
			numerical, e := s.MassProperties(ctx, parametric.IntegrationOptions{RelativeTolerance: 1e-9})
			if e != nil {
				t.Fatal(e)
			}
			if math.Abs(numerical.Volume-reference.Volume) > 1e-7 || math.Abs(numerical.Area-reference.Area) > 1e-6 {
				t.Fatal("native boundary quadrature", numerical.Volume, reference.Volume, numerical.Area, reference.Area)
			}
			xyz := [3]float64{numerical.Centroid.X, numerical.Centroid.Y, numerical.Centroid.Z}
			for i, x := range xyz {
				if math.Abs(x-reference.Centroid[i]) > 1e-7 {
					t.Fatal("native centroid", xyz, reference.Centroid)
				}
			}
			if len(reference.Samples) != 20*(len(ids[0])+len(ids[1])) {
				t.Fatal("missing geometric samples")
			}
			for _, sample := range reference.Samples {
				if sample.Side < 0 || sample.Side > 1 || sample.Band < 0 || sample.Band >= len(ids[sample.Side]) || !finite(sample.BoundaryDistance) || sample.BoundaryDistance > 1e-8 {
					t.Fatal("invalid original boundary sample")
				}
				id := ids[sample.Side][sample.Band]
				face := s.faces[id]
				w := face.Surface.(*geom.WrappedSurfacePatch)
				if face.Reversed != (sample.Side == 1) {
					t.Fatal("inward ownership")
				}
				point, e := w.FundamentalPatch().EvaluateRational(ctx, ratInset(sample.U), ratInset(sample.V), geom.SurfacePatchEvaluationOptions{})
				if e != nil {
					t.Fatal(e)
				}
				for axis, v := range point.Point {
					x, _ := v.Float64()
					if !finite(sample.Point[axis]) || math.Abs(x-sample.Point[axis]) > 2e-11 {
						t.Fatal("independent original surface sample", sample.Side, sample.Band, sample.U, sample.V, axis, x, sample.Point[axis])
					}
				}
				if sample.U == 0 && sample.V == 3.5 {
					if mode == "filled" {
						location, e := s.LocateWrappedAnisotropicStationRational(ctx, point.Point, WrappedRationalStationPointOptions{})
						if e != nil || location.Location != parametric.Boundary {
							t.Fatal("original filled boundary", e)
						}
					} else {
						location, e := s.LocateWrappedAnisotropicStationBoreRational(ctx, point.Point, WrappedRationalStationPointOptions{})
						if e != nil || location != parametric.Boundary {
							t.Fatal("original bore boundary", e)
						}
					}
				}
			}
			t.Logf("OCCT%s %s V=%.12g area=%.12g centroid=%v samples=%d; raw whole-face area=%.12g; D1 quadrature change=%g", reference.OCCTVersion, mode, reference.Volume, reference.Area, reference.Centroid, len(reference.Samples), reference.WholeFaceArea, reference.AreaChange)
		})
	}
}
