package main

import (
	"database/sql"
	_ "github.com/go-sql-driver/mysql"
)

const RULE_CLASS = 1
const RULE_CATEGORY = 2
const RULE_INHERIT = 4

// Get classes matched by a rule by Id
func GetClassesForCategoryById(db *sql.DB, id int64) (map[int64]*Class, error) {
	class_category, err := GetClassCategoryById(db, id)
	if err != nil {
		return nil, err
	}
	class_map := make(map[int64]*Class)
	for _, class := range class_category.Classes {
		class_map[class.Id] = class
		if err != nil {
			return nil, err
		}
	}
	return class_map, nil
}

// Get list of categories
func GetCategories(db *sql.DB) ([]*DSCategory, error) {
	rows, err := db.Query("SELECT id, name from ds_category")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	categories := make([]*DSCategory, 0)
	for rows.Next() {
		category := new(DSCategory)
		if err := rows.Scan(
			&category.Id,
			&category.Name); err != nil {
			return nil, err
		}
		categories = append(categories, category)
	}
	return categories, nil
}

// Get all categories that a class can be counted towards
func GetCategoriesMatchedbyClass(db *sql.DB, class_id int64) ([]*ClassCategory, error) {
	rows, err := db.Query(`SELECT DISTINCT(category)
	FROM class_category_rule WHERE class_id = ?`, class_id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var category_id int64
	category_list := make([]*ClassCategory, 0)
	for rows.Next() {
		if err := rows.Scan(
			&category_id,
		); err != nil {
			return nil, err
		}
		category, err := GetClassCategoryById(db, category_id)
		if err != nil {
			return nil, err
		}
		category_list = append(category_list, category)
	}

	return category_list, nil
}
