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
	"reflect"
	"testing"
)

func TestWrappedRoundedCylinderExactBlendExtinction(t *testing.T) {
	ctx := context.Background()
	options := geom.SurfacePatchOptions{}
	for _, radii := range [][2]float64{{.5, 1}, {1, .5}, {.5, .5}} {
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
			out, history, e := WrappedRoundedCylinder(ctx, source, -.5, options)
			if e != nil {
				t.Fatal("exact blend extinction", e)
			}
			expectedRadii := [2]float64{radii[0] - .5, radii[1] - .5}
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
				if e != nil || proof.Radius.Cmp(big.NewRat(7, 2)) != 0 || proof.LowerZ.Cmp(big.NewRat(1, 2)) != 0 || proof.UpperZ.Cmp(big.NewRat(15, 2)) != 0 {
					t.Fatal("sharp core", proof, e)
				}
			} else {
				proof, e := out.ValidateWrappedCylinderRimFillet(ctx, options)
				if e != nil || proof.Radius.Cmp(big.NewRat(7, 2)) != 0 || proof.LowerZ.Cmp(big.NewRat(1, 2)) != 0 || proof.UpperZ.Cmp(big.NewRat(15, 2)) != 0 {
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
			// radius3.5, height7 cylinder; zero radii contribute exactly zero.
			R, H := 3.5, 7.0
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
			// The collapse itself is supported, not any negative-radius continuation.
			bad, _, e := WrappedRoundedCylinder(ctx, source, math.Nextafter(-.5, math.Inf(-1)), options)
			if !errors.Is(e, geom.ErrDomain) || bad.Topology().Faces != 0 {
				t.Fatal("one ULP past extinction", e)
			}
		})
	}
}

func TestWrappedRoundedCylinderExtinctionScope(t *testing.T) {
	ctx := context.Background()
	o := geom.SurfacePatchOptions{}
	w, err := geom.NewCanonicalWrappedCylinder(ctx, geom.Vec3{}, 4, 8, o)
	if err != nil {
		t.Fatal(err)
	}
	stock, _, err := trimmed.NewWrappedCylinder(ctx, w, 1e-9, o)
	if err != nil {
		t.Fatal(err)
	}
	one, _, err := trimmed.FilletWrappedCylinderRims(ctx, stock, .5, 0, o)
	if err != nil {
		t.Fatal(err)
	}
	out, h, err := WrappedRoundedCylinder(ctx, one, -.5, o)
	if !errors.Is(err, geom.ErrDomain) || out.Topology().Faces != 0 || h.ResultLateral != -1 {
		t.Fatal("one-sharp-rim extinction remains outside proof", h, err)
	}
	two, _, err := trimmed.FilletWrappedCylinderRims(ctx, stock, .5, 1, o)
	if err != nil {
		t.Fatal(err)
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	out, h, err = WrappedRoundedCylinder(cancelled, two, -.5, o)
	if !errors.Is(err, context.Canceled) || out.Topology().Faces != 0 || h.ResultLateral != -1 {
		t.Fatal("cancelled transition must be atomic", h, err)
	}
}
