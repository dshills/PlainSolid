package exchange_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"testing"

	"plainsolid/brep/trimmed"
	"plainsolid/exchange/iges"
	"plainsolid/exchange/step"
	"plainsolid/geom"
)

func TestCuspidalCubicCreasedFuseExchange(t *testing.T) {
	ctx := context.Background()
	const name, units = "Cuspidal Creased Fuse", .001
	const volume = 279423.0/22528 + 729.0/4096
	so := step.CuspidalCubicSTEPOptions{PoleTolerance: 1e-14}
	iopts := iges.CuspidalCubicIGESOptions{PoleTolerance: 1e-14}
	for order := range 2 {
		t.Run(fmt.Sprint(order), func(t *testing.T) {
			raw, err := os.ReadFile(fmt.Sprintf("testdata/native_cuspidal_cubic_creased_fuse_order_%d.json", order))
			if err != nil {
				t.Fatal(err)
			}
			fuse, err := trimmed.ReadCuspidalCubicFuseJSON(ctx, bytes.NewReader(raw), trimmed.JSONLimits{}, trimmed.CuspidalCubicFuseOptions{})
			if err != nil {
				t.Fatal(err)
			}
			var st, ig bytes.Buffer
			if err := step.WriteCuspidalCubicFuse(ctx, &st, fuse, name, units, step.Limits{}, so); err != nil {
				t.Fatal(err)
			}
			if err := iges.WriteCuspidalCubicFuse(ctx, &ig, fuse, units, iges.Limits{}, iopts); err != nil {
				t.Fatal(err)
			}
			if err := step.VerifyCuspidalCubicFuseSTEP(ctx, bytes.NewReader(st.Bytes()), fuse, name, units, step.Limits{}, so); err != nil {
				t.Fatal(err)
			}
			if err := iges.VerifyCuspidalCubicFuseIGES(ctx, bytes.NewReader(ig.Bytes()), fuse, units, iges.Limits{}, iopts); err != nil {
				t.Fatal(err)
			}
			changed := bytes.Replace(st.Bytes(), []byte(".CURVE_3D."), []byte(".PCURVE_S1."), 1)
			if bytes.Equal(changed, st.Bytes()) ||
				!errors.Is(step.VerifyCuspidalCubicFuseSTEP(ctx, bytes.NewReader(changed), fuse, name, units, step.Limits{}, so), geom.ErrConvergence) {
				t.Fatal("changed creased Fuse model/owner curve preference passed verification")
			}
			checkCuspidalCreasedReadback(t, ctx, fuse.Boundary(), fuse.History().Vertices[0],
				[3]int{9, 24, 17}, st.Bytes(), ig.Bytes(), volume)
		})
	}
}
