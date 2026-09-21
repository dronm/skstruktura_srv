package models

type MaterialBalances []struct {
	ConstructionSite Ref     `json:"construction_site"`
	Balance          float64 `json:"balance"`
}
