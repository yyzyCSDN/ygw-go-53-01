package table

import "featurestore/internal/model"

// UserProfileTable returns the built-in user profile feature table used by
// the demo service and the browse page.
func UserProfileTable() *Table {
	return &Table{
		ID:   "user_profile",
		Name: "用户画像特征表",
		Fields: []model.Field{
			{Name: "age", Type: model.FieldInt},
			{Name: "city", Type: model.FieldString},
			{Name: "level", Type: model.FieldInt},
			{Name: "active_days", Type: model.FieldInt},
			{Name: "score", Type: model.FieldFloat},
			{Name: "is_vip", Type: model.FieldBool},
		},
	}
}

// ItemProfileTable returns the built-in item feature table.
func ItemProfileTable() *Table {
	return &Table{
		ID:   "item_profile",
		Name: "物品特征表",
		Fields: []model.Field{
			{Name: "category", Type: model.FieldString},
			{Name: "price", Type: model.FieldFloat},
			{Name: "stock", Type: model.FieldInt},
			{Name: "rating", Type: model.FieldFloat},
			{Name: "on_sale", Type: model.FieldBool},
		},
	}
}

// SeedRegistry registers the built-in tables into a fresh registry.
func SeedRegistry(r *Registry) error {
	if err := r.Register(UserProfileTable()); err != nil {
		return err
	}
	return r.Register(ItemProfileTable())
}
