package models

import wmodels "github.com/dronm/webapp/models"

// UserDetail is the complete user aggregate returned by the detail endpoint.
// ConstructionSiteIDs is managed together with the user and has no direct CRUD
// endpoint of its own.
type UserDetail struct {
	ID                  int    `json:"id"`
	Name                string `json:"name"`
	RoleID              RoleID `json:"role_id"`
	ConstructionSiteIDs []int  `json:"construction_site_ids"`
}

// UserList is the safe database-backed user collection model. It intentionally
// excludes both the password hash and aggregate-only fields.
type UserList struct {
	ID     int    `json:"id" primaryKey:"true"`
	Name   string `json:"name" required:"true" maxLen:"100"`
	RoleID RoleID `json:"role_id" enum:"role_id"`
}

func (m UserList) Relation() string {
	return userRelation
}

func (m UserList) CollectionAgg() any {
	return &wmodels.TotCount{TotCount: 0}
}
