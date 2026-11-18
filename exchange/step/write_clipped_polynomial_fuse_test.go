package step

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
	"testing"

	"plainsolid/brep/trimmed"
	"plainsolid/geom"
)

func TestWriteClippedPolynomialFuse(t *testing.T) {
	slowtest.Skip(t)
	ctx := context.Background()
	input, err := os.Open("../testdata/native_clipped_polynomial_fuse.json")
	if err != nil {
		t.Fatal(err)
	}
	defer input.Close()
	fuse, err := trimmed.ReadClippedPolynomialFuseJSON(ctx, input, trimmed.JSONLimits{}, trimmed.PolynomialCubicCrossingTangencyEnvelopeOptions{})
	if err != nil {
		t.Fatal(err)
	}
	var a, b bytes.Buffer
	opts := ClippedPolynomialFuseSTEPOptions{PoleTolerance: 1e-14}
	for _, w := range []*bytes.Buffer{&a, &b} {
		if err := WriteClippedPolynomialFuse(ctx, w, fuse, "Stepped roofs", .001, Limits{}, opts); err != nil {
			t.Fatal(err)
		}
	}
	if !bytes.Equal(a.Bytes(), b.Bytes()) {
		t.Fatal("nondeterministic STEP export")
	}
	parsed, err := Parse(bytes.NewReader(a.Bytes()), Limits{})
	if err != nil {
		t.Fatal(err)
	}
	counts := map[string]int{}
	for _, e := range parsed.Entities {
		for _, r := range e.Records {
			counts[r.Type]++
			if r.Type == "SURFACE_CURVE" && len(r.Args[2].Items) != 2 {
				t.Fatal("missing second original curve owner")
			}
		}
	}
	for kind, count := range map[string]int{"MANIFOLD_SOLID_BREP": 1, "CLOSED_SHELL": 1, "ADVANCED_FACE": 20, "EDGE_CURVE": 46, "VERTEX_POINT": 28, "PCURVE": 92} {
		if counts[kind] != count {
			t.Fatalf("%s count %d, want %d", kind, counts[kind], count)
		}
	}
	verify := func(raw []byte, unit float64) error {
		return VerifyClippedPolynomialFuseSTEP(ctx, bytes.NewReader(raw), fuse, "Stepped roofs", unit, Limits{}, opts)
	}
	if err := verify(bytes.Replace(a.Bytes(), []byte("geometry.step"), []byte("renamed.step"), 1), .001); err != nil {
		t.Fatal("harmless header change:", err)
	}
	changed := bytes.Replace(a.Bytes(), []byte(".CURVE_3D."), []byte(".PCURVE_S1."), 1)
	if err := verify(changed, .001); !errors.Is(err, geom.ErrConvergence) {
		t.Fatalf("changed model-vs-owner premise accepted: %v", err)
	}
	if err := verify(a.Bytes(), 1); !errors.Is(err, geom.ErrConvergence) {
		t.Fatalf("changed material units accepted: %v", err)
	}
	var refused bytes.Buffer
	if err := WriteClippedPolynomialFuse(ctx, &refused, fuse, "Stepped roofs", .001, Limits{}, ClippedPolynomialFuseSTEPOptions{}); !errors.Is(err, geom.ErrUnrepresentable) || refused.Len() != 0 {
		t.Fatalf("nondecimal original pole without opt-in returned bytes: %v", err)
	}
	if err := WriteClippedPolynomialFuse(ctx, &refused, fuse, "Stepped roofs", .001, Limits{MaxFaces: 19}, opts); err == nil || refused.Len() != 0 {
		t.Fatalf("face limit returned bytes: %v", err)
	}
	if err := WriteClippedPolynomialFuse(ctx, &refused, fuse, "Stepped roofs", .001, Limits{MaxBytes: 256}, opts); err == nil || refused.Len() != 0 {
		t.Fatalf("byte limit returned bytes: %v", err)
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if err := WriteClippedPolynomialFuse(cancelled, &refused, fuse, "Stepped roofs", .001, Limits{}, opts); !errors.Is(err, context.Canceled) || refused.Len() != 0 {
		t.Fatalf("cancelled write returned bytes: %v", err)
	}
	if oracle := os.Getenv("PLAINSOLID_OCCT_CLIPPED_FUSE_STEP"); oracle != "" {
		path := filepath.Join(t.TempDir(), "clipped-fuse.step")
		if err := os.WriteFile(path, a.Bytes(), 0600); err != nil {
			t.Fatal(err)
		}
		data, err := exec.Command(oracle, "read", path).CombinedOutput()
		var got struct {
			Volume, Area  float64
			Faces, Solids int
		}
		if err != nil || json.Unmarshal(data, &got) != nil {
			t.Fatalf("independent STEP reader: %v\n%s", err, data)
		}
		volume, _ := fuse.ExactVolume().Float64()
		if got.Solids != 1 || got.Faces != 20 || math.Abs(got.Volume-volume) > 1e-7 || math.Abs(got.Area-40.443802639426487) > 1e-7 {
			t.Fatalf("independent STEP measures: %+v; exact volume %.16g", got, volume)
		}
	}
}

func TestClippedFuseSTEPNonDecimalPoleRequiresExplicitBound(t *testing.T) {
	t.Parallel()
	q := func(a, b int64) *big.Rat { return big.NewRat(a, b) }
	s := geom.ExactBezierSpan{Start: 0, End: 1, Control: [][4]*big.Rat{
		{q(0, 1), q(0, 1), q(0, 1), q(1, 1)},
		{q(1, 7), q(1, 3), q(0, 1), q(1, 1)},
		{q(1, 1), q(1, 1), q(0, 1), q(1, 1)},
	}}
	for _, tol := range []*big.Rat{q(0, 1), q(1, 1000000000000000000), q(1, 100000000000000)} {
		calls := int64(0)
		id, err := emitClippedFuseSTEPBezier(func(string) int64 { calls++; return calls }, s, 2, tol)
		if tol.Cmp(q(1, 100000000000000)) < 0 {
			if !errors.Is(err, geom.ErrUnrepresentable) || calls != 0 || id != 0 {
				t.Fatalf("tolerance %s: entities %d result %d error %v", tol, calls, id, err)
			}
		} else if err != nil || calls != 4 || id != 4 {
			t.Fatalf("bounded decimal conversion: entities %d result %d error %v", calls, id, err)
		}
	}
}
