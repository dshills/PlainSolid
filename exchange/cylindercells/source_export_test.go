package cylindercells

import (
	"bytes"
	"errors"
	"io"
	"plainsolid/exchange/step"
	"regexp"
	"strings"
	"testing"
)

func reverseSourceBounds(s string) string {
	edge := regexp.MustCompile(`ORIENTED_EDGE\(([^\n]*)\.(T|F)\.\)`)
	s = edge.ReplaceAllStringFunc(s, func(v string) string {
		if strings.HasSuffix(v, ".T.)") {
			return strings.TrimSuffix(v, ".T.)") + ".F.)"
		}
		return strings.TrimSuffix(v, ".F.)") + ".T.)"
	})
	loop := regexp.MustCompile(`EDGE_LOOP\('',\((#[0-9]+(?:,#[0-9]+)*)\)\)`)
	s = loop.ReplaceAllStringFunc(s, func(v string) string {
		m := loop.FindStringSubmatch(v)
		ids := strings.Split(m[1], ",")
		for i, j := 0, len(ids)-1; i < j; i, j = i+1, j-1 {
			ids[i], ids[j] = ids[j], ids[i]
		}
		return "EDGE_LOOP('',(" + strings.Join(ids, ",") + "))"
	})
	bound := regexp.MustCompile(`FACE_OUTER_BOUND\('',(#[0-9]+),\.T\.\)`)
	return bound.ReplaceAllString(s, "FACE_OUTER_BOUND('',$1,.F.)")
}
func TestCylinderOriginalVersusCanonicalExport(t *testing.T) {
	for _, dialect := range []string{Half, Quarter} {
		a, e := FromNative(native(t), "source named part", SIUnit{-3}, dialect)
		if e != nil {
			t.Fatal(e)
		}
		var b bytes.Buffer
		if e = WriteCanonicalSTEP(&b, a); e != nil {
			t.Fatal(e)
		}
		canonical := b.String()
		authored := "  \n" + reverseSourceBounds(canonical) + "\n  "
		if authored == canonical {
			t.Fatal("missing alternate encoding")
		}
		a, e = FromSTEP(strings.NewReader(authored), dialect)
		if e != nil {
			t.Fatal(e)
		}
		b.Reset()
		if e = WriteOriginalSTEP(&b, a); e != nil || b.String() != authored {
			t.Fatal("source changed", e)
		}
		b.Reset()
		if e = WriteCanonicalSTEP(&b, a); e != nil || b.String() != canonical {
			t.Fatal("canonical regeneration", e)
		}
		legacy := new(bytes.Buffer)
		if e = WriteSTEP(legacy, a); e != nil || legacy.String() != canonical {
			t.Fatal("legacy export behavior", e)
		}
		b.Reset()
		if e = WriteNative(&b, a); e != nil {
			t.Fatal(e)
		}
		a, e = ReadNative(&b)
		if e != nil {
			t.Fatal(e)
		}
		if e = WriteOriginalSTEP(&b, a); e != nil || b.String() != authored {
			t.Fatal("source lost through persistence", e)
		}
	}
}

type failedSourceWriter struct{ short bool }

func (w failedSourceWriter) Write(b []byte) (int, error) {
	if w.short {
		return len(b) - 1, nil
	}
	return 0, io.ErrClosedPipe
}
func TestCylinderOriginalExportReject(t *testing.T) {
	a, e := FromNative(native(t), "source", SIUnit{-3}, Half)
	if e != nil {
		t.Fatal(e)
	}
	var b bytes.Buffer
	if e = WriteOriginalSTEP(&b, a); !errors.Is(e, ErrNoSource) || b.Len() != 0 {
		t.Fatal(e)
	}
	if e = WriteCanonicalSTEP(&b, a); e != nil {
		t.Fatal(e)
	}
	a, e = FromSTEP(&b, Half)
	if e != nil {
		t.Fatal(e)
	}
	bad := a
	bad.Name = "stale"
	if e = WriteOriginalSTEP(&b, bad); e == nil || b.Len() != 0 {
		t.Fatal("stale evidence")
	}
	if e = WriteOriginalSTEP(failedSourceWriter{short: true}, a); !errors.Is(e, io.ErrShortWrite) {
		t.Fatal(e)
	}
	if e = WriteOriginalSTEP(failedSourceWriter{}, a); !errors.Is(e, io.ErrClosedPipe) {
		t.Fatal(e)
	}
	var bare bytes.Buffer
	d, e := step.NativeSplitCylinderDescriptor(native(t), "source", .001)
	if e != nil {
		t.Fatal(e)
	}
	if e = step.WriteSplitCylinderModel(&bare, d); e != nil {
		t.Fatal(e)
	}
	if _, e = FromSTEP(&bare, Half); e == nil {
		t.Fatal("bare evidence silently rooted")
	}
}
