package domain

// 聚合行：只读查源表得到，marshal 进 snapshot.data_json。

// SelectedNominationRow 已入选提名行，供 AI② 生成案例文章使用（只读查 stars 表）。
type SelectedNominationRow struct {
	NominationID int64  `json:"nominationId"`
	NomineeName  string `json:"nomineeName"`
	Dimension    string `json:"dimension"`
	DimensionID  int64  `json:"dimensionId"`
	CaseText     string `json:"caseText"`
	CaseRefined  string `json:"caseRefined"`
}

type StarWinnerRow struct {
	UserID    int64  `json:"userId"`
	Name      string `json:"name"`
	AvatarURL string `json:"avatarUrl"`
	Dimension string `json:"dimension"`
	Citation  string `json:"citation"`
}

type ValueRow struct {
	DimensionID     int64  `json:"dimensionId"`
	Name            string `json:"name"`
	Description     string `json:"description"`
	Icon            string `json:"icon"`
	Color           string `json:"color"`
	NominationCount int    `json:"nominationCount"`
}

type HonorRow struct {
	UserID   int64  `json:"userId"`
	Name     string `json:"name"`
	Badge    string `json:"badge"`
	Rarity   string `json:"rarity"`
	IconURL  string `json:"iconUrl"`
	EarnedAt string `json:"earnedAt"`
}

type LotteryRow struct {
	UserID int64  `json:"userId"`
	Name   string `json:"name"`
	Prize  string `json:"prize"`
	WonAt  string `json:"wonAt"`
}

type ActivityRow struct {
	ID      int64  `json:"id"`
	Title   string `json:"title"`
	StartAt string `json:"startAt"`
}

type LeaderRow struct {
	UserID int64  `json:"userId"`
	Name   string `json:"name"`
	Score  int    `json:"score"`
}
