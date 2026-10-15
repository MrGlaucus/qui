package automations

import (
	"encoding/json"
	"fmt"
	"strconv"
	"sync"
	"time"

	qbt "github.com/autobrr/go-qbittorrent"

	"github.com/autobrr/qui/internal/models"
)

type durationRuleKey struct {
	instance, rule int
	dryRun         bool
}
type durationKey struct {
	node, hash string
	addedOn    int64
}
type durationRun struct{ since, last time.Time }
type durationRuleState struct {
	revision string
	runs     map[durationKey]durationRun
}
type durationTracker struct {
	mu    sync.Mutex
	rules map[durationRuleKey]*durationRuleState
}

func (d *durationTracker) prune(cutoff time.Time) {
	d.mu.Lock()
	defer d.mu.Unlock()
	for key, state := range d.rules {
		for entry, run := range state.runs {
			if run.last.Before(cutoff) {
				delete(state.runs, entry)
			}
		}
		if len(state.runs) == 0 {
			delete(d.rules, key)
		}
	}
}

func (d *durationTracker) clearInstance(instance int) {
	d.mu.Lock()
	defer d.mu.Unlock()
	for key := range d.rules {
		if key.instance == instance {
			delete(d.rules, key)
		}
	}
}

func (d *durationTracker) clearRules(instance int, rules []*models.Automation, dryRun bool) {
	d.mu.Lock()
	defer d.mu.Unlock()
	for _, rule := range rules {
		delete(d.rules, durationRuleKey{instance, rule.ID, dryRun})
	}
}

func (d *durationTracker) reconcile(instance int, rules []*models.Automation) {
	d.mu.Lock()
	defer d.mu.Unlock()
	active := make(map[int]bool, len(rules))
	for _, rule := range rules {
		active[rule.ID] = rule.Enabled && ruleHasDuration(rule)
	}
	for key := range d.rules {
		if key.instance == instance && !active[key.rule] {
			delete(d.rules, key)
		}
	}
}

func durationNodes(rule *models.Automation) map[*RuleCondition]string {
	nodes := make(map[*RuleCondition]string)
	var walk func(*RuleCondition, string)
	walk = func(c *RuleCondition, path string) {
		if c == nil {
			return
		}
		if c.HoldForSeconds > 0 {
			nodes[c] = path
		}
		for i, child := range c.Conditions {
			walk(child, fmt.Sprintf("%s/%d", path, i))
		}
	}
	for i, root := range rule.ConditionTrees() {
		walk(root, strconv.Itoa(i))
	}
	return nodes
}

func ruleHasDuration(rule *models.Automation) bool { return len(durationNodes(rule)) > 0 }

// observe records only successful, fresh samples. Repeated reads of the same
// sample cannot advance a timer. Long gaps discard continuity rather than
// counting unobserved time. A condition becoming false discards its start time.
func (s *durationRuleState) observe(key durationKey, matches bool, sample time.Time, gap time.Duration, seconds int) bool {
	if !matches {
		delete(s.runs, key)
		return false
	}
	run, exists := s.runs[key]
	if !exists || sample.Before(run.last) || sample.Sub(run.last) > gap {
		run = durationRun{since: sample}
	}
	run.last = sample
	s.runs[key] = run
	return sample.Sub(run.since) >= time.Duration(seconds)*time.Second
}

func (s *Service) observeDurations(instance int, rules []*models.Automation, torrents []qbt.Torrent, eval *EvalContext, sample, now time.Time, dryRun bool) {
	s.durations.mu.Lock()
	defer s.durations.mu.Unlock()
	if s.durations.rules == nil {
		s.durations.rules = make(map[durationRuleKey]*durationRuleState)
	}
	eval.durationResults = make(map[*RuleCondition]map[string]bool)
	defer func() { eval.observeDuration = nil }()
	gap := 2 * s.cfg.ScanInterval
	if gap <= 0 {
		gap = 2 * DefaultConfig().ScanInterval
	}
	for _, rule := range rules {
		nodes := durationNodes(rule)
		if len(nodes) == 0 {
			continue
		}
		key := durationRuleKey{instance, rule.ID, dryRun}
		if sample.IsZero() || now.Sub(sample) > gap || sample.After(now) {
			delete(s.durations.rules, key)
			continue
		}
		encoded, err := json.Marshal(rule)
		if err != nil {
			delete(s.durations.rules, key)
			continue
		}
		revision := string(encoded)
		state := s.durations.rules[key]
		if state == nil || state.revision != revision {
			state = &durationRuleState{revision: revision, runs: make(map[durationKey]durationRun)}
			s.durations.rules[key] = state
		}
		seen := make(map[durationKey]bool)
		eval.observeDuration = func(cond *RuleCondition, torrent qbt.Torrent, matches bool) bool {
			entry := durationKey{nodes[cond], torrent.Hash, torrent.AddedOn}
			seen[entry] = true
			ready := state.observe(entry, matches, sample, gap, cond.HoldForSeconds)
			if eval.durationResults[cond] == nil {
				eval.durationResults[cond] = make(map[string]bool)
			}
			eval.durationResults[cond][torrent.Hash] = ready
			return ready
		}
		activateRuleGrouping(eval, rule, torrents, s.syncManager)
		if ruleUsesCondition(rule, FieldFreeSpace) {
			eval.LoadFreeSpaceSourceState(GetFreeSpaceRuleKey(rule))
		}
		roots := rule.ConditionTrees()
		for _, torrent := range torrents {
			if len(selectMatchingRules(torrent, []*models.Automation{rule}, s.syncManager)) == 0 {
				continue
			}
			for _, root := range roots {
				EvaluateConditionWithContext(root, torrent, eval, 0)
			}
		}
		for entry := range state.runs {
			if !seen[entry] {
				delete(state.runs, entry)
			}
		}
	}
}
