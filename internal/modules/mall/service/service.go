package service

import (
	"context"
	"errors"
	"math/rand"
	"strings"
	"time"

	"github.com/standardsoftware/culture_points_mall/internal/modules/mall/domain"
	"github.com/standardsoftware/culture_points_mall/internal/modules/mall/repository"
	pointssvc "github.com/standardsoftware/culture_points_mall/internal/modules/points/service"
	valuessvc "github.com/standardsoftware/culture_points_mall/internal/modules/values/service"
)

type Service struct {
	Repo      *repository.GormRepo
	Points    *pointssvc.Service
	Values    *valuessvc.Service
	FreezeTTL time.Duration
}

func New(r *repository.GormRepo, p *pointssvc.Service, v *valuessvc.Service) *Service {
	return &Service{Repo: r, Points: p, Values: v, FreezeTTL: 30 * time.Second}
}

type DrawResult struct {
	Win        bool   `json:"win"`
	PrizeID    int64  `json:"prizeId,omitempty"`
	PrizeName  string `json:"prizeName"`
	PrizeImage string `json:"prizeImage,omitempty"`
	Amount     int    `json:"amount"`
}

var (
	ErrItemNotBlindbox   = errors.New("item not blindbox")
	ErrNoPrizes          = errors.New("no prizes configured")
	ErrInvalidItemType   = errors.New("type must be 'item' or 'blindbox'")
	ErrInvalidItemName   = errors.New("name required")
	ErrInvalidItemCost   = errors.New("cost must be > 0")
	ErrItemNotRedeemable = errors.New("盲盒请用抽奖，不能直接兑换")
	ErrOutOfStock        = errors.New("库存不足")
)

type CreateItemCmd struct {
	TenantID int64
	Type     string
	Name     string
	Cost     int
	Stock    *int
	ImageURL string
}

// ListItems 列出商城商品；typ 为空字符串则列全部，否则按 type 过滤（item / blindbox）
func (s *Service) ListItems(ctx context.Context, tenantID int64, typ string) ([]domain.Item, error) {
	return s.Repo.ListItems(ctx, tenantID, typ)
}

// CreateItem 新增积分商城商品（item 或 blindbox）
func (s *Service) CreateItem(ctx context.Context, cmd CreateItemCmd) (*domain.Item, error) {
	if cmd.Type != "item" && cmd.Type != "blindbox" {
		return nil, ErrInvalidItemType
	}
	if strings.TrimSpace(cmd.Name) == "" {
		return nil, ErrInvalidItemName
	}
	if cmd.Cost <= 0 {
		return nil, ErrInvalidItemCost
	}
	it := &domain.Item{
		TenantID: cmd.TenantID,
		Type:     cmd.Type,
		Name:     strings.TrimSpace(cmd.Name),
		Cost:     cmd.Cost,
		Stock:    cmd.Stock,
		ImageURL: cmd.ImageURL,
	}
	if err := s.Repo.CreateItem(ctx, it); err != nil {
		return nil, err
	}
	return it, nil
}

// Draw 抽奖完整 TCC 链路
func (s *Service) Draw(ctx context.Context, tenantID, userID, boxID int64) (*DrawResult, error) {
	box, err := s.Repo.GetItem(ctx, tenantID, boxID)
	if err != nil {
		return nil, err
	}
	if box.Type != "blindbox" {
		return nil, ErrItemNotBlindbox
	}

	// Try
	txID, err := s.Points.TryFreeze(ctx, tenantID, userID, box.Cost, s.FreezeTTL)
	if err != nil {
		return nil, err
	}

	// 抽奖
	prizes, err := s.Repo.ListPrizes(ctx, boxID)
	if err != nil {
		_ = s.Points.CancelByTxID(ctx, txID)
		return nil, err
	}
	if len(prizes) == 0 {
		_ = s.Points.CancelByTxID(ctx, txID)
		return nil, ErrNoPrizes
	}

	// 记 freeze 行
	freeze := &domain.Freeze{
		TxID: txID, UserID: userID, BoxItemID: boxID, Amount: box.Cost,
		Status: domain.FreezeTry, ExpiresAt: time.Now().Add(s.FreezeTTL),
	}
	if err := s.Repo.CreateFreeze(ctx, freeze); err != nil {
		_ = s.Points.CancelByTxID(ctx, txID)
		return nil, err
	}

	prize := weightedPick(prizes)

	// 判断「未中奖」
	if isMissPrize(prize.PrizeName) {
		_ = s.Points.CancelByTxID(ctx, txID)
		_ = s.Repo.MarkCancelled(ctx, txID)
		return &DrawResult{Win: false, PrizeName: prize.PrizeName, Amount: box.Cost}, nil
	}

	// Confirm：扣分需绑定一个维度。盲盒消费没有专属维度，
	// 这里取租户的第一个维度（按 sort_order）作为「消费扣减」归属。
	dimID := s.resolveConsumptionDim(ctx, tenantID)

	if err := s.Points.Confirm(ctx, tenantID, userID, box.Cost, dimID, "盲盒抽奖 · "+box.Name); err != nil {
		_ = s.Points.CancelByTxID(ctx, txID)
		_ = s.Repo.MarkCancelled(ctx, txID)
		return nil, err
	}
	_ = s.Repo.MarkConfirmed(ctx, txID)
	prizeID := prize.ID
	_ = s.Repo.CreateOrder(ctx, &domain.Order{
		TenantID: tenantID, UserID: userID, ItemID: &boxID, PrizeID: &prizeID, Cost: box.Cost, Status: "paid",
	})

	return &DrawResult{
		Win: true, PrizeID: prize.ID, PrizeName: prize.PrizeName,
		PrizeImage: prize.PrizeImage, Amount: box.Cost,
	}, nil
}

type RedeemResult struct {
	ItemName string `json:"itemName"`
	Cost     int    `json:"cost"`
}

// Redeem 直接兑换非盲盒商品：校验库存 → 冻结积分 → 确认扣分 → 建订单 → 减库存
func (s *Service) Redeem(ctx context.Context, tenantID, userID, itemID int64) (*RedeemResult, error) {
	item, err := s.Repo.GetItem(ctx, tenantID, itemID)
	if err != nil {
		return nil, err
	}
	if item.Type != "item" {
		return nil, ErrItemNotRedeemable
	}
	if item.Stock != nil && *item.Stock <= 0 {
		return nil, ErrOutOfStock
	}

	txID, err := s.Points.TryFreeze(ctx, tenantID, userID, item.Cost, s.FreezeTTL)
	if err != nil {
		return nil, err
	}
	dimID := s.resolveConsumptionDim(ctx, tenantID)
	if err := s.Points.Confirm(ctx, tenantID, userID, item.Cost, dimID, "积分兑换 · "+item.Name); err != nil {
		_ = s.Points.CancelByTxID(ctx, txID)
		return nil, err
	}
	iid := itemID
	_ = s.Repo.CreateOrder(ctx, &domain.Order{TenantID: tenantID, UserID: userID, ItemID: &iid, Cost: item.Cost, Status: "paid"})
	_ = s.Repo.DecrementStock(ctx, itemID)
	return &RedeemResult{ItemName: item.Name, Cost: item.Cost}, nil
}

func (s *Service) resolveConsumptionDim(ctx context.Context, tenantID int64) int64 {
	dims, err := s.Values.GetDimensions(ctx, tenantID)
	if err != nil || len(dims) == 0 {
		return 1 // 兜底（不会真用到，正常情况下租户都有 6 维度）
	}
	return dims[0].ID
}

func weightedPick(prizes []domain.BlindboxPrize) domain.BlindboxPrize {
	total := 0
	for _, p := range prizes {
		total += p.Weight
	}
	if total <= 0 {
		return prizes[0]
	}
	x := rand.Intn(total)
	cum := 0
	for _, p := range prizes {
		cum += p.Weight
		if x < cum {
			return p
		}
	}
	return prizes[len(prizes)-1]
}

func isMissPrize(name string) bool {
	if name == "" {
		return true
	}
	for _, kw := range []string{"未中奖", "鼓励", "差一点"} {
		if strings.Contains(name, kw) {
			return true
		}
	}
	return false
}
