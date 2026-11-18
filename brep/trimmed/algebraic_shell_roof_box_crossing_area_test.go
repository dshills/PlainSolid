package trimmed

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
	"plainsolid/internal/slowtest"
	"testing"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

func TestDirectCrossingRoofArea(t *testing.T) {
	slowtest.Skip(t)
	ctx := context.Background()
	a, b, _, _ := mixedWallFixture(t)
	refs := map[string]float64{"intersection": 3.992955181787411, "union": 8.487908279091027, "difference": 8.057158869984429}
	if oracle := os.Getenv("PLAINSOLID_OCCT_CROSSING_ROOFS"); oracle != "" {
		b, err := exec.CommandContext(ctx, oracle).Output()
		if err != nil {
			t.Fatal(err)
		}
		in := json.NewDecoder(bytes.NewReader(b))
		seen := map[string]bool{}
		for {
			var row struct {
				Name string
				Area float64
			}
			if err := in.Decode(&row); err != nil {
				if !errors.Is(err, io.EOF) {
					t.Fatal(err)
				}
				break
			}
			if want, ok := refs[row.Name]; ok {
				if seen[row.Name] || math.Abs(want-row.Area) > 1e-7 {
					t.Fatal("independent OCCT area changed", row)
				}
				seen[row.Name], refs[row.Name] = true, row.Area
			}
		}
		if len(seen) != len(refs) {
			t.Fatal("missing independent OCCT Boolean results", seen)
		}
	}
	var boxes [2]*mixedRoofBox
	for i, shape := range []Solid{a, b} {
		var err error
		boxes[i], err = proveMixedRoofBoxMode(ctx, shape, 1, 2000000, true)
		if err != nil {
			t.Fatal(err)
		}
	}
	q := &MixedRoofMaterialQuery{box: boxes}
	for i, tc := range []struct {
		op   parametric.BooleanOp
		name string
	}{{parametric.Intersection, "intersection"}, {parametric.Union, "union"}, {parametric.Difference, "difference"}} {
		q.op = tc.op
		o := MixedRoofAreaOptions{AbsoluteGap: .1}
		r, err := q.BoundCrossingSurfaceArea(ctx, o)
		if err != nil {
			t.Fatal(tc.name, err)
		}
		oracle := new(big.Rat).SetFloat64(refs[tc.name])
		gap := new(big.Rat).Sub(r.Area.Upper, r.Area.Lower)
		lo, _ := r.Area.Lower.Float64()
		hi, _ := r.Area.Upper.Float64()
		t.Log(tc.name, "area interval", lo, hi, "cells", r.Cells, "exact operations", r.ExactOperations)
		if r.Area.Lower.Cmp(oracle) > 0 || r.Area.Upper.Cmp(oracle) < 0 || gap.Cmp(ratInset(o.AbsoluteGap)) > 0 || len(r.Faces) != 12 {
			t.Fatal("original surface area did not enclose OCCT", tc.name, lo, hi, refs[tc.name])
		}
		sum := mixedRoofZeroInterval()
		for j, f := range r.Faces {
			if f.Owner != (MixedRoofFaceOwner{j / 6, j % 6}) || f.Area.Lower.Sign() < 0 || f.Area.Lower.Cmp(f.Area.Upper) > 0 || f.Reversed != (i == 2 && j >= 6 && f.Retained) {
				t.Fatal("wrong retained or oriented original face", tc.name, f)
			}
			sum = addBooleanMeasureIntervals(sum, f.Area)
		}
		if sum.Lower.Cmp(r.Area.Lower) != 0 || sum.Upper.Cmp(r.Area.Upper) != 0 {
			t.Fatal("face ledger must sum to the exact total interval")
		}
		if i == 0 {
			fl := r.Faces[6+boxes[1].floorFace].Area
			if fl.Lower.Cmp(big.NewRat(1, 1)) != 0 || fl.Upper.Cmp(fl.Lower) != 0 {
				t.Fatal("intersection floor is the exact 1-square-unit footprint", fl)
			}
		}
	}
	if r, err := q.BoundCrossingSurfaceArea(ctx, MixedRoofAreaOptions{MaxCells: 1}); !errors.Is(err, geom.ErrConvergence) || r.Faces != nil {
		t.Fatal("an exhausted budget leaked a partial face ledger", r, err)
	}
	cancelled, stop := context.WithCancel(ctx)
	stop()
	if r, err := q.BoundCrossingSurfaceArea(cancelled, MixedRoofAreaOptions{}); !errors.Is(err, context.Canceled) || r.Faces != nil {
		t.Fatal("cancellation leaked a partial face ledger", r, err)
	}
	q.box[1] = q.box[0]
	if r, err := q.BoundCrossingSurfaceArea(ctx, MixedRoofAreaOptions{}); !errors.Is(err, geom.ErrUnrepresentable) || r.Faces != nil {
		t.Fatal("coincident original floors cannot be assigned two owners", r, err)
	}
}
