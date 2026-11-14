package geom

import (
	"errors"
	"testing"
)

func TestVerifiedTubeConstructorReport(t *testing.T) {
	for _, rational := range []bool{false, true} {
		source := affineTubeLineFixture(t, rational)
		options := SurfacePairTubeOptions{FreeAxis: source.axis}
		direct, e := CertifySurfacePairTube(source.a, source.b, options)
		if e != nil || !direct.ProvenBranch {
			t.Fatal(direct, e)
		}
		tube, report, e := NewVerifiedSurfaceTubeWithCertificate(source.a, source.b, options)
		if e != nil || report != direct || tube.FreeTangentSign() != direct.FreeTangentSign {
			t.Fatal(report, direct, e)
		}
		t.Logf("rational=%v exact work: old report+constructor=%d, combined=%d", rational, 2*report.Test.ExactOperations, report.Test.ExactOperations)
		n := report.Test.ExactOperations
		// Public diagnostics are entirely detached from immutable tube provenance.
		report = SurfaceTubeCertificate{FreeAxis: 3, FreeTangentSign: -tube.FreeTangentSign()}
		got, e := tube.Evaluate(SurfaceTubeEvaluationOptions{Parameter: .5})
		if e != nil || !got.Enclosed || tube.FreeTangentSign() != direct.FreeTangentSign {
			t.Fatal(got, e)
		}
		for _, limit := range []int{n, n - 1} {
			options.MaxOperations = limit
			q, r, e := NewVerifiedSurfaceTubeWithCertificate(source.a, source.b, options)
			if limit == n {
				if e != nil || !r.ProvenBranch || q.FreeTangentSign() == 0 || r.Test.ExactOperations != n {
					t.Fatal(limit, r, e)
				}
			} else if !errors.Is(e, ErrConvergence) || q != (VerifiedSurfaceTube{}) {
				t.Fatal("partial tube on exhausted proof", q, r, e)
			}
		}
	}
}

func TestVerifiedTubeConstructorReportRejectsUnproved(t *testing.T) {
	source := affineTubeLineFixture(t, false)
	// Identical patches have no regular unique one-dimensional intersection.
	tube, report, e := NewVerifiedSurfaceTubeWithCertificate(source.a, source.a, SurfacePairTubeOptions{})
	if e == nil || tube != (VerifiedSurfaceTube{}) || report.ProvenBranch {
		t.Fatal(tube, report, e)
	}
	tube, _, e = NewVerifiedSurfaceTubeWithCertificate(nil, source.b, SurfacePairTubeOptions{})
	if !errors.Is(e, ErrInvalid) || tube != (VerifiedSurfaceTube{}) {
		t.Fatal(tube, e)
	}
}
