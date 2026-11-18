package iges

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
	"plainsolid/internal/slowtest"
	"strings"
	"testing"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

func readBoreRimOCCTFixture(t *testing.T, name string) []byte {
	t.Helper()
	raw, err := os.ReadFile("testdata/occt-model-only-eccentric-bore-rim-" + name + ".igs")
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func TestReadWrappedCylinderBoreRimFilletOCCT(t *testing.T) {
	slowtest.Skip(t)
	ctx, o := context.Background(), geom.SurfacePatchOptions{}
	for _, c := range []struct {
		name               string
		radii              [2]*big.Rat
		faces, sourceEdges int
		volume, area       float64
	}{
		{"lower", [2]*big.Rat{big.NewRat(1, 2), new(big.Rat)}, 5, 8, 880.50489958418973, 632.40885550364703},
		{"upper", [2]*big.Rat{new(big.Rat), big.NewRat(3, 4)}, 5, 8, 879.57274568077219, 631.19948065961216},
		{"both", [2]*big.Rat{big.NewRat(1, 2), big.NewRat(3, 4)}, 6, 10, 878.86090593302492, 628.61392105642233},
	} {
		t.Run(c.name, func(t *testing.T) {
			raw := readBoreRimOCCTFixture(t, c.name)
			m, err := ReadWrappedCylinderBoreRimFillet(ctx, bytes.NewReader(raw), Limits{}, o)
			if err != nil {
				t.Fatal(err)
			}
			p, err := m.Solid.ValidateWrappedCylinderBoreRimFillet(ctx, o)
			if err != nil || !p.Complete || m.LengthUnit != .001 || p.Stock.CenterX.Sign() != 0 ||
				p.Stock.CenterY.Cmp(big.NewRat(64, 1)) != 0 || p.Hole.CenterX.Cmp(big.NewRat(3, 2)) != 0 ||
				p.Hole.CenterY.Cmp(big.NewRat(66, 1)) != 0 || p.Fillets[0].Cmp(c.radii[0]) != 0 || p.Fillets[1].Cmp(c.radii[1]) != 0 {
				t.Fatalf("native axes, file unit or rolling radii changed: %+v, %v", p, err)
			}
			if m.SourceGroupID != 1 || m.SourceSolidID != 3 || m.SourceShellID != 5 ||
				m.SourceEdgeListID != 19 || m.SourceVertexListID != 25 || len(m.NativeFaceSourceIDs) != c.faces {
				t.Fatalf("original directory ownership changed: %+v", m)
			}
			top := m.Solid.Topology()
			if top.Faces != c.faces || top.Vertices != 4*c.faces || top.Edges != 5*c.faces-2 ||
				top.EulerCharacteristic != 0 || top.InnerWires != 2 || len(top.Shells) != 1 {
				t.Fatalf("original annular ownership changed: %+v", top)
			}
			sourceCopy := m.SourceIGES()
			if !bytes.Equal(raw, sourceCopy) {
				t.Fatal("original IGES bytes changed")
			}
			sourceCopy[0] ^= 1
			if !bytes.Equal(raw, m.SourceIGES()) {
				t.Fatal("the caller can edit the retained source bytes")
			}
			edgeCounts := map[int]int{}
			curveIDs := map[int]int{}
			for i, id := range m.NativeEdgeSourceIndices {
				e := m.NativeCurveSourceIDs[i]
				if prior, ok := curveIDs[id]; ok && prior != e {
					t.Fatalf("source edge %d maps to different original curves", id)
				}
				edgeCounts[id]++
				curveIDs[id] = e
			}
			whole := 0
			for i := 1; i <= c.sourceEdges; i++ {
				switch edgeCounts[i] {
				case 4:
					whole++
				case 1:
				default:
					t.Fatalf("invalid exact split of source edge %d: %v", i, edgeCounts)
				}
			}
			if whole != c.faces || len(curveIDs) != c.sourceEdges {
				t.Fatalf("original circular trim IDs lost: %v", edgeCounts)
			}
			vertices := map[int]int{}
			for _, id := range m.NativeVertexSourceIndices {
				vertices[id]++
			}
			if len(vertices) != c.faces+1 || vertices[0] != 3*c.faces {
				t.Fatalf("lost original seam vertices or new exact quarters: %v", m.NativeVertexSourceIndices)
			}
			pMass, err := m.Solid.MassProperties(ctx, parametric.IntegrationOptions{RelativeTolerance: 1e-8})
			if err != nil || math.Abs(pMass.Volume-c.volume) > 1e-7 || math.Abs(pMass.Area-c.area) > 1e-7 {
				t.Fatalf("native signed mass differs from original OCCT: %+v, %v", pMass, err)
			}
			var canonical, repeated bytes.Buffer
			if err := WriteWrappedCylinderBoreRimFillet(ctx, &canonical, m.Solid, m.LengthUnit, o); err != nil {
				t.Fatal(err)
			}
			reopened, err := ReadWrappedCylinderBoreRimFillet(ctx, bytes.NewReader(canonical.Bytes()), Limits{}, o)
			if err != nil || reopened.LengthUnit != m.LengthUnit || reopened.SourceGroupID != 0 {
				t.Fatalf("strict reopen of PlainSolid's own four-quadrant producer: %v", err)
			}
			if err := WriteWrappedCylinderBoreRimFillet(ctx, &repeated, reopened.Solid, reopened.LengthUnit, o); err != nil ||
				!bytes.Equal(canonical.Bytes(), repeated.Bytes()) {
				t.Fatalf("canonical IGES changed on strict round trip: %v", err)
			}
			if oracle := os.Getenv("PLAINSOLID_OCCT_ECCENTRIC_BORE_RIM_TRANSFER"); oracle != "" {
				path := filepath.Join(t.TempDir(), "strict-reexport.igs")
				if err := os.WriteFile(path, repeated.Bytes(), 0600); err != nil {
					t.Fatal(err)
				}
				result, err := exec.CommandContext(ctx, oracle, path).CombinedOutput()
				var got struct {
					Solids, Faces int
					Volume, Area  float64
					Toroids       [][9]float64
				}
				start := bytes.Index(result, []byte(`{"Solids":`))
				if err != nil || start < 0 || json.Unmarshal(result[start:], &got) != nil || got.Solids != 1 ||
					got.Faces != c.faces || len(got.Toroids) != c.faces-4 || math.Abs(got.Volume-c.volume) > 1e-7 ||
					math.Abs(got.Area-c.area) > 1e-7 || got.Volume <= 0 {
					t.Fatalf("signed pinned retransfer of independently imported original: %v\n%s", err, result)
				}
				for _, torus := range got.Toroids {
					if torus[0] != 1.5 || torus[1] != 66 || torus[5] != 2 || torus[6] != torus[3] {
						t.Fatalf("pinned readback moved an original axis or shoulder: %v", torus)
					}
				}
				t.Logf("pinned IGES retransfer: %d faces, positive volume %.16g, area %.16g", got.Faces, got.Volume, got.Area)
			}
		})
	}
}

func TestReadWrappedCylinderBoreRimFilletStrict(t *testing.T) {
	slowtest.Skip(t)
	ctx, o := context.Background(), geom.SurfacePatchOptions{}
	raw := readBoreRimOCCTFixture(t, "both")
	reject := func(t *testing.T, data []byte, l Limits) {
		t.Helper()
		m, err := ReadWrappedCylinderBoreRimFillet(ctx, bytes.NewReader(data), l, o)
		if err == nil || len(m.Solid.Faces()) != 0 || len(m.SourceIGES()) != 0 || len(m.NativeEdgeSourceIndices) != 0 {
			t.Fatalf("invalid original admitted or partial result returned: %v", err)
		}
	}
	reject(t, readBoreRimOCCTFixture(t, "off-support"), Limits{})
	for _, c := range []struct {
		name              string
		entity, parameter int
		prior, value      byte
	}{
		{"same-oriented lower shoulder", 121, 10, '1', '0'},
		{"wrong lower shoulder owner", 121, 9, '8', '4'},
		{"inward lower blend", 5, 13, '1', '0'},
		{"meridian transform dilation", 57, 1, '1', '2'},
		{"wrong group member", 1, 2, '3', '5'},
	} {
		t.Run(c.name, func(t *testing.T) {
			bad := bytes.Clone(raw)
			changeBoreRimIGESDigit(t, bad, c.entity, c.parameter, c.prior, c.value)
			if bytes.Equal(raw, bad) {
				t.Fatal("negative did not change the intended source field")
			}
			reject(t, bad, Limits{})
		})
	}
	badStatus := bytes.Clone(raw)
	for _, line := range bytes.Split(badStatus, []byte{'\n'}) {
		if len(line) == 80 && line[72] == 'D' && strings.TrimLeft(string(line[73:]), "0 ") == "3" {
			copy(line[64:72], "00000000")
		}
	}
	if bytes.Equal(raw, badStatus) {
		t.Fatal("negative did not change the grouped solid's directory status")
	}
	reject(t, badStatus, Limits{})
	reject(t, raw, Limits{MaxBytes: int64(len(raw) - 1)})
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if m, err := ReadWrappedCylinderBoreRimFillet(canceled, bytes.NewReader(raw), Limits{}, o); !errors.Is(err, context.Canceled) || len(m.SourceIGES()) != 0 {
		t.Fatalf("canceled read returned material: %v", err)
	}
}

func changeBoreRimIGESDigit(t *testing.T, raw []byte, entity, parameter int, prior, value byte) {
	t.Helper()
	current := 0
	for _, line := range bytes.Split(raw, []byte{'\n'}) {
		if len(line) != 80 || line[72] != 'P' {
			continue
		}
		owner, err := field(string(line[64:72]))
		if err != nil || owner != entity {
			continue
		}
		for i, c := range line[:64] {
			if c == ',' {
				current++
			} else if current == parameter && c >= '0' && c <= '9' {
				if c != prior {
					t.Fatalf("entity %d parameter %d originally %c, expected %c", entity, parameter, c, prior)
				}
				line[i] = value
				return
			}
		}
	}
	t.Fatalf("no source parameter %d in directory entry %d", parameter, entity)
}
