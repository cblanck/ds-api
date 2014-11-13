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
			&rule.Inherited_id,
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

// Get a set of class IDs that are matched by one or more of the given rules
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
	class_id_set, err := GetClassesForRules(db, ds_category.rules)
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
