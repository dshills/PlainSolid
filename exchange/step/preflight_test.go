package step

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
)

func preflightFile(data string) string {
	return "ISO-10303-21;HEADER;FILE_DESCRIPTION(('inventory'),'2;1');FILE_NAME('','',(''),(''),'','','');FILE_SCHEMA(('AUTOMOTIVE_DESIGN'));ENDSEC;DATA;" + data + "ENDSEC;END-ISO-10303-21;"
}
func TestPreflightIndependentFixtures(t *testing.T) {
	paths, e := filepath.Glob("testdata/*.step")
	if e != nil || len(paths) < 20 {
		t.Fatal(e, len(paths))
	}
	for _, path := range paths {
		t.Run(filepath.Base(path), func(t *testing.T) {
			b, e := os.ReadFile(path)
			if e != nil {
				t.Fatal(e)
			}
			r, e := Preflight(bytes.NewReader(b), Limits{})
			if e != nil {
				t.Fatal(e)
			}
			if r.EntityCount == 0 || len(r.Types) == 0 || len(r.Unclassified) != 0 || len(r.Schemas) == 0 {
				t.Fatal(r)
			}
			again, e := Preflight(bytes.NewReader(b), Limits{})
			if e != nil || !reflect.DeepEqual(r, again) {
				t.Fatal("nondeterministic report", e)
			}
		})
	}
}
func TestPreflightUnknownAndComplex(t *testing.T) {
	raw := preflightFile("#8=UNSUPPORTED_SWEPT_PATCH('',#2);#2=(REPRESENTATION_ITEM('')B_SPLINE_SURFACE()FUTURE_SURFACE());#3=UNSUPPORTED_SWEPT_PATCH('',#2);")
	r, e := Preflight(strings.NewReader(raw), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	if r.EntityCount != 3 || r.ComplexEntityCount != 1 || len(r.Unclassified) != 2 || r.Unclassified[0].Type != "FUTURE_SURFACE" || r.Unclassified[1].Type != "UNSUPPORTED_SWEPT_PATCH" || !reflect.DeepEqual(r.Unclassified[1].EntityIDs, []int64{3, 8}) || !slices.Contains(r.Features, "spline-surfaces") {
		t.Fatal(r)
	}
	// Unknown entities must not prevent discovering a known complex component.
	// Conversely, a recognized name with invalid EXPRESS arguments is NOT valid geometry.
	r, e = Preflight(strings.NewReader(preflightFile("#1=MANIFOLD_SOLID_BREP();")), Limits{})
	if e != nil || len(r.Unclassified) != 0 || !slices.Contains(r.CandidateReaders, "ReadAnalyticBodies") {
		t.Fatal(r, e)
	}
	if _, e = ReadAnalyticBodies(strings.NewReader(preflightFile("#1=MANIFOLD_SOLID_BREP();")), Limits{}); e == nil {
		t.Fatal("strict reader accepted malformed geometry")
	}
}
func TestPreflightMalformedAndBounds(t *testing.T) {
	for _, raw := range []string{"", preflightFile("#1=UNKNOWN(#2);"), preflightFile("#1=UNKNOWN();#1=UNKNOWN();"), preflightFile("#1=UNKNOWN(1.E999);")} {
		if _, e := Preflight(strings.NewReader(raw), Limits{}); e == nil {
			t.Fatal(raw)
		}
	}
	raw := preflightFile("#1=UNKNOWN();#2=UNKNOWN();")
	for _, l := range []Limits{{MaxBytes: 32}, {MaxEntities: 1}, {MaxValues: 1}} {
		if _, e := Preflight(strings.NewReader(raw), l); e == nil {
			t.Fatal(l)
		}
	}
}
func FuzzPreflight(f *testing.F) {
	f.Add(preflightFile("#1=(REPRESENTATION_ITEM('')FUTURE_CURVE());"))
	f.Add(preflightFile("#1=MANIFOLD_SOLID_BREP();"))
	f.Fuzz(func(t *testing.T, raw string) {
		r, e := Preflight(strings.NewReader(raw), Limits{MaxBytes: 65536, MaxEntities: 512, MaxValues: 4096, MaxDepth: 16})
		if e == nil {
			for _, u := range r.Unclassified {
				if u.Feature != "" || len(u.EntityIDs) == 0 {
					t.Fatal(u)
				}
			}
		}
	})
}
