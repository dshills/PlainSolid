package step

import (
	"bytes"
	"strings"
	"testing"
)

func TestAnalyticSupportContextStrict(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name  string
		write func(*bytes.Buffer) error
		read  func([]byte) (int, error)
	}{
		{"cylinder", func(b *bytes.Buffer) error {
			m := cylinderSupportFixture()
			m.LengthUnit = .3048
			return WriteCylinderSupports(b, m)
		}, func(b []byte) (int, error) {
			m, e := ReadCylinderSupports(bytes.NewReader(b), Limits{})
			return len(m.Surfaces), e
		}},
		{"plane", func(b *bytes.Buffer) error {
			m := planeSupportFixture()
			m.LengthUnit = .3048
			return WritePlaneSupports(b, m)
		}, func(b []byte) (int, error) {
			m, e := ReadPlaneSupports(bytes.NewReader(b), Limits{})
			return len(m.Surfaces), e
		}},
		{"sphere", func(b *bytes.Buffer) error {
			m := sphereSupportFixture()
			m.LengthUnit = .3048
			return WriteSphereSupports(b, m)
		}, func(b []byte) (int, error) {
			m, e := ReadSphereSupports(bytes.NewReader(b), Limits{})
			return len(m.Surfaces), e
		}},
		{"torus", func(b *bytes.Buffer) error {
			m := torusSupportFixture()
			m.LengthUnit = .3048
			return WriteTorusSupports(b, m)
		}, func(b []byte) (int, error) {
			m, e := ReadTorusSupports(bytes.NewReader(b), Limits{})
			return len(m.Surfaces), e
		}},
	}
	mutations := [][3]string{
		{"rounded dimension", "GEOMETRIC_REPRESENTATION_CONTEXT(3)", "GEOMETRIC_REPRESENTATION_CONTEXT(3.00000000000000001)"},
		{"angular prefix", "SI_UNIT($,.RADIAN.)", "SI_UNIT(.MILLI.,.RADIAN.)"},
		{"angle dimension", "SI_UNIT($,.RADIAN.)", "SI_UNIT($,.METRE.)"},
		{"angle arity", "PLANE_ANGLE_UNIT()", "PLANE_ANGLE_UNIT(0)"},
		{"duplicate angular category", "PLANE_ANGLE_UNIT()", "SOLID_ANGLE_UNIT()"},
		{"solid angle prefix", "SI_UNIT($,.STERADIAN.)", "SI_UNIT(.MILLI.,.STERADIAN.)"},
		{"rounded dimensional exponent", "DIMENSIONAL_EXPONENTS(1.,", "DIMENSIONAL_EXPONENTS(1.00000000000000001,"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var b bytes.Buffer
			if e := c.write(&b); e != nil {
				t.Fatal(e)
			}
			if n, e := c.read(b.Bytes()); e != nil || n != 1 {
				t.Fatal(n, e)
			}
			for _, mut := range mutations {
				t.Run(mut[0], func(t *testing.T) {
					raw := b.String()
					if strings.Count(raw, mut[1]) != 1 {
						t.Fatal("stale mutation", mut)
					}
					n, e := c.read([]byte(strings.Replace(raw, mut[1], mut[2], 1)))
					if e == nil || n != 0 {
						t.Fatalf("malformed context accepted: count=%d error=%v", n, e)
					}
				})
			}
		})
	}
}
