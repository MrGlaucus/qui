package handlers

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/autobrr/qui/internal/models"
)

func TestValidateConditionDuration(t *testing.T) {
	t.Parallel()
	for _, seconds := range []int{-1, 0, 30, 2592000, 2592001} {
		condition := &models.RuleCondition{Operator: models.OperatorAnd, Conditions: []*models.RuleCondition{{HoldForSeconds: seconds}}}
		err := validateConditionDuration(condition, 0)
		if seconds < 0 || seconds > 2592000 {
			require.Error(t, err)
		} else {
			require.NoError(t, err)
		}
	}
}
