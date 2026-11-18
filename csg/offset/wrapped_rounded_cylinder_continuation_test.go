package offset

import (
	"bytes"
	"context"
	"errors"
	"math"
	"math/big"
	"plainsolid/brep/edgecheck"
	"plainsolid/brep/parametric"
	"plainsolid/brep/trimmed"
	"plainsolid/geom"
	"plainsolid/internal/slowtest"
	"reflect"
	"testing"
)

func TestWrappedRoundedCylinderPastBlend(t *testing.T) {
	slowtest.Skip(t)
	ctx := context.Background()
	options := geom.SurfacePatchOptions{}
	for _, radii := range [][2]float64{{.5, 1}, {1, .5}} {
		t.Run(new(big.Rat).SetFloat64(radii[0]).RatString()+"_"+new(big.Rat).SetFloat64(radii[1]).RatString(), func(t *testing.T) {
			w, e := geom.NewCanonicalWrappedCylinder(ctx, geom.Vec3{}, 4, 8, options)
			if e != nil {
				t.Fatal(e)
			}
			stock, _, e := trimmed.NewWrappedCylinder(ctx, w, 1e-9, options)
			if e != nil {
				t.Fatal(e)
			}
			source, _, e := trimmed.FilletWrappedCylinderRims(ctx, stock, radii[0], radii[1], options)
			if e != nil {
				t.Fatal(e)
			}
			// Original face IDs must be discovered, rather than inherited from builder order.
			faces := source.Faces()
			faces = append(faces[2:], faces[:2]...)
			source, e = trimmed.NewSolid(source.Vertices(), source.Edges(), faces, source.Tolerance())
			if e != nil {
				t.Fatal(e)
			}
			before, e := source.ValidateWrappedCylinderRimFillet(ctx, options)
			if e != nil {
				t.Fatal(e)
			}
			out, continuation, e := ErodeWrappedRoundedCylinderPastBlend(ctx, source, .75, options)
			history := continuation.Faces
			if e != nil {
				t.Fatal("sharp-rim continuation", e)
			}
			expectedRadii := [2]float64{math.Max(0, radii[0]-.75), math.Max(0, radii[1]-.75)}
			expectedFaces := 3
			for _, r := range expectedRadii {
				if r > 0 {
					expectedFaces++
				}
			}
			if out.Topology().Faces != expectedFaces || out.Topology().EulerCharacteristic != 2 || len(history.SourceFaces) != expectedFaces {
				t.Fatal("regularized topology", out.Topology(), history)
			}
			if history.SourceFaces[history.ResultLateral][0] != before.Lateral {
				t.Fatal("lateral ancestry")
			}
			for end, r := range expectedRadii {
				if history.SourceFaces[history.ResultCaps[end]][0] != before.Caps[end] {
					t.Fatal("cap ancestry", end)
				}
				if r == 0 {
					if history.ResultBlends[end] != -1 {
						t.Fatal("zero-area blend retained")
					}
					for _, owners := range history.SourceFaces {
						for _, id := range owners {
							if id == before.Blends[end] {
								t.Fatal("collapsed blend mapped to surviving face")
							}
						}
					}
				} else if id := history.ResultBlends[end]; id < 0 || len(history.SourceFaces[id]) != 1 || history.SourceFaces[id][0] != before.Blends[end] {
					t.Fatal("surviving curved ancestry", end)
				}
			}
			if expectedFaces == 3 {
				_, proof, e := out.ValidateCanonicalWrappedCylinder(ctx, options)
				if e != nil || proof.Radius.Cmp(big.NewRat(13, 4)) != 0 || proof.LowerZ.Cmp(big.NewRat(3, 4)) != 0 || proof.UpperZ.Cmp(big.NewRat(29, 4)) != 0 {
					t.Fatal("sharp core", proof, e)
				}
			} else {
				proof, e := out.ValidateWrappedCylinderRimFillet(ctx, options)
				if e != nil || proof.Radius.Cmp(big.NewRat(13, 4)) != 0 || proof.LowerZ.Cmp(big.NewRat(3, 4)) != 0 || proof.UpperZ.Cmp(big.NewRat(29, 4)) != 0 {
					t.Fatal("mixed core", proof, e)
				}
				for i, r := range expectedRadii {
					if proof.Fillets[i].Cmp(new(big.Rat).SetFloat64(r)) != 0 {
						t.Fatal("exact new radius", proof.Fillets)
					}
				}
			}
			mass, e := out.MassProperties(ctx, parametric.IntegrationOptions{RelativeTolerance: 1e-8})
			if e != nil {
				t.Fatal(e)
			}
			// Independent meridian integrals: remove each rounded corner from a
			// radius3.25, height6.5 cylinder; zero radii contribute exactly zero.
			R, H := 3.25, 6.5
			volume := math.Pi * R * R * H
			area := 2 * math.Pi * R * (H - expectedRadii[0] - expectedRadii[1])
			for _, r := range expectedRadii {
				volume -= math.Pi * ((2-math.Pi/2)*R*r*r + (math.Pi/2-5.0/3)*r*r*r)
				area += math.Pi*(R-r)*(R-r) + math.Pi*math.Pi*r*(R-r) + 2*math.Pi*r*r
			}
			if math.Abs(mass.Volume-volume) > 1e-7 || math.Abs(mass.Area-area) > 1e-7 {
				t.Fatal("independent core integral", mass, volume, area)
			}
			agreement, e := out.ValidateEdgeAgreement(ctx, edgecheck.Options{})
			if e != nil || !agreement.Complete {
				t.Fatal("exact shared rim", agreement, e)
			}
			var native bytes.Buffer
			if e = trimmed.WriteJSON(&native, out); e != nil {
				t.Fatal(e)
			}
			back, e := trimmed.ReadJSONContext(ctx, &native, trimmed.JSONLimits{})
			if e != nil || !reflect.DeepEqual(back.Topology(), out.Topology()) {
				t.Fatal("native transition reload", e)
			}
			rim := continuation.SharpRim
			if len(rim.Edges) != 4 || rim.Radius.Cmp(big.NewRat(13, 4)) != 0 || rim.SourceLateral != before.Lateral || rim.SourceCap != before.Caps[rim.End] || rim.SourceBlend != before.Blends[rim.End] {
				t.Fatal("contact ancestry", rim)
			}
			expectedZ := big.NewRat(3, 4)
			if rim.End == 1 {
				expectedZ = big.NewRat(29, 4)
			}
			if rim.Z.Cmp(expectedZ) != 0 {
				t.Fatal("contact height", rim)
			}
			for _, depth := range []float64{.5, 1, 1.25, math.NaN()} {
				bad, history, e := ErodeWrappedRoundedCylinderPastBlend(ctx, source, depth, options)
				if e == nil || bad.Topology().Faces != 0 || history.SharpRim.Radius != nil {
					t.Fatal("atomic unsupported depth", depth, e)
				}
			}
			cancelled, cancel := context.WithCancel(ctx)
			cancel()
			bad, _, e := ErodeWrappedRoundedCylinderPastBlend(cancelled, source, .75, options)
			if !errors.Is(e, context.Canceled) || bad.Topology().Faces != 0 {
				t.Fatal(e)
			}
		})
	}
}
