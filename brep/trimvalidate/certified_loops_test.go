package trimvalidate

import (
	"context"
	"errors"
	"math/big"
	"sync"
	"testing"
)

func TestCertifiedLoopsRetainExactTopology(t *testing.T) {
	ctx := context.Background()
	outer := rectangle(0, 0, 4, 4)
	hole := reverse(circle(2, 2, 1))
	proof, report, e := CertifyLoops(ctx, outer, []Loop{hole}, Options{})
	if e != nil || report.Status != Certified || proof == nil {
		t.Fatal(report, e)
	}
	q := func(x, y int64) [2]*big.Rat { return [2]*big.Rat{big.NewRat(x, 1), big.NewRat(y, 1)} }
	// Alter caller-owned segments after construction. Native lookups and an
	// independently certified uncached copy still locate the same geometry.
	originalOuter, originalHole := append(Loop(nil), outer...), append(Loop(nil), hole...)
	outer[0] = Segment{}
	hole[0] = Segment{}
	var wg sync.WaitGroup
	for _, item := range []struct {
		p    [2]*big.Rat
		want Location
	}{{q(0, 0), Boundary}, {q(2, 1), Boundary}, {q(1, 1), Inside}, {q(2, 2), Outside}, {q(5, 2), Outside}} {
		wg.Add(1)
		go func() {
			defer wg.Done()
			got, pr, e := proof.Locate(ctx, item.p)
			independent, _, ie := LocateRationalInLoops(ctx, originalOuter, []Loop{originalHole}, item.p, Options{})
			if e != nil || ie != nil || pr.Status != Certified || got != independent || got != item.want {
				t.Error(item.p, got, independent, pr, e, ie)
			}
		}()
	}
	wg.Wait()
	if got, r, e := proof.LocateBox(ctx, q(1, 1), q(2, 1)); e != nil || got != Unknown || r.Status != Unresolved {
		t.Fatal("box must not cross the boundary", got, r, e)
	}
	ctx, cancel := context.WithCancel(ctx)
	cancel()
	if _, _, e := proof.Locate(ctx, q(1, 1)); !errors.Is(e, context.Canceled) {
		t.Fatal(e)
	}
	if bad, r, _ := CertifyLoops(context.Background(), outer, []Loop{hole}, Options{}); bad != nil || r.Status == Certified {
		t.Fatal("certified malformed caller loops", r)
	}
}
