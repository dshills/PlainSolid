package analytic

import (
	"bytes"
	"reflect"
	"strings"
	"testing"
)

func TestNativeMixedRoundtrip(t *testing.T) {
	s, e := FromIGES(fixture(t))
	if e != nil {
		t.Fatal(e)
	}
	m, e := FromSTEP(s)
	if e != nil {
		t.Fatal(e)
	}
	var b bytes.Buffer
	if e = WriteNative(&b, m); e != nil {
		t.Fatal(e)
	}
	raw := append([]byte(nil), b.Bytes()...)
	n, e := ReadNative(&b)
	if e != nil || !reflect.DeepEqual(m, n) {
		t.Fatal(n, e)
	}
	out, e := n.STEP()
	if e != nil || !reflect.DeepEqual(s, out) {
		t.Fatal(out, e)
	}
	if e = WriteNative(&b, n); e != nil || !bytes.Equal(raw, b.Bytes()) {
		t.Fatal(e)
	}
	n.Supports[0].Name = "changed"
	if m.Supports[0].Name == "changed" {
		t.Fatal("aliased")
	}
}
func TestNativeMixedReject(t *testing.T) {
	s, _ := FromIGES(fixture(t))
	m, _ := FromSTEP(s)
	var b bytes.Buffer
	_ = WriteNative(&b, m)
	raw := b.String()
	bad := []string{
		strings.Replace(raw, `"Version":1`, `"Version":2`, 1),
		strings.Replace(raw, `"Version":1`, `"Version":1,"version":1`, 1),
		strings.Replace(raw, `"Version":1`, `"Version":1,"Unknown":0`, 1),
		strings.Replace(raw, `"Kind":"torus"`, `"Kind":"cone"`, 1),
		strings.Replace(raw, `"Radius":4`, `"Radius":0`, 1),
		strings.Replace(raw, `"LengthUnit":0.001`, `"LengthUnit":0`, 1),
		raw + `{}`, `null`, strings.Repeat("[", 10) + strings.Repeat("]", 10), strings.Repeat(" ", MaxNativeBytes+1),
	}
	for i, x := range bad {
		if x == raw {
			t.Fatalf("ineffective mutation %d", i)
		}
		v, e := ReadNative(strings.NewReader(x))
		if e == nil || len(v.Supports) != 0 {
			t.Fatalf("accepted %d: %v", i, e)
		}
	}
	m.Supports[0].Axis.X = .5
	b.Reset()
	if e := WriteNative(&b, m); e == nil || b.Len() != 0 {
		t.Fatal("non-atomic invalid geometry")
	}
}
func FuzzNativeMixed(f *testing.F) {
	f.Add([]byte(`{"Format":"plainsolid-analytic-supports","Version":1,"Model":{}}`))
	f.Fuzz(func(t *testing.T, b []byte) {
		if len(b) > MaxNativeBytes {
			return
		}
		m, e := ReadNative(bytes.NewReader(b))
		if e == nil {
			var out bytes.Buffer
			if e = WriteNative(&out, m); e != nil {
				t.Fatal(e)
			}
		}
	})
}
