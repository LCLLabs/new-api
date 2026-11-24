package setting

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"sync/atomic"

	"github.com/QuantumNous/new-api/common"
)

const maxDistillPatternLength = 400

// DistillStructure is the contest-harvest gate. All of its conditions must hold.
type DistillStructure struct {
	MinMaxTokens      int    `json:"min_max_tokens"`
	RequireNoTools    bool   `json:"require_no_tools"`
	RequireSingleTurn bool   `json:"require_single_turn"`
	ContestPattern    string `json:"contest_pattern"`
}

// DistillRuleSet is the six editable catalogs. CotPatterns are the distill
// category expressions from vm2api's hard-policy list. There they are matched
// against every user turn. Here they share the latest-user window in
// service.joinPrompt, because new-api has no separate hard-policy pass.
type DistillRuleSet struct {
	HardNeedles  []string         `json:"hard_needles"`
	Patterns     []string         `json:"patterns"`
	Fingerprints []string         `json:"fingerprints"`
	Needles      []string         `json:"needles"`
	CotPatterns  []string         `json:"cot_patterns"`
	Structure    DistillStructure `json:"structure"`
}

// DistillConfig is one validated set of the three distill options. It is not
// changed after it is built, and it is published as a whole, so a request
// never sees the groups of one save with the rules or switch of another.
type DistillConfig struct {
	groups     []string
	groupSet   map[string]struct{}
	rules      DistillRuleSet
	skipOpenAI bool
}

var currentDistill atomic.Pointer[DistillConfig]

func init() {
	currentDistill.Store(newDistillConfig([]string{}, factoryDistillRules(), true))
}

func factoryDistillRules() DistillRuleSet {
	return DistillRuleSet{
		HardNeedles: []string{
			"Memory-stage-one extractor",
			"MUST return strict JSON only",
			"MUST distill reusable",
			"MUST extract durable memory",
			"durable rollout knowledge",
			"You MUST extract durable memory now",
		},
		Patterns: []string{},
		Fingerprints: []string{
			"Let  $a,b,A,B$  be given reals. We consider the function defined by",
			`The graph of the function $y=\cos x - \sin x$ has a line of symmetry given by`,
			`Prove that if $\alpha, \beta, \gamma$ are the angles of a triangle, then`,
			"Jia walks from home to Yi's house. At the same time, Yi rides a bicycle",
			"Colonoscopy reveals a fungating hemorrhagic mass in the ascending colon",
			"reactant: Cc1ccc2c(cnn2C2CCCCO2)c1B1OC(C)(C)C(C)(C)O1",
			"What is the area, in square units, of an isosceles right triangle with a hypotenuse of 20 units?",
		},
		Needles: []string{
			"<answer>",
			"<|begin_of_thought|>",
			"<|end_of_thought|>",
			"<|begin_of_solution|>",
			`Return your final response within \boxed{}`,
			"Generate an executable Python function generated from the given prompt",
			"systematic long thinking process",
			"as an internal monologue",
			`Please reason step by step, and put your final answer within \boxed{}`,
		},
		CotPatterns: []string{
			`\bdistill(?:ation|ing)?\s+(?:of\s+)?(?:(?:the|your|a|an|reusable|durable|rollout|hidden|internal|full|complete)\s+)?(?:reasoning(?:\s+traces?)?|chain[- ]of[- ]thoughts?|teacher(?:\s+model)?)\b`,
			`\b(?:extract|export|dump|reveal|harvest|exfiltrate)(?:ing|ed|ion|s)?\s+(?:(?:the|your|a|an|full|hidden|internal|complete|raw|durable|entire)\s+){0,4}(?:chain[- ]of[- ]thoughts?|reasoning\s+traces?|hidden\s+reasoning|internal\s+reasoning|internal\s+monologues?)\b`,
			`\b(?:chain[- ]of[- ]thought|reasoning\s+trace)\s+extraction\b`,
			`(?:思维链蒸馏|推理蒸馏|思考链蒸馏)`,
			`(?:提取|导出|抽取|蒸馏|收割|扒取)\s*(?:出|取)?\s*(?:你的|本人的|完整|全部|隐藏|内部)?\s*(?:的)?\s*(?:思维链|思考链|推理链)`,
			`(?:思维链|思考链|推理链)\s*(?:的)?\s*(?:提取|导出|抽取|蒸馏|收割)`,
			`(?:提取|导出|抽取)\s*(?:出)?\s*(?:隐藏|内部)\s*(?:的)?\s*(?:推理|思维|思考)`,
		},
		Structure: DistillStructure{
			MinMaxTokens:      8192,
			RequireNoTools:    true,
			RequireSingleTurn: true,
			ContestPattern:    `prove that|\\boxed|\baime\b|olympiad|\bgpqa\b|which of the following|livecodebench|amc[_\s-]?aime`,
		},
	}
}

func cloneRules(rules DistillRuleSet) DistillRuleSet {
	rules.HardNeedles = append([]string(nil), rules.HardNeedles...)
	rules.Patterns = append([]string(nil), rules.Patterns...)
	rules.Fingerprints = append([]string(nil), rules.Fingerprints...)
	rules.Needles = append([]string(nil), rules.Needles...)
	rules.CotPatterns = append([]string(nil), rules.CotPatterns...)
	return rules
}

func newDistillConfig(groups []string, rules DistillRuleSet, skipOpenAI bool) *DistillConfig {
	groupSet := make(map[string]struct{}, len(groups))
	for _, group := range groups {
		groupSet[group] = struct{}{}
	}
	return &DistillConfig{groups: groups, groupSet: groupSet, rules: rules, skipOpenAI: skipOpenAI}
}

// ParseDistillConfig validates the stored values of DistillCheckGroups,
// DistillRules and DistillSkipOpenAI together.
func ParseDistillConfig(groupsValue, rulesValue, skipOpenAIValue string) (*DistillConfig, error) {
	groups, err := parseDistillGroups(groupsValue)
	if err != nil {
		return nil, err
	}
	rules, err := parseDistillRules(rulesValue)
	if err != nil {
		return nil, err
	}
	skipOpenAI, err := strconv.ParseBool(skipOpenAIValue)
	if err != nil {
		return nil, fmt.Errorf("invalid boolean: DistillSkipOpenAI")
	}
	return newDistillConfig(groups, rules, skipOpenAI), nil
}

// PublishDistillConfig replaces the distill settings every later request reads.
func PublishDistillConfig(config *DistillConfig) {
	if config != nil {
		currentDistill.Store(config)
	}
}

// CurrentDistillConfig returns the published settings. Read it once per request.
func CurrentDistillConfig() *DistillConfig {
	return currentDistill.Load()
}

// Checks reports whether this user group is on the check list.
// An empty group, including channel tests, is never on the list.
func (c *DistillConfig) Checks(group string) bool {
	_, ok := c.groupSet[strings.TrimSpace(group)]
	return ok
}

// SkipOpenAI reports whether gpt-* models skip the distill check.
func (c *DistillConfig) SkipOpenAI() bool {
	return c.skipOpenAI
}

// Rules returns a copy the caller may keep or change.
func (c *DistillConfig) Rules() DistillRuleSet {
	return cloneRules(c.rules)
}

func DistillSkipOpenAI() bool {
	return CurrentDistillConfig().skipOpenAI
}

func DistillCheckGroupsJSON() string {
	encoded, err := common.Marshal(CurrentDistillConfig().groups)
	if err != nil {
		return "[]"
	}
	return string(encoded)
}

func DistillRulesJSON() string {
	encoded, err := common.Marshal(CurrentDistillConfig().rules)
	if err != nil {
		return "{}"
	}
	return string(encoded)
}

func parseDistillGroups(value string) ([]string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return []string{}, nil
	}
	var groups []string
	if err := common.UnmarshalJsonStr(value, &groups); err != nil {
		return nil, fmt.Errorf("distill groups must be a JSON array of strings")
	}
	cleaned := make([]string, 0, len(groups))
	seen := make(map[string]struct{}, len(groups))
	for _, group := range groups {
		group = strings.TrimSpace(group)
		if group == "" {
			continue
		}
		if _, ok := seen[group]; ok {
			continue
		}
		seen[group] = struct{}{}
		cleaned = append(cleaned, group)
	}
	return cleaned, nil
}

func cleanPhrases(items []string) []string {
	cleaned := make([]string, 0, len(items))
	for _, item := range items {
		item = strings.TrimSpace(item)
		if item != "" {
			cleaned = append(cleaned, item)
		}
	}
	return cleaned
}

func validatePattern(source, label string) error {
	source = strings.TrimSpace(source)
	if source == "" {
		return nil
	}
	if len(source) > maxDistillPatternLength {
		return fmt.Errorf("%s longer than %d characters", label, maxDistillPatternLength)
	}
	if _, err := regexp.Compile("(?i)" + source); err != nil {
		return fmt.Errorf("%s: %w", label, err)
	}
	return nil
}

func parseDistillRules(value string) (DistillRuleSet, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return DistillRuleSet{}, fmt.Errorf("distill rules must be a JSON object")
	}
	var rules DistillRuleSet
	if err := common.UnmarshalJsonStr(value, &rules); err != nil {
		return DistillRuleSet{}, fmt.Errorf("distill rules must be a JSON object")
	}
	rules.HardNeedles = cleanPhrases(rules.HardNeedles)
	rules.Patterns = cleanPhrases(rules.Patterns)
	rules.Fingerprints = cleanPhrases(rules.Fingerprints)
	rules.Needles = cleanPhrases(rules.Needles)
	rules.CotPatterns = cleanPhrases(rules.CotPatterns)
	if rules.Structure.MinMaxTokens < 0 {
		return DistillRuleSet{}, fmt.Errorf("min_max_tokens must be zero or positive")
	}
	for i, source := range rules.Patterns {
		if err := validatePattern(source, fmt.Sprintf("pattern %d", i+1)); err != nil {
			return DistillRuleSet{}, err
		}
	}
	for i, source := range rules.CotPatterns {
		if err := validatePattern(source, fmt.Sprintf("chain-of-thought pattern %d", i+1)); err != nil {
			return DistillRuleSet{}, err
		}
	}
	if err := validatePattern(rules.Structure.ContestPattern, "contest pattern"); err != nil {
		return DistillRuleSet{}, err
	}
	return rules, nil
}
