package models

type RoleID string

const (
	RoleIDAdmin                   RoleID = "admin"
	RoleIDConstructionSiteManager RoleID = "construction_site_manager"
	RoleIDAccountant              RoleID = "accountant"
	RoleIDSupplyManager           RoleID = "supply_manager"
)

func RoleIDValues() []string {
	return []string{
		string(RoleIDAdmin),
		string(RoleIDConstructionSiteManager),
		string(RoleIDAccountant),
		string(RoleIDSupplyManager),
	}
}

func (v RoleID) IsValid() bool {
	switch v {
	case RoleIDAdmin,
		RoleIDConstructionSiteManager,
		RoleIDAccountant,
		RoleIDSupplyManager:
		return true
	default:
		return false
	}
}

func (v RoleID) String() string {
	return string(v)
}
