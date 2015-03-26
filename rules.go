package main

import (
	"database/sql"
	"errors"
	"fmt"
	"github.com/deckarep/golang-set"
	_ "github.com/go-sql-driver/mysql"
)

const RULE_CLASS = 1
const RULE_CATEGORY = 2

// Get a slice of DSCategoryRule objects associated with a given rule category
func GetRulesForCategory(db *sql.DB, id int64) ([]*DSCategoryRule, error) {
	rows, err := db.Query(`SELECT id, category, ruletype, class_id, category_id,
    inherited_id, passfail_allowed FROM ds_category_rule WHERE category = ?`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	rules := make([]*DSCategoryRule, 0)

	for rows.Next() {
		rule := new(DSCategoryRule)
		if err := rows.Scan(
			&rule.Id,
			&rule.Category,
			&rule.Ruletype,
			&rule.Class_id,
			&rule.Category_id,
			&rule.Inherit_id,
			&rule.Passfail_allowed); err != nil {
			return nil, err
		}
		rules = append(rules, rule)
	}
	return rules, nil
}

// Get a set of class IDs that are matched by a given rule struct
func GetClassesForRule(db *sql.DB, rule *DSCategoryRule) (mapset.Set, error) {
	switch rule.Ruletype {
	case RULE_CLASS:
		{
			classes := mapset.NewSet()
			if !rule.Class_id.Valid {
				return nil, errors.New(fmt.Sprintf("Rule %d has malformed structure", rule.Id))
			}
			classes.Add(rule.Class_id.Int64)
			return classes, nil
		}

	case RULE_CATEGORY:
		{
			if !rule.Category_id.Valid {
				return nil, errors.New(fmt.Sprintf("Rule %d has malformed structure", rule.Id))
			}
			rules, err := GetRulesForCategory(db, rule.Category_id.Int64)
			if err != nil {
				return nil, err
			}
			return GetClassesForRules(db, rules)
		}
	}
	return mapset.NewSet(), nil
}

// Get a set of class IDs that are matched by and of the rules
func GetClassesForRules(db *sql.DB, rules []*DSCategoryRule) (mapset.Set, error) {
	classes := mapset.NewSet()
	for _, rule := range rules {
		rule_classes, err := GetClassesForRule(db, rule)
		if err != nil {
			return nil, err
		}
		classes = classes.Union(rule_classes)
	}
	return classes, nil
}

// Get classes matched by a rule by Id
func GetClassesForRuleById(db *sql.DB, id int64) (map[int64]*Class, error) {
	ds_category, err := GetDSCategoryById(db, id)
	if err != nil {
		return nil, err
	}
	// Iterate over the rules for the category, and union the classes
	class_id_set, err := GetClassesForRules(db, ds_category.Rules)
	if err != nil {
		return nil, err
	}
	class_map := make(map[int64]*Class)
	for class_id := range class_id_set.Iter() {
		class_map[class_id.(int64)], err = GetClassById(db, class_id.(int64))
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
func GetCategoriesMatchedbyClass(db *sql.DB, class_id int64) ([]*DSCategory, error) {
	all_categories, err := GetCategories(db)
	if err != nil {
		return nil, err
	}
	matching_categories := make([]*DSCategory, 0)

	for _, category := range all_categories {
		classes_matched, err := GetClassesForRuleById(db, category.Id)
		if err != nil {
			return nil, err
		}
		if _, exists := classes_matched[class_id]; exists {
			matching_categories = append(matching_categories, category)
		}
	}

	return matching_categories, nil

}
