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
	ErrItemOffShelf      = errors.New("商品已下架")
)

type CreateItemCmd struct {
	TenantID int64
	Type     string
	Name     string
	Cost     int
	Stock    *int
	ImageURL string
}

// ListItems 列出商城商品（含已下架，供后台/HR-Agent 管理用）；typ 为空则列全部，否则按 type 过滤。
func (s *Service) ListItems(ctx context.Context, tenantID int64, typ string) ([]domain.Item, error) {
	return s.Repo.ListItems(ctx, tenantID, typ, false)
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
		Status:   domain.StatusOnShelf,
	}
	if err := s.Repo.CreateItem(ctx, it); err != nil {
		return nil, err
	}
	return it, nil
}

type UpdateItemCmd struct {
	TenantID int64
	ItemID   int64
	Name     *string
	Cost     *int
	Stock    *int
	StockSet bool // true 时才改库存：Stock=nil 表示改为不限量
	ImageURL *string
}

// UpdateItem 局部更新商品，返回更新后的 item 与更新前的 item（后者供「回撤」精确还原）。
func (s *Service) UpdateItem(ctx context.Context, cmd UpdateItemCmd) (newIt *domain.Item, oldIt *domain.Item, err error) {
	old, err := s.Repo.GetItem(ctx, cmd.TenantID, cmd.ItemID)
	if err != nil {
		return nil, nil, err
	}
	fields := map[string]any{}
	if cmd.Name != nil && strings.TrimSpace(*cmd.Name) != "" {
		fields["name"] = strings.TrimSpace(*cmd.Name)
	}
	if cmd.Cost != nil {
		if *cmd.Cost <= 0 {
			return nil, nil, ErrInvalidItemCost
		}
		fields["cost"] = *cmd.Cost
	}
	if cmd.StockSet {
		if cmd.Stock != nil {
			fields["stock"] = *cmd.Stock
		} else {
			fields["stock"] = nil // 不限量
		}
	}
	if cmd.ImageURL != nil {
		fields["image_url"] = *cmd.ImageURL
	}
	if len(fields) == 0 {
		return old, old, nil
	}
	if err := s.Repo.UpdateItemFields(ctx, cmd.TenantID, cmd.ItemID, fields); err != nil {
		return nil, nil, err
	}
	newIt, err = s.Repo.GetItem(ctx, cmd.TenantID, cmd.ItemID)
	if err != nil {
		return nil, nil, err
	}
	return newIt, old, nil
}

// SetItemStatus 上架/下架商品，返回改之前的状态（供「回撤」还原）。
func (s *Service) SetItemStatus(ctx context.Context, tenantID, id int64, status string) (string, error) {
	old, err := s.Repo.GetItem(ctx, tenantID, id)
	if err != nil {
		return "", err
	}
	if err := s.Repo.UpdateItemFields(ctx, tenantID, id, map[string]any{"status": status}); err != nil {
		return "", err
	}
	return old.Status, nil
}

// DeleteItem 删除商品（用于「撤销新增商品」回撤）。
func (s *Service) DeleteItem(ctx context.Context, tenantID, id int64) error {
	return s.Repo.DeleteItem(ctx, tenantID, id)
}

// BoxPrizeCmd 盲盒里一件「好物奖品」的配置。
type BoxPrizeCmd struct {
	ItemID *int64
	Weight int
	Stock  *int // nil = 不限份数
}

// SaveBoxConfigCmd 整存盲盒配置：扣分模式 + 无奖品权重 + 勾选的好物奖品。
type SaveBoxConfigCmd struct {
	ChargeOnMiss  bool
	NoPrizeWeight int
	Prizes        []BoxPrizeCmd
}

// SaveBoxConfig 校验并整存盲盒奖池配置（清空重建）。好物名称/图片做快照便于订单与历史展示。
func (s *Service) SaveBoxConfig(ctx context.Context, tenantID, boxID int64, cmd SaveBoxConfigCmd) error {
	box, err := s.Repo.GetItem(ctx, tenantID, boxID)
	if err != nil {
		return err
	}
	if box.Type != "blindbox" {
		return ErrItemNotBlindbox
	}
	inputs := make([]repository.PrizeInput, 0, len(cmd.Prizes)+1)
	for _, p := range cmd.Prizes {
		if p.ItemID == nil {
			continue
		}
		good, err := s.Repo.GetItem(ctx, tenantID, *p.ItemID)
		if err != nil {
			return err
		}
		if good.Type != "item" {
			return ErrInvalidItemType
		}
		w := p.Weight
		if w < 0 {
			w = 0
		}
		inputs = append(inputs, repository.PrizeInput{
			ItemID:     p.ItemID,
			PrizeName:  good.Name,
			PrizeImage: good.ImageURL,
			Weight:     w,
			Stock:      p.Stock,
		})
	}
	npw := cmd.NoPrizeWeight
	if npw < 0 {
		npw = 0
	}
	inputs = append(inputs, repository.PrizeInput{ItemID: nil, PrizeName: "谢谢参与", Weight: npw})
	return s.Repo.ReplaceBoxConfig(ctx, boxID, cmd.ChargeOnMiss, inputs)
}

// Draw 抽奖完整 TCC 链路。
// 奖池 = 配置进盲盒的「积分好物」+ 一行「无奖品」；中奖 = 抽到的奖项有 item_id。
// 每次抽奖消耗 box.Cost 分：中奖必扣；未中奖按 box.ChargeOnMiss（后台可配）决定扣/退。
func (s *Service) Draw(ctx context.Context, tenantID, userID, boxID int64) (*DrawResult, error) {
	box, err := s.Repo.GetItem(ctx, tenantID, boxID)
	if err != nil {
		return nil, err
	}
	if box.Type != "blindbox" {
		return nil, ErrItemNotBlindbox
	}

	// Try：冻结积分
	txID, err := s.Points.TryFreeze(ctx, tenantID, userID, box.Cost, s.FreezeTTL)
	if err != nil {
		return nil, err
	}

	// 抽奖：仅在「还有份数」的奖项里加权抽取（份数 NULL = 不限量）
	prizes, err := s.Repo.ListPrizeViews(ctx, boxID)
	if err != nil {
		_ = s.Points.CancelByTxID(ctx, txID)
		return nil, err
	}
	candidates := drawablePrizes(prizes)
	if len(candidates) == 0 {
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

	prize := pickPrize(candidates)
	win := prize.ItemID != nil

	// Confirm：扣分需绑定一个维度。盲盒消费没有专属维度，
	// 这里取租户的第一个维度（按 sort_order）作为「消费扣减」归属。
	confirm := func(reason string) error {
		dimID := s.resolveConsumptionDim(ctx, tenantID)
		if err := s.Points.Confirm(ctx, tenantID, userID, box.Cost, dimID, reason); err != nil {
			_ = s.Points.CancelByTxID(ctx, txID)
			_ = s.Repo.MarkCancelled(ctx, txID)
			return err
		}
		_ = s.Repo.MarkConfirmed(ctx, txID)
		return nil
	}

	if !win {
		if box.ChargeOnMiss {
			if err := confirm("盲盒抽奖 · " + box.Name + "（未中奖）"); err != nil {
				return nil, err
			}
			return &DrawResult{Win: false, PrizeName: prize.PrizeName, Amount: box.Cost}, nil
		}
		_ = s.Points.CancelByTxID(ctx, txID)
		_ = s.Repo.MarkCancelled(ctx, txID)
		return &DrawResult{Win: false, PrizeName: prize.PrizeName, Amount: 0}, nil
	}

	if err := confirm("盲盒抽奖 · " + box.Name); err != nil {
		return nil, err
	}
	prizeID := prize.ID
	_ = s.Repo.CreateOrder(ctx, &domain.Order{
		TenantID: tenantID, UserID: userID, ItemID: prize.ItemID, PrizeID: &prizeID, Cost: box.Cost, Status: "paid",
	})
	_ = s.Repo.DecrementPrizeStock(ctx, prize.ID) // 中奖好物份数 -1（NULL 不限量则不变）

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
	if item.Status == domain.StatusOffShelf {
		return nil, ErrItemOffShelf
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

// drawablePrizes 过滤出仍有份数的奖项（份数 NULL = 不限量）。
func drawablePrizes(prizes []repository.PrizeView) []repository.PrizeView {
	out := make([]repository.PrizeView, 0, len(prizes))
	for _, p := range prizes {
		if p.Stock == nil || *p.Stock > 0 {
			out = append(out, p)
		}
	}
	return out
}

// pickPrize 在候选奖项里按 weight 加权随机抽取一项。
func pickPrize(prizes []repository.PrizeView) repository.PrizeView {
	weights := make([]int, len(prizes))
	total := 0
	for i, p := range prizes {
		w := p.Weight
		if w < 0 {
			w = 0
		}
		weights[i] = w
		total += w
	}
	if total <= 0 {
		return prizes[0]
	}
	return prizes[pickIndexByWeight(weights, rand.Intn(total))]
}

// pickIndexByWeight 返回累积权重首次覆盖 x 的下标（x ∈ [0,total)）。纯函数，便于单测。
func pickIndexByWeight(weights []int, x int) int {
	cum := 0
	for i, w := range weights {
		cum += w
		if x < cum {
			return i
		}
	}
	return len(weights) - 1
}
