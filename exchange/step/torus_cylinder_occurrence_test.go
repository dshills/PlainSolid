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
	"plainsolid/brep/parametric"
	rigidplacement "plainsolid/brep/placement"
	"plainsolid/geom"
	"strconv"
	"testing"
	"time"
)

func torusCylinderExportPose(t *testing.T, mode int) rigidplacement.RationalRigid {
	t.Helper()
	r := big.NewRat
	var m [3][4]*big.Rat
	if mode == 1 {
		m = [3][4]*big.Rat{{r(0, 1), r(0, 1), r(-1, 1), r(2, 1)}, {r(0, 1), r(1, 1), r(0, 1), r(-3, 1)}, {r(1, 1), r(0, 1), r(0, 1), r(4, 1)}}
	} else {
		m = [3][4]*big.Rat{{r(3, 5), r(0, 1), r(4, 5), r(1, 10)}, {r(0, 1), r(1, 1), r(0, 1), r(-2, 5)}, {r(-4, 5), r(0, 1), r(3, 5), r(3, 8)}}
	}
	p, e := rigidplacement.NewRationalRigid(m, 4096)
	if e != nil {
		t.Fatal(e)
	}
	return p
}
func TestTorusCylinderSTEPExactOccurrence(t *testing.T) {
	ctx := context.Background()
	r := big.NewRat
	for _, mode := range []int{1, 2} {
		for _, op := range []geom.TorusCylinderOperation{geom.TorusCylinderCommon, geom.TorusCylinderDifference, geom.TorusCylinderUnion} {
			b := torusCylinderSTEPBody(op)
			pose := torusCylinderExportPose(t, mode)
			s, e := NewTorusCylinderSTEPOccurrence(ctx, b, pose, .001, 1e-8)
			if e != nil {
				t.Fatal(e)
			}
			p := [3]*big.Rat{r(7, 1), r(0, 1), r(0, 1)}
			if op == geom.TorusCylinderCommon {
				p[0] = r(3, 1)
			}
			world, e := pose.TransformPoint(ctx, p, false, 4096)
			if e != nil {
				t.Fatal(e)
			}
			loc, owner, e := s.Query().ClassifyRational(ctx, world, geom.SurfacePatchOptions{})
			if e != nil || loc != geom.BoreBoundary || owner != geom.TorusCylinderTorus {
				t.Fatal("original owner", loc, owner, e)
			}
			zero := [3]*big.Rat{r(0, 1), r(0, 1), r(0, 1)}
			origin, _ := pose.TransformPoint(ctx, zero, false, 4096)
			direction, _ := pose.TransformVector(ctx, [3]*big.Rat{r(1, 1), r(0, 1), r(1, 10)}, false, 4096)
			line, e := s.Query().ClipRationalLine(ctx, origin, direction, r(-8, 1), r(8, 1), geom.ToroidLineOptions{ParameterTolerance: 1e-8})
			if e != nil {
				t.Fatal(e)
			}
			want := 2
			if op == geom.TorusCylinderUnion {
				want = 1
			}
			if len(line.Intervals) != want {
				t.Fatal("world line", line)
			}
			proof, e := s.ExportProof()
			if e != nil || proof.ErrorUpper.Cmp(new(big.Rat).SetFloat64(1e-8)) > 0 {
				t.Fatal(e)
			}
			var raw, again bytes.Buffer
			if e = s.WriteSTEP(&raw); e != nil {
				t.Fatal(e)
			}
			if e = s.WriteSTEP(&again); e != nil || !bytes.Equal(raw.Bytes(), again.Bytes()) {
				t.Fatal("deterministic", e)
			}
			if _, e = ReadTorusCylinderSet(bytes.NewReader(raw.Bytes()), Limits{}, 1e-8); e == nil {
				t.Fatal("canonical importer widened")
			}
			f, e := Parse(bytes.NewReader(raw.Bytes()), Limits{})
			if e != nil {
				t.Fatal(e)
			}
			// The original torus placement must encode the exact proper rational world
			// origin/axis/reference, not independent rounded nonorthogonal float vectors.
			matrix, _ := pose.Matrix()
			for _, ent := range f.Entities {
				if len(ent.Records) != 1 || ent.Records[0].Type != "TOROIDAL_SURFACE" {
					continue
				}
				pl := f.Entities[ent.Records[0].Args[1].Ref].Records[0]
				for k, col := range []int{3, 2, 0} {
					record := f.Entities[pl.Args[k+1].Ref].Records[0]
					for j := 0; j < 3; j++ {
						q, e := torusCylinderImportNumber(record.Args[1].Items[j])
						if e != nil || q.Cmp(matrix[j][col]) != 0 {
							t.Fatal("exact encoded placement", mode, k, j, e)
						}
					}
				}
			}
		}
	}
	pose := torusCylinderExportPose(t, 2)
	m, _ := pose.Matrix()
	m[0][3] = r(1, 7)
	bad, e := rigidplacement.NewRationalRigid(m, 4096)
	if e != nil {
		t.Fatal(e)
	}
	s, e := NewTorusCylinderSTEPOccurrence(ctx, torusCylinderSTEPBody(geom.TorusCylinderCommon), bad, .001, 1e-8)
	if e == nil {
		t.Fatal("nonterminating pose accepted")
	}
	var raw bytes.Buffer
	if e = s.WriteSTEP(&raw); e == nil || raw.Len() != 0 {
		t.Fatal("partial failure")
	}
	cancelCtx, cancel := context.WithCancel(ctx)
	cancel()
	if _, e = NewTorusCylinderSTEPOccurrence(cancelCtx, torusCylinderSTEPBody(geom.TorusCylinderCommon), pose, .001, 1e-8); e == nil {
		t.Fatal("cancel")
	}
}
func TestOCCTPlacedTorusCylinderSTEP(t *testing.T) {
	oracle := os.Getenv("PLAINSOLID_OCCT_TORUS_CYLINDER_PLACED_STEP")
	if oracle == "" {
		t.Skip("set pinned placed STEP oracle")
	}
	ctx := context.Background()
	r := big.NewRat
	for _, mode := range []int{1, 2} {
		for _, op := range []geom.TorusCylinderOperation{geom.TorusCylinderCommon, geom.TorusCylinderDifference, geom.TorusCylinderUnion} {
			b := torusCylinderSTEPBody(op)
			s, e := NewTorusCylinderSTEPOccurrence(ctx, b, torusCylinderExportPose(t, mode), .001, 1e-8)
			if e != nil {
				t.Fatal(e)
			}
			path := filepath.Join(t.TempDir(), "placed.step")
			f, e := os.Create(path)
			if e != nil {
				t.Fatal(e)
			}
			e = s.WriteSTEP(f)
			ce := f.Close()
			if e != nil || ce != nil {
				t.Fatal(e, ce)
			}
			originalBytes, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			restored, err := ReadTorusCylinderOccurrence(ctx, bytes.NewReader(originalBytes), Limits{}, 1e-8)
			if err != nil {
				t.Fatal(err)
			}
			var roundtrip bytes.Buffer
			if err = restored.WriteSTEP(&roundtrip); err != nil || !bytes.Equal(originalBytes, roundtrip.Bytes()) {
				t.Fatal("placed paired reexport", err)
			}
			s = restored

			output := filepath.Join(t.TempDir(), "read.json")
			args := []string{output, path, "5", "2", "6", "-3", "4", strconv.Itoa(int(op)), strconv.Itoa(mode)}
			call, cancel := context.WithTimeout(ctx, 30*time.Second)
			log, e := exec.CommandContext(call, oracle, args...).CombinedOutput()
			cancel()
			if e != nil {
				t.Fatalf("OCCT: %v %s", e, log)
			}
			data, e := os.ReadFile(output)
			if e != nil {
				t.Fatal(e)
			}
			var got struct {
				Volume, Area, ReferenceVolume, ReferenceArea, EdgeResidual float64
				Centroid, ReferenceCentroid                                [3]float64
				Solids, Shells, Faces                                      int
			}
			if e = json.Unmarshal(data, &got); e != nil {
				t.Fatal(e)
			}
			if got.Solids != 1 || got.Shells != 1 || got.EdgeResidual > 1e-8 || math.Abs(got.Volume-got.ReferenceVolume) > 1e-5 || math.Abs(got.Area-got.ReferenceArea) > 1e-5 {
				t.Fatal(mode, op, got)
			}
			native, err := s.Original().Solid().MassProperties(ctx, parametric.IntegrationOptions{RelativeTolerance: 1e-8})
			if err != nil {
				t.Fatal(err)
			}
			if math.Abs(native.Volume-got.Volume) > 1e-5 || math.Abs(native.Area-got.Area) > 1e-5 {
				t.Fatal("original native invariants", native, got)
			}
			moments, e := s.Query().BoundMoments(ctx, rigidplacement.TorusCylinderOccurrenceMomentOptions{Source: geom.TorusCylinderMomentOptions{VolumeTolerance: r(1, 100), SecondMomentTolerance: r(1, 10), CentroidTolerance: r(1, 1000), InertiaTolerance: r(1, 5)}})
			if e != nil {
				t.Fatal(e)
			}
			vlo, _ := moments.Volume[0].Float64()
			vhi, _ := moments.Volume[1].Float64()
			if got.Volume < vlo-1e-5 || got.Volume > vhi+1e-5 {
				t.Fatal("exact world volume", got.Volume, vlo, vhi)
			}
			for j := 0; j < 3; j++ {
				if math.Abs(got.Centroid[j]-got.ReferenceCentroid[j]) > 1e-7 {
					t.Fatal("independent world centroid", got)
				}
				lo, _ := moments.Centroid[j][0].Float64()
				hi, _ := moments.Centroid[j][1].Float64()
				if got.Centroid[j] < lo-1e-7 || got.Centroid[j] > hi+1e-7 {
					t.Fatal("exact world moment", got.Centroid[j], lo, hi)
				}
			}
			t.Log(mode, op, got.EdgeResidual)
		}
	}
}
