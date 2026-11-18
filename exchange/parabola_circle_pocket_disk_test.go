package exchange_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"testing"

	"plainsolid/brep/edgecheck"
	"plainsolid/brep/parametric"
	"plainsolid/brep/trimmed"
	"plainsolid/exchange/iges"
	"plainsolid/exchange/step"
	"plainsolid/geom"
)

func TestParabolaCirclePocketDiskExchange(t *testing.T) {
	ctx := context.Background()
	full := os.Getenv("PLAINSOLID_OCCT_POCKET_DISK_STEP_OWNERS") != "" || os.Getenv("PLAINSOLID_OCCT_POCKET_DISK_IGES_OWNERS") != "" ||
		os.Getenv("PLAINSOLID_OCCT_CLIPPED_CUT_STEP_COMPONENTS") != "" || os.Getenv("PLAINSOLID_OCCT_CLIPPED_CUT_IGES") != ""
	for layout := range 2 {
		for axis := range 2 {
			if !full && axis != layout {
				continue
			}
			stock := pocketDiskExchangeStocks(t, axis, layout)
			for order := range 2 {
				t.Run(fmt.Sprintf("%d/%d/%d", layout, axis, order), func(t *testing.T) {
					t.Parallel()
					src := [3]*geom.BSplineSurface{stock[0], stock[1+order], stock[2-order]}
					if full || order == 0 {
						for i, op := range [2]parametric.BooleanOp{parametric.Intersection, parametric.Union} {
							name := [2]string{"common", "fuse"}[i]
							t.Run(name, func(t *testing.T) {
								checkPocketDiskEnvelopeExchange(t, ctx, src, axis, layout, order, i, name, op)
							})
						}
					}
					t.Run("cut", func(t *testing.T) {
						checkPocketDiskCutExchange(t, ctx, src, axis, layout, order)
					})
				})
			}
		}
	}
}

func pocketDiskExchangeStocks(t *testing.T, axis, layout int) [3]*geom.BSplineSurface {
	t.Helper()
	knots := append(make([]float64, 9), 1, 1, 1, 1, 1, 1, 1, 1, 1)
	var stocks [3]*geom.BSplineSurface
	for owner := range stocks {
		poles := make([]geom.Vec3, 0, 81)
		for i := range 9 {
			for j := range 9 {
				free, dependent := i, j
				if axis == 1 {
					free, dependent = j, i
				}
				z := 0.0
				if owner == 1 {
					z = 2
				}
				if owner == 2 {
					z = pocketDiskHeightPoles[free][dependent]
					if layout == 1 {
						z = 3 - pocketDiskHeightPoles[free][8-dependent]/2
					}
				}
				poles = append(poles, geom.Vec3{X: float64(i) / 8, Y: float64(j) / 8, Z: z})
			}
		}
		var err error
		stocks[owner], err = geom.NewBSplineSurface(8, 8, 9, 9, poles, knots, knots, nil)
		if err != nil {
			t.Fatal(err)
		}
	}
	return stocks
}

func checkPocketDiskEnvelopeExchange(t *testing.T, ctx context.Context, src [3]*geom.BSplineSurface, axis, layout, order, i int,
	name string, operation parametric.BooleanOp) {
	t.Helper()
	const unit = .001
	proof := edgecheck.Options{}
	r, err := trimmed.CombineParabolaCirclePocketDiskSplineGraphs(ctx, src[0], src[1], src[2], axis, operation, 1e-9, trimmed.ParabolaCircleGraphOptions{})
	if err != nil {
		t.Fatal(err)
	}
	var st, ig bytes.Buffer
	if err := step.WriteParabolaCirclePocketDiskEnvelope(ctx, &st, r, name, unit, step.Limits{}, proof); err != nil {
		t.Fatal(err)
	}
	if err := iges.WriteParabolaCirclePocketDiskEnvelope(ctx, &ig, r, unit, iges.Limits{}, proof); err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(st.Bytes(), []byte("TRIMMED_CURVE")) {
		t.Fatal("missing original large circle's exact carrier")
	}
	body, history := r.Boundary(), r.History()
	if axis == layout && order == 0 {
		checkExactPocketDiskRim(t, ctx, body, axis, layout, history.DiskEdges, 1)
		if err := step.VerifyParabolaCirclePocketDiskEnvelopeSTEP(ctx, bytes.NewReader(st.Bytes()), r, name, unit, step.Limits{}, proof); err != nil {
			t.Fatal(err)
		}
		if err := iges.VerifyParabolaCirclePocketDiskEnvelopeIGES(ctx, bytes.NewReader(ig.Bytes()), r, unit, iges.Limits{}, proof); err != nil {
			t.Fatal(err)
		}
	}
	// Independent exact Green integrals over the original seven
	// regions, including the disk's full-circle polar moments.
	volume := [2][2]float64{{1.9524360713221687, 2.159320603153823}, {1.9203396984230885, 2.0237819643389159}}
	checkTwoCirclesExchangeReadback(t, ctx, []trimmed.Solid{body}, []float64{volume[layout][i]}, st.Bytes(), ig.Bytes())
	checkPocketDiskPinnedOwners(t, ctx, axis, layout, order, name, 1, 1, st.Bytes(), ig.Bytes())
}

func checkPocketDiskCutExchange(t *testing.T, ctx context.Context, src [3]*geom.BSplineSurface, axis, layout, order int) {
	t.Helper()
	const name, unit = "Nested original parabola circle pocket Cut", .001
	proof := edgecheck.Options{}
	r, err := trimmed.CutParabolaCirclePocketDiskSplineGraphs(ctx, src[0], src[1], src[2], axis, 1e-9, trimmed.ParabolaCircleGraphOptions{})
	if err != nil {
		t.Fatal(err)
	}
	var st, ig bytes.Buffer
	stepErr := step.WriteParabolaCirclePocketDiskCut(ctx, &st, r, name, unit, step.Limits{}, proof)
	igesErr := iges.WriteParabolaCirclePocketDiskCut(ctx, &ig, r, unit, iges.Limits{}, proof)
	if order == layout {
		if len(r.Components()) != 3 || r.DiskSignedVolumePiCoefficient().Sign() != 1 ||
			!errors.Is(stepErr, geom.ErrUnrepresentable) || st.Len() != 0 || !errors.Is(igesErr, geom.ErrUnrepresentable) || ig.Len() != 0 {
			t.Fatal("isolated outward disk transfer must fail atomically", stepErr, igesErr, st.Len(), ig.Len())
		}
		return
	}
	if stepErr != nil || igesErr != nil {
		t.Fatal(stepErr, igesErr)
	}
	parts, history := r.Components(), r.History()
	if len(parts) != 4 || history[1].Region != 2 {
		t.Fatal("lost the original four-cycle or holed pocket", history)
	}
	if axis == layout {
		checkExactPocketDiskRim(t, ctx, parts[1], axis, layout, history[1].DiskEdges, 2)
		if err := step.VerifyParabolaCirclePocketDiskCutSTEP(ctx, bytes.NewReader(st.Bytes()), r, name, unit, step.Limits{}, proof); err != nil {
			t.Fatal(err)
		}
		if err := iges.VerifyParabolaCirclePocketDiskCutIGES(ctx, bytes.NewReader(ig.Bytes()), r, unit, iges.Limits{}, proof); err != nil {
			t.Fatal(err)
		}
		var blocked bytes.Buffer
		if err := step.WriteParabolaCirclePocketDiskCut(ctx, &blocked, r, name, unit, step.Limits{MaxSolids: 3}, proof); !errors.Is(err, geom.ErrConvergence) || blocked.Len() != 0 {
			t.Fatal("published an incomplete STEP contact cycle", err)
		}
		prefix, err := parts[0].ExtractExactBezierBoundary(ctx, proof)
		if err != nil {
			t.Fatal(err)
		}
		controls := countTwoCirclesExchangeControls(t, prefix)
		if err := iges.WriteParabolaCirclePocketDiskCut(ctx, &blocked, r, unit, iges.Limits{MaxControlPoints: controls}, proof); !errors.Is(err, geom.ErrConvergence) || blocked.Len() != 0 {
			t.Fatal("published an incomplete IGES contact cycle", err)
		}
	}
	volumes := []float64{.159320601907153, 2.3438490860857207e-10, 5.0614246903741087e-10, 5.0614246903741087e-10}
	if layout != 0 {
		for i := range volumes {
			volumes[i] /= 2
		}
	}
	checkTwoCirclesExchangeReadback(t, ctx, parts, volumes, st.Bytes(), ig.Bytes())
	checkPocketDiskPinnedOwners(t, ctx, axis, layout, order, "cut", 4, 2, st.Bytes(), ig.Bytes())
}

func checkExactPocketDiskRim(t *testing.T, ctx context.Context, body trimmed.Solid, axis, layout int, edges [4]int, inner int) {
	t.Helper()
	b, err := body.ExtractExactBezierBoundary(ctx, edgecheck.Options{})
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range edges {
		checkExactPocketDiskCircleLaw(t, b.Edges[id].Model, axis, layout, 2)
		owners := make(map[*geom.BSplineSurface]bool, 2)
		directions := [2]int{}
		holes := 0
		for _, face := range b.Faces {
			for loop, wire := range face.Loops {
				for _, use := range wire {
					if use.Edge != id {
						continue
					}
					if face.Spline == nil || owners[face.Spline] {
						t.Fatal("wrong original disk trim owner", id)
					}
					owners[face.Spline] = true
					if use.Reversed != face.Reversed {
						directions[1]++
					} else {
						directions[0]++
					}
					if loop > 0 {
						holes++
					}
					checkExactPocketDiskCircleLaw(t, use.UV, axis, layout, 0)
				}
			}
		}
		if len(owners) != 2 || directions != ([2]int{1, 1}) || holes != inner {
			t.Fatal("wrong original opposing rim and hole coedges", id, directions, holes)
		}
	}
}

func checkExactPocketDiskCircleLaw(t *testing.T, span geom.ExactBezierSpan, axis, layout int, z int64) {
	t.Helper()
	if len(span.Control) != 3 || span.Start != 0 || span.End != 1 {
		t.Fatal("lost exact original quarter law or active parameter range")
	}
	cy := big.NewRat(21, 128)
	if layout == 1 {
		cy.Sub(big.NewRat(1, 1), cy)
	}
	// A homogeneous quadratic circle identity has degree four.
	for i := range int64(5) {
		weights := [3]int64{(4 - i) * (4 - i), 2 * i * (4 - i), i * i}
		var p [4]big.Rat
		for dim := range p {
			for j, weight := range weights {
				p[dim].Add(&p[dim], new(big.Rat).Mul(span.Control[j][dim], big.NewRat(weight, 1)))
			}
		}
		x := new(big.Rat).Quo(&p[axis], &p[3])
		x.Sub(x, big.NewRat(1, 2)).Mul(x, x)
		y := new(big.Rat).Quo(&p[1-axis], &p[3])
		y.Sub(y, cy).Mul(y, y)
		if x.Add(x, y).Cmp(big.NewRat(1, 262144)) != 0 || new(big.Rat).Quo(&p[2], &p[3]).Cmp(big.NewRat(z, 1)) != 0 {
			t.Fatal("changed original exact small circle law")
		}
	}
}

func checkPocketDiskPinnedOwners(t *testing.T, ctx context.Context, axis, layout, order int, operation string, solids, holes int, st, ig []byte) {
	t.Helper()
	for _, f := range []struct {
		reader, extension string
		data              []byte
	}{
		{os.Getenv("PLAINSOLID_OCCT_POCKET_DISK_STEP_OWNERS"), "step", st},
		{os.Getenv("PLAINSOLID_OCCT_POCKET_DISK_IGES_OWNERS"), "igs", ig},
	} {
		if f.reader == "" {
			continue
		}
		path := filepath.Join(t.TempDir(), "pocket-disk."+f.extension)
		if err := os.WriteFile(path, f.data, 0o600); err != nil {
			t.Fatal(err)
		}
		out, err := exec.CommandContext(ctx, f.reader, path, strconv.Itoa(axis), strconv.Itoa(layout), strconv.Itoa(order), operation).CombinedOutput()
		if err != nil {
			t.Fatalf("%s original owner audit: %v\n%s", f.extension, err, out)
		}
		var report struct {
			Solids, Coedges, Rays, RoofPoles, RingJets, Holes int
			Pairs                                             [4]int
			Subdivisions                                      [3]int
		}
		first := bytes.IndexByte(out, '{')
		if first < 0 || json.Unmarshal(out[first:], &report) != nil {
			t.Fatalf("%s malformed original owner audit: %s", f.extension, out)
		}
		wantPairs := [4]int{}
		poles := 7 * 81
		if operation == "cut" {
			wantPairs = [4]int{1, 1, 1, 1}
			poles = 8 * 81
		}
		if report.Solids != solids || report.Coedges != 44 || report.Rays != 32 || report.RoofPoles != poles ||
			report.RingJets != 16 || report.Holes != holes || report.Pairs != wantPairs || report.Subdivisions != ([3]int{4, 4, 4}) {
			t.Fatalf("%s changed original owner and rim ledger: %+v", f.extension, report)
		}
		t.Logf("%s original owner and rim ledger %+v", f.extension, report)
	}
}

// Exact Bernstein coefficients of 2 + (245/16)(y-p)QcrossQdisk.
// The source law and its signed region integrals are independent of exchange.
var pocketDiskHeightPoles = [9][9]float64{
	{0x1.d1d1418952150p-1, 0x1.a0a3ecfa608a8p0, 0x1.c3d3c243380a8p0, 0x1.d56eea1f2f8a8p0, 0x1.e91b2e0e470a8p0, 0x1.e92d57907e8a8p0, 0x1.dca93025d60a8p0, 0x1.16a0c0a726c54p+1, 0x1.f6ac0a44f2854p+1},
	{0x1.1515cb1efb054p+1, 0x1.28e8b0f2d6c54p+1, 0x1.1b08d77042854p+1, 0x1.1892a3573e454p+1, 0x1.1df9f967ca054p+1, 0x1.1a0a3e61e5c54p+1, 0x1.10e6570591854p+1, 0x1.3f08a812cd454p+1, 0x1.1e218b24cc82ap+2},
	{0x1.f89f4cb92d0a8p0, 0x1.f9564ed6e48a8p0, 0x1.dc9e5f07bc0a8p0, 0x1.df9646cbb38a8p0, 0x1.e80bcfa2cb0a8p0, 0x1.ca7bc30d028a8p0, 0x1.9011ea8a5a0a8p0, 0x1.bca90f9ad18a8p0, 0x1.ca657ddf34854p+1},
	{0x1.f74906364e0a8p0, 0x1.fa3bf5ce058a8p0, 0x1.ee8714b8dd0a8p0, 0x1.ffd32c76d48a8p0, 0x1.043c0343f6054p+1, 0x1.d77c6c6c238a8p0, 0x1.769627a37b0a8p0, 0x1.702a01adf28a8p0, 0x1.8aa5e205c5054p+1},
	{0x1.04c9285aac854p+1, 0x1.08c3f26588454p+1, 0x1.0736aef9f4054p+1, 0x1.130cc2d7efc54p+1, 0x1.178912bf7b854p+1, 0x1.f68c06e12e8a8p0, 0x1.866af356860a8p0, 0x1.6b41b45efd8a8p0, 0x1.7d2909bd4a854p+1},
	{0x1.f74906364e0a8p0, 0x1.fa3bf5ce058a8p0, 0x1.ee8714b8dd0a8p0, 0x1.ffd32c76d48a8p0, 0x1.043c0343f6054p+1, 0x1.d77c6c6c238a8p0, 0x1.769627a37b0a8p0, 0x1.702a01adf28a8p0, 0x1.8aa5e205c5054p+1},
	{0x1.f89f4cb92d0a8p0, 0x1.f9564ed6e48a8p0, 0x1.dc9e5f07bc0a8p0, 0x1.df9646cbb38a8p0, 0x1.e80bcfa2cb0a8p0, 0x1.ca7bc30d028a8p0, 0x1.9011ea8a5a0a8p0, 0x1.bca90f9ad18a8p0, 0x1.ca657ddf34854p+1},
	{0x1.1515cb1efb054p+1, 0x1.28e8b0f2d6c54p+1, 0x1.1b08d77042854p+1, 0x1.1892a3573e454p+1, 0x1.1df9f967ca054p+1, 0x1.1a0a3e61e5c54p+1, 0x1.10e6570591854p+1, 0x1.3f08a812cd454p+1, 0x1.1e218b24cc82ap+2},
	{0x1.d1d1418952150p-1, 0x1.a0a3ecfa608a8p0, 0x1.c3d3c243380a8p0, 0x1.d56eea1f2f8a8p0, 0x1.e91b2e0e470a8p0, 0x1.e92d57907e8a8p0, 0x1.dca93025d60a8p0, 0x1.16a0c0a726c54p+1, 0x1.f6ac0a44f2854p+1},
}
