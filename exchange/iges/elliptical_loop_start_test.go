package iges

import (
	"fmt"
	"reflect"
	"strings"
	"testing"
)

func TestEllipticalDirectedLoopRotations(t *testing.T) {
	base := []string{"0,23,1,0,0", "0,23,2,0,0", "0,23,3,1,0", "0,23,2,1,0"}
	decode := func(parts []string) topologyRecord {
		t.Helper()
		budget := 100
		v, e := parseParameters("508,4,"+strings.Join(parts, ",")+";", ',', ';', &budget)
		if e != nil {
			t.Fatal(e)
		}
		return topologyRecord{id: 25, kind: 508, values: v}
	}
	for shift := 0; shift < 4; shift++ {
		t.Run(fmt.Sprint(shift), func(t *testing.T) {
			parts := append(append([]string(nil), base[shift:]...), base[:shift]...)
			records := map[int]topologyRecord{25: decode(parts)}
			roles, e := ellipseLoopStarts(records, false)
			if e != nil {
				t.Fatal(e)
			}
			want := []int{}
			for j := 0; j < 4; j++ {
				want = append(want, (j-shift+4)%4+1)
			}
			if !reflect.DeepEqual(roles[25], want) || !reflect.DeepEqual(records[25].values, decode(base).values) {
				t.Fatal("lost directed cycle", roles)
			}
		})
	}
	for _, bad := range [][]string{
		{base[3], base[2], base[1], base[0]},
		{"0,23,1,1,0", base[1], base[2], base[3]},
		{base[0], base[0], base[2], base[3]},
	} {
		if _, e := ellipseLoopStarts(map[int]topologyRecord{25: decode(bad)}, false); e == nil {
			t.Fatal("reversed or altered cycle accepted")
		}
	}
}
