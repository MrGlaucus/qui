package models

// ConditionTrees returns action and scoring predicates in a stable order.
// Disabled actions do not participate in continuous observation.
func (a *Automation) ConditionTrees() []*RuleCondition {
	var trees []*RuleCondition
	if c := a.Conditions; c != nil {
		if c.SpeedLimits != nil && c.SpeedLimits.Enabled {
			trees = append(trees, c.SpeedLimits.Condition)
		}
		if c.ShareLimits != nil && c.ShareLimits.Enabled {
			trees = append(trees, c.ShareLimits.Condition)
		}
		if c.Pause != nil && c.Pause.Enabled {
			trees = append(trees, c.Pause.Condition)
		}
		if c.Resume != nil && c.Resume.Enabled {
			trees = append(trees, c.Resume.Condition)
		}
		if c.Recheck != nil && c.Recheck.Enabled {
			trees = append(trees, c.Recheck.Condition)
		}
		if c.Reannounce != nil && c.Reannounce.Enabled {
			trees = append(trees, c.Reannounce.Condition)
		}
		if c.Delete != nil && c.Delete.Enabled {
			trees = append(trees, c.Delete.Condition)
		}
		for _, action := range c.TagActions() {
			if action != nil && action.Enabled {
				trees = append(trees, action.Condition)
			}
		}
		if c.Category != nil && c.Category.Enabled {
			trees = append(trees, c.Category.Condition)
		}
		if c.Move != nil && c.Move.Enabled {
			trees = append(trees, c.Move.Condition)
		}
		if c.ExternalProgram != nil && c.ExternalProgram.Enabled {
			trees = append(trees, c.ExternalProgram.Condition)
		}
		if c.AutoManagement != nil {
			trees = append(trees, c.AutoManagement.Condition)
		}
		if c.ExportToInstance != nil && c.ExportToInstance.Enabled {
			trees = append(trees, c.ExportToInstance.Condition)
		}
	}
	if a.SortingConfig != nil {
		for _, score := range a.SortingConfig.ScoreRules {
			if score.Conditional != nil {
				trees = append(trees, score.Conditional.Condition)
			}
		}
	}
	return trees
}
