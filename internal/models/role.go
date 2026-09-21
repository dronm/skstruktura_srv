package models

type RoleID string

const (
	RoleIDAdmin                   RoleID = "admin"
	RoleIDConstructionSiteManager RoleID = "construction_site_manager"
	RoleIDAccountant              RoleID = "accountant"
	RoleIDSupplier                RoleID = "supplier"
)

func RoleIDValues() []string {
	return []string{
		string(RoleIDAdmin),
		string(RoleIDConstructionSiteManager),
		string(RoleIDAccountant),
		string(RoleIDSupplier),
	}
}

func (v RoleID) IsValid() bool {
	switch v {
	case RoleIDAdmin,
		RoleIDConstructionSiteManager,
		RoleIDAccountant,
		RoleIDSupplier:
		return true
	default:
		return false
	}
}

func (v RoleID) String() string {
	return string(v)
}
