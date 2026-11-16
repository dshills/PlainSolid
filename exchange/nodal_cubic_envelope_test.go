package exchange_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"plainsolid/brep/trimmed"
	"plainsolid/exchange/iges"
	"plainsolid/exchange/step"
	"plainsolid/geom"
)

type exactNodalTransfer struct {
	solid                  trimmed.Solid
	history                trimmed.NodalCubicCommonHistory
	writeSTEP, writeIGES   func(io.Writer) error
	verifySTEP, verifyIGES func(io.Reader, float64) error
}

func nodalTransferSource(ctx context.Context, r io.Reader, common bool) (exactNodalTransfer, error) {
	const name, units = "Nodal envelope", .001
	so := step.NodalCubicSTEPOptions{PoleTolerance: 1e-14}
	igesOptions := iges.NodalCubicIGESOptions{PoleTolerance: 1e-14}
	if common {
		v, err := trimmed.ReadNodalCubicCommonJSON(ctx, r, trimmed.JSONLimits{}, trimmed.NodalCubicCommonOptions{})
		if err != nil {
			return exactNodalTransfer{}, err
		}
		return exactNodalTransfer{solid: v.Boundary(), history: v.History(),
			writeSTEP: func(w io.Writer) error { return step.WriteNodalCubicCommon(ctx, w, v, name, units, step.Limits{}, so) },
			writeIGES: func(w io.Writer) error {
				return iges.WriteNodalCubicCommon(ctx, w, v, units, iges.Limits{}, igesOptions)
			},
			verifySTEP: func(r io.Reader, u float64) error {
				return step.VerifyNodalCubicCommonSTEP(ctx, r, v, name, u, step.Limits{}, so)
			},
			verifyIGES: func(r io.Reader, u float64) error {
				return iges.VerifyNodalCubicCommonIGES(ctx, r, v, u, iges.Limits{}, igesOptions)
			},
		}, nil
	}
	v, err := trimmed.ReadNodalCubicFuseJSON(ctx, r, trimmed.JSONLimits{}, trimmed.NodalCubicFuseOptions{})
	if err != nil {
		return exactNodalTransfer{}, err
	}
	return exactNodalTransfer{solid: v.Boundary(), history: v.History(),
		writeSTEP: func(w io.Writer) error { return step.WriteNodalCubicFuse(ctx, w, v, name, units, step.Limits{}, so) },
		writeIGES: func(w io.Writer) error { return iges.WriteNodalCubicFuse(ctx, w, v, units, iges.Limits{}, igesOptions) },
		verifySTEP: func(r io.Reader, u float64) error {
			return step.VerifyNodalCubicFuseSTEP(ctx, r, v, name, u, step.Limits{}, so)
		},
		verifyIGES: func(r io.Reader, u float64) error {
			return iges.VerifyNodalCubicFuseIGES(ctx, r, v, u, iges.Limits{}, igesOptions)
		},
	}, nil
}

func TestNodalCubicEnvelopeExchange(t *testing.T) {
	ctx := context.Background()
	const units = .001
	for _, mode := range []struct {
		name         string
		volume, area float64
	}{
		{"common", 807933.0 / 65536, 32.5713789018693},
		{"fuse", 821259.0 / 65536, 33.2084356859493},
	} {
		for order := range 2 {
			t.Run(fmt.Sprintf("%s/order%d", mode.name, order), func(t *testing.T) {
				data, err := os.ReadFile(fmt.Sprintf("testdata/native_nodal_cubic_%s_order_%d.json", mode.name, order))
				if err != nil {
					t.Fatal(err)
				}
				source, err := nodalTransferSource(ctx, bytes.NewReader(data), mode.name == "common")
				if err != nil {
					t.Fatal(err)
				}
				var st, ig bytes.Buffer
				if err := source.writeSTEP(&st); err != nil {
					t.Fatal(err)
				}
				if err := source.writeIGES(&ig); err != nil {
					t.Fatal(err)
				}
				if err := source.verifySTEP(bytes.NewReader(st.Bytes()), units); err != nil {
					t.Fatal(err)
				}
				if err := source.verifyIGES(bytes.NewReader(ig.Bytes()), units); err != nil {
					t.Fatal(err)
				}
				f, err := step.Parse(bytes.NewReader(st.Bytes()), step.Limits{})
				if err != nil {
					t.Fatal(err)
				}
				pcurves := 0
				for _, entity := range f.Entities {
					for _, r := range entity.Records {
						if r.Type == "PCURVE" {
							pcurves++
						}
					}
				}
				if pcurves != 2*len(source.solid.Edges()) {
					t.Fatalf("expected both original coedges per model edge; found %d pcurves", pcurves)
				}
				tampered := bytes.Replace(st.Bytes(), []byte(".CURVE_3D."), []byte(".PCURVE_S1."), 1)
				if bytes.Equal(tampered, st.Bytes()) || !errors.Is(source.verifySTEP(bytes.NewReader(tampered), units), geom.ErrConvergence) {
					t.Fatal("STEP accepted changing an original master curve")
				}
				if err := source.verifyIGES(bytes.NewReader(ig.Bytes()), 1); !errors.Is(err, geom.ErrConvergence) {
					t.Fatalf("IGES accepted different source units: %v", err)
				}
				checkNodalCubicReadback(t, ctx, "step", st.Bytes(), os.Getenv("PLAINSOLID_OCCT_CLIPPED_CUT_STEP_COMPONENTS"), source, mode.volume, mode.area)
				checkNodalCubicReadback(t, ctx, "igs", ig.Bytes(), os.Getenv("PLAINSOLID_OCCT_CLIPPED_CUT_IGES"), source, mode.volume, mode.area)
			})
		}
	}
}

type pinnedExactComponentReport struct {
	Faces, Edges, Shells int
	Volume, Area         float64
	Vertices             [][3]float64
	EdgeVertices         [][2]int
}

type pinnedExactReadbackReport struct {
	Solids, IndexedVertices int
	Components              []pinnedExactComponentReport
}

func readPinnedExactComponents(t *testing.T, ctx context.Context, format string, data []byte, reader string) pinnedExactReadbackReport {
	t.Helper()
	file := filepath.Join(t.TempDir(), "exact-envelope."+format)
	if err := os.WriteFile(file, data, 0600); err != nil {
		t.Fatal(err)
	}
	raw, err := exec.CommandContext(ctx, reader, file).CombinedOutput()
	if err != nil {
		t.Fatalf("independent %s reader: %v\n%s", format, err, raw)
	}
	var payload []byte
	for _, line := range bytes.Split(raw, []byte{'\n'}) {
		if line = bytes.TrimSpace(line); bytes.HasPrefix(line, []byte{'{'}) {
			payload = line
		}
	}
	var out pinnedExactReadbackReport
	if err := json.Unmarshal(payload, &out); err != nil {
		t.Fatalf("independent %s material report: %v\n%s", format, err, raw)
	}
	return out
}

func checkNodalCubicReadback(t *testing.T, ctx context.Context, format string, data []byte, reader string, source exactNodalTransfer, volume, area float64) {
	t.Helper()
	if reader == "" {
		return
	}
	out := readPinnedExactComponents(t, ctx, format, data, reader)
	if out.Solids != 1 || len(out.Components) != 1 || out.IndexedVertices != 18 {
		t.Fatalf("independent %s changed native material topology", format)
	}
	body := out.Components[0]
	vertices := source.solid.Vertices()
	discrete := func(v geom.Vec3) [3]float64 { return [3]float64{v.X, v.Y, v.Z} }
	original := map[[3]float64]bool{}
	for _, v := range vertices {
		original[discrete(v)] = true
	}
	for _, v := range body.Vertices {
		if !original[v] {
			t.Fatalf("independent %s changed an exact original vertex: %v", format, v)
		}
		delete(original, v)
	}
	if body.Faces != 12 || body.Edges != 28 || body.Shells != 1 || len(original) != 0 ||
		math.Abs(body.Volume-volume) > 1e-8 || math.Abs(body.Area-area) > 1e-8 {
		t.Fatalf("independent %s changed topology or material: %d faces, %d edges, volume %.16g, area %.16g, vertices omitted %d",
			format, body.Faces, body.Edges, body.Volume, body.Area, len(original))
	}
	n := discrete(vertices[source.history.Vertices[0]])
	e := source.solid.Edges()[source.history.Seam]
	a, b := discrete(vertices[e.Start]), discrete(vertices[e.End])
	rays, seams := 0, 0
	for _, pair := range body.EdgeVertices {
		if pair[0] < 0 || pair[0] >= len(body.Vertices) || pair[1] < 0 || pair[1] >= len(body.Vertices) {
			t.Fatalf("independent %s lost a model endpoint", format)
		}
		p, q := body.Vertices[pair[0]], body.Vertices[pair[1]]
		if p == n || q == n {
			rays++
		}
		if p == a && q == b || p == b && q == a {
			seams++
		}
	}
	if rays != 4 || seams != 1 {
		t.Fatalf("independent %s changed the original node incidence or exterior seam: %d rays, %d seams", format, rays, seams)
	}
}
