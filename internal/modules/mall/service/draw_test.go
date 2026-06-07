package service

import (
	"testing"

	"github.com/standardsoftware/culture_points_mall/internal/modules/mall/repository"
)

func TestPickIndexByWeight(t *testing.T) {
	weights := []int{60, 25, 10, 5} // 累积区间 [0,60) [60,85) [85,95) [95,100)
	cases := []struct {
		x    int
		want int
	}{
		{0, 0}, {59, 0},
		{60, 1}, {84, 1},
		{85, 2}, {94, 2},
		{95, 3}, {99, 3},
	}
	for _, c := range cases {
		if got := pickIndexByWeight(weights, c.x); got != c.want {
			t.Errorf("pickIndexByWeight(%d) = %d, want %d", c.x, got, c.want)
		}
	}
}

func TestPickIndexByWeight_SkipsZeroWeight(t *testing.T) {
	// 权重为 0 的奖项永远抽不到
	weights := []int{0, 10, 0}
	for x := 0; x < 10; x++ {
		if got := pickIndexByWeight(weights, x); got != 1 {
			t.Errorf("x=%d got %d want 1", x, got)
		}
	}
}

func intp(v int) *int { return &v }

func TestDrawablePrizes_FiltersDepletedStock(t *testing.T) {
	prizes := []repository.PrizeView{
		{ID: 1, Stock: nil},     // 不限量 → 保留
		{ID: 2, Stock: intp(0)}, // 0 份 → 移出
		{ID: 3, Stock: intp(2)}, // 仍有份数 → 保留
	}
	got := drawablePrizes(prizes)
	if len(got) != 2 {
		t.Fatalf("want 2 drawable, got %d", len(got))
	}
	if got[0].ID != 1 || got[1].ID != 3 {
		t.Errorf("unexpected drawable set: %+v", got)
	}
}

func TestPickPrize_WinDetectionByItemID(t *testing.T) {
	// 仅「无奖品」行（ItemID=nil，权重 100）→ 必抽中它且判定为未中奖
	only := []repository.PrizeView{{ID: 9, ItemID: nil, PrizeName: "谢谢参与", Weight: 100}}
	got := pickPrize(only)
	if got.ItemID != nil {
		t.Errorf("expected miss row, got itemID=%v", got.ItemID)
	}
}
