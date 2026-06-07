package domain

type Item struct {
	ID       int64  `gorm:"primaryKey"`
	TenantID int64  `gorm:"column:tenant_id"`
	Type     string `gorm:"column:type"`
	Name     string `gorm:"column:name"`
	Cost     int    `gorm:"column:cost"`
	Stock    *int   `gorm:"column:stock"`
	ImageURL string `gorm:"column:image_url"`
	Status   string `gorm:"column:status"` // on_shelf=在售 / off_shelf=已下架
	// ChargeOnMiss 仅 blindbox 使用：抽到「无奖品」时是否也扣分（true=都扣，false=中奖才扣）。
	ChargeOnMiss bool `gorm:"column:charge_on_miss"`
}

func (Item) TableName() string { return "mall_items" }

const (
	StatusOnShelf  = "on_shelf"
	StatusOffShelf = "off_shelf"
)

type BlindboxPrize struct {
	ID        int64 `gorm:"primaryKey"`
	BoxItemID int64 `gorm:"column:box_item_id"`
	// ItemID 指向 mall_items 中的「积分好物」；NULL 表示「无奖品（谢谢参与）」行。
	ItemID     *int64 `gorm:"column:item_id"`
	PrizeName  string `gorm:"column:prize_name"`
	PrizeImage string `gorm:"column:prize_image"`
	Weight     int    `gorm:"column:weight"`
	Stock      *int   `gorm:"column:stock"`
}

func (BlindboxPrize) TableName() string { return "mall_blindbox_pool" }
