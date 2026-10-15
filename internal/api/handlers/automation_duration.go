package handlers

import (
	"errors"

	"github.com/autobrr/qui/internal/models"
)

func validateConditionDuration(condition *models.RuleCondition, depth int) error {
	if condition == nil {
		return nil
	}
	if depth > 20 {
		return errors.New("condition nesting exceeds 20 levels")
	}
	if condition.HoldForSeconds < 0 || condition.HoldForSeconds > 2592000 {
		return errors.New("holdForSeconds must be between 0 and 2592000 seconds (30 days)")
	}
	for _, child := range condition.Conditions {
		if err := validateConditionDuration(child, depth+1); err != nil {
			return err
		}
	}
	return nil
}
