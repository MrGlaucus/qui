package automations

import (
	"testing"
	"time"

	qbt "github.com/autobrr/go-qbittorrent"
	"github.com/stretchr/testify/require"

	"github.com/autobrr/qui/internal/models"
)

func timedRule(root *RuleCondition) *models.Automation {
	return &models.Automation{ID: 1, Enabled: true, TrackerPattern: "*", Conditions: &models.ActionConditions{
		Pause: &models.PauseAction{Enabled: true, Condition: root},
	}}
}

func TestDurationSortingIsRuleScoped(t *testing.T) {
	t.Parallel()
	condition := &RuleCondition{Field: FieldUpSpeed, Operator: OperatorLessThan, Value: "204800", HoldForSeconds: 30}
	config := &models.SortingConfig{Type: models.SortingTypeScore, ScoreRules: []models.ScoreRule{{
		Type:        models.ScoreRuleTypeConditional,
		Conditional: &models.ConditionalScoreRule{Condition: condition, Score: 100},
	}}}
	// Two rules with identical config must not reuse a score computed from just
	// one rule's timer: the second rule may have only just been enabled.
	a, b := &models.Automation{ID: 1, SortingConfig: config}, &models.Automation{ID: 2, SortingConfig: config}
	require.False(t, rulesCanShareSortingBatch(a, b))
	other := *condition
	other.HoldForSeconds = 60
	require.False(t, conditionEqual(condition, &other))
	condition.HoldForSeconds = 0
	require.True(t, rulesCanShareSortingBatch(a, b))
}

func TestDurationTorrentsHaveIndependentTimers(t *testing.T) {
	t.Parallel()
	cond := &RuleCondition{Field: FieldUpSpeed, Operator: OperatorLessThan, Value: "204800", HoldForSeconds: 30}
	rule := timedRule(cond)
	svc := &Service{cfg: DefaultConfig()}
	start := time.Unix(10000, 0)
	torrents := []qbt.Torrent{{Hash: "a", UpSpeed: 100}, {Hash: "b", UpSpeed: 300000}}
	for _, second := range []int{0, 20, 40} {
		ctx := &EvalContext{}
		sample := start.Add(time.Duration(second) * time.Second)
		svc.observeDurations(1, []*models.Automation{rule}, torrents, ctx, sample, sample, false)
		if second == 40 {
			require.True(t, EvaluateConditionWithContext(cond, torrents[0], ctx, 0))
			require.False(t, EvaluateConditionWithContext(cond, torrents[1], ctx, 0))
		}
		torrents[1].UpSpeed = 100
	}
}

func TestDurationContinuousSamples(t *testing.T) {
	t.Parallel()
	cond := &RuleCondition{Field: FieldUpSpeed, Operator: OperatorLessThan, Value: "204800", HoldForSeconds: 30}
	rule := timedRule(cond)
	svc := &Service{cfg: DefaultConfig()}
	start := time.Unix(10000, 0)
	torrent := qbt.Torrent{Hash: "a", AddedOn: 1, UpSpeed: 102400}
	observe := func(second int, speed int64) bool {
		torrent.UpSpeed = speed
		ctx := &EvalContext{}
		sample := start.Add(time.Duration(second) * time.Second)
		svc.observeDurations(1, []*models.Automation{rule}, []qbt.Torrent{torrent}, ctx, sample, sample, false)
		return EvaluateConditionWithContext(cond, torrent, ctx, 0)
	}
	require.False(t, observe(0, 102400))
	require.False(t, observe(20, 102400))
	require.False(t, observe(20, 102400), "same cached sample cannot mature the timer")
	require.True(t, observe(30, 102400), "threshold is inclusive")
	require.False(t, observe(40, 204800), "equal speed breaks strict less-than")
	require.False(t, observe(50, 102400))
	require.True(t, observe(80, 102400))
	require.False(t, observe(140, 102400), "unobserved gap resets continuity")
	require.False(t, EvaluateConditionWithContext(cond, torrent, nil, 0), "instant evaluation cannot bypass history")
}

func TestDurationObservesShortCircuitedChildren(t *testing.T) {
	t.Parallel()
	for _, op := range []models.ConditionOperator{OperatorAnd, OperatorOr} {
		t.Run(string(op), func(t *testing.T) {
			svc := &Service{cfg: DefaultConfig()}
			timed := &RuleCondition{Field: FieldUpSpeed, Operator: OperatorLessThan, Value: "200", HoldForSeconds: 30}
			first := &RuleCondition{Field: FieldName, Operator: OperatorEqual, Value: "matching"}
			root := &RuleCondition{Operator: op, Conditions: []*RuleCondition{first, timed}}
			rule := timedRule(root)
			torrent := qbt.Torrent{Hash: "a", Name: "different", UpSpeed: 100}
			if op == OperatorOr {
				torrent.Name = "matching"
			}
			for _, second := range []int{0, 20, 40} {
				sample := time.Unix(10000+int64(second), 0)
				ctx := &EvalContext{}
				svc.observeDurations(1, []*models.Automation{rule}, []qbt.Torrent{torrent}, ctx, sample, sample, false)
				require.Equal(t, second == 40, ctx.durationResults[timed]["a"])
			}
		})
	}
}

func TestDurationGroupsNegationAndNestedTimers(t *testing.T) {
	t.Parallel()
	svc := &Service{cfg: DefaultConfig()}
	child := &RuleCondition{Field: FieldUpSpeed, Operator: OperatorGreaterThan, Value: "200", Negate: true, HoldForSeconds: 20}
	root := &RuleCondition{Operator: OperatorAnd, Conditions: []*RuleCondition{child}, HoldForSeconds: 30}
	rule := timedRule(root)
	torrent := qbt.Torrent{Hash: "a", UpSpeed: 100}
	for _, second := range []int{0, 20, 40, 50} {
		sample := time.Unix(10000+int64(second), 0)
		ctx := &EvalContext{}
		svc.observeDurations(1, []*models.Automation{rule}, []qbt.Torrent{torrent}, ctx, sample, sample, false)
		require.Equal(t, second == 50, EvaluateConditionWithContext(root, torrent, ctx, 0))
	}
}

func TestDurationResetAndIsolation(t *testing.T) {
	t.Parallel()
	for _, reset := range []string{"stale", "unknown", "edited", "disabled", "deleted", "torrent removed", "readded", "tracker changed", "restart", "instance", "rule", "dry run"} {
		t.Run(reset, func(t *testing.T) {
			svc := &Service{cfg: DefaultConfig()}
			cond := &RuleCondition{Field: FieldUpSpeed, Operator: OperatorLessThan, Value: "200", HoldForSeconds: 30}
			rule := timedRule(cond)
			torrent := qbt.Torrent{Hash: "a", AddedOn: 1, UpSpeed: 100}
			start := time.Unix(10000, 0)
			for _, second := range []int{0, 20, 40} {
				sample := start.Add(time.Duration(second) * time.Second)
				svc.observeDurations(1, []*models.Automation{rule}, []qbt.Torrent{torrent}, &EvalContext{}, sample, sample, false)
			}
			instance, dry := 1, false
			next := start.Add(60 * time.Second)
			switch reset {
			case "stale":
				svc.observeDurations(1, []*models.Automation{rule}, []qbt.Torrent{torrent}, &EvalContext{}, start, next, false)
			case "unknown":
				svc.observeDurations(1, []*models.Automation{rule}, []qbt.Torrent{torrent}, &EvalContext{}, time.Time{}, next, false)
			case "edited":
				cond.Value = "300"
			case "disabled":
				rule.Enabled = false
				svc.durations.reconcile(1, []*models.Automation{rule})
				rule.Enabled = true
			case "deleted":
				svc.durations.reconcile(1, nil)
			case "torrent removed":
				svc.observeDurations(1, []*models.Automation{rule}, nil, &EvalContext{}, next, next, false)
			case "readded":
				torrent.AddedOn = 2
			case "tracker changed":
				rule.TrackerPattern = "missing.example.test"
				svc.observeDurations(1, []*models.Automation{rule}, []qbt.Torrent{torrent}, &EvalContext{}, next, next, false)
				rule.TrackerPattern = "*"
			case "restart":
				svc = &Service{cfg: DefaultConfig()}
			case "instance":
				instance = 2
			case "rule":
				rule.ID = 2
			case "dry run":
				dry = true
			}
			ctx := &EvalContext{}
			svc.observeDurations(instance, []*models.Automation{rule}, []qbt.Torrent{torrent}, ctx, next, next, dry)
			require.False(t, EvaluateConditionWithContext(cond, torrent, ctx, 0))
		})
	}
}
