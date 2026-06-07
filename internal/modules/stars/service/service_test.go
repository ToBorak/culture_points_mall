package service

import "testing"

func TestAwardable(t *testing.T) {
	cases := []struct {
		count int64
		per   int
		cap   int
		want  bool
	}{
		{0, 2, 6, true},  // 第1次提报 0->2 <=6
		{2, 2, 6, true},  // 第3次 4->6 <=6
		{3, 2, 6, false}, // 第4次 6->8 >6
		{3, 4, 16, true}, // 第4次被提名 12->16 <=16
		{4, 4, 16, false},// 第5次 16->20 >16
	}
	for _, c := range cases {
		if got := awardable(c.count, c.per, c.cap); got != c.want {
			t.Fatalf("awardable(%d,%d,%d)=%v want %v", c.count, c.per, c.cap, got, c.want)
		}
	}
}
