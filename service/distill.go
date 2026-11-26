package service

import (
	"encoding/json"
	"regexp"
	"strings"
	"unicode"

	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/setting"
)

const (
	distillRuleHarvestNeedle = "hard_needles"
	distillRuleRegex         = "patterns"
	distillRuleFingerprint   = "fingerprints"
	distillRuleNeedle        = "needles"
	distillRuleHarvest       = "structure"
	distillRuleCot           = "chain-of-thought extraction"
)

var (
	distillWhitespace   = regexp.MustCompile(`\s+`)
	distillBackslashGap = regexp.MustCompile(`\\\s+`)
	distillNegationTail = regexp.MustCompile(`(?i)(?:\b(?:do\s+not|don't|never|must\s+not|should\s+not)\s+)$`)
)

type distillPrompt struct {
	system string
	user   string
	joined string
	parts  []string
}

type distillShape struct {
	messages     int
	tools        int
	maxTokens    *int
	display      string
	thinkingType string
	budget       *int
	effort       string
	n            *int
}

// CheckDistill reports whether this request should be rejected.
// The second result is the rule name and is safe to log.
func CheckDistill(info *relaycommon.RelayInfo) (bool, string) {
	if info == nil {
		return false, ""
	}
	config := setting.CurrentDistillConfig()
	if !config.Checks(info.UserGroup) {
		return false, ""
	}
	if config.SkipOpenAI() && isGPTModel(info.OriginModelName) {
		return false, ""
	}
	rules := config.Rules()
	prompt, shape, ok := distillFromRequest(info.Request)
	if !ok {
		return false, ""
	}
	if phrase := containsPhrase(prompt.joined, rules.HardNeedles); phrase != "" {
		return true, distillRuleHarvestNeedle
	}
	if matchPattern(normalizeDistillText(prompt.joined), rules.Patterns) {
		return true, distillRuleRegex
	}
	if phrase := containsNormalized(prompt.user, rules.Fingerprints); phrase != "" {
		return true, distillRuleFingerprint
	}
	if phrase := containsPhrase(prompt.joined, rules.Needles); phrase != "" {
		return true, distillRuleNeedle
	}
	if contestMatches(rules.Structure.ContestPattern, prompt.joined) && isHarvest(shape, rules.Structure) {
		return true, distillRuleHarvest
	}
	// CotPatterns use the same latest-user window as the five catalogs above.
	if matchPattern(prompt.joined, rules.CotPatterns) {
		return true, distillRuleCot
	}
	// Keep message boundaries intact while removing whitespace between Han
	// characters. The original text is matched first for custom patterns.
	normalized := make([]string, len(prompt.parts))
	for i, part := range prompt.parts {
		normalized[i] = removeHanWhitespace(part)
	}
	if matchPattern(strings.Join(normalized, "\n"), rules.CotPatterns) {
		return true, distillRuleCot
	}
	return false, ""
}

func isGPTModel(name string) bool {
	name = strings.ToLower(strings.TrimSpace(name))
	if slash := strings.LastIndex(name, "/"); slash >= 0 {
		name = name[slash+1:]
	}
	return strings.HasPrefix(name, "gpt-")
}

func distillFromRequest(request dto.Request) (distillPrompt, distillShape, bool) {
	switch req := request.(type) {
	case *dto.GeneralOpenAIRequest:
		if req == nil {
			return distillPrompt{}, distillShape{}, false
		}
		return openAIPrompt(req), openAIShape(req), true
	case *dto.ClaudeRequest:
		if req == nil {
			return distillPrompt{}, distillShape{}, false
		}
		return claudePrompt(req), claudeShape(req), true
	case *dto.OpenAIResponsesRequest:
		if req == nil {
			return distillPrompt{}, distillShape{}, false
		}
		return responsesPrompt(req), responsesShape(req), true
	default:
		return distillPrompt{}, distillShape{}, false
	}
}

func openAIPrompt(req *dto.GeneralOpenAIRequest) distillPrompt {
	var systems []string
	var users []string
	for _, message := range req.Messages {
		role := strings.ToLower(message.Role)
		switch role {
		case "system", "developer":
			if text := contentToText(message.Content); text != "" {
				systems = append(systems, text)
			}
		case "user":
			if text := contentToText(message.Content); text != "" {
				users = append(users, text)
			}
		}
	}
	return joinPrompt(systems, users)
}

func openAIShape(req *dto.GeneralOpenAIRequest) distillShape {
	shape := distillShape{messages: len(req.Messages), tools: len(req.Tools), effort: req.ReasoningEffort, n: req.N}
	if req.MaxTokens != nil {
		value := int(*req.MaxTokens)
		shape.maxTokens = &value
	} else if req.MaxCompletionTokens != nil {
		value := int(*req.MaxCompletionTokens)
		shape.maxTokens = &value
	}
	return shape
}

func claudePrompt(req *dto.ClaudeRequest) distillPrompt {
	var systems []string
	if text := contentToText(req.System); text != "" {
		systems = append(systems, text)
	}
	var users []string
	for _, message := range req.Messages {
		role := strings.ToLower(message.Role)
		switch role {
		case "system", "developer":
			if text := contentToText(message.Content); text != "" {
				systems = append(systems, text)
			}
		case "user":
			if text := contentToText(message.Content); text != "" {
				users = append(users, text)
			}
		}
	}
	return joinPrompt(systems, users)
}

func claudeShape(req *dto.ClaudeRequest) distillShape {
	shape := distillShape{messages: len(req.Messages), tools: countAnyTools(req.Tools)}
	if req.MaxTokens != nil {
		value := int(*req.MaxTokens)
		shape.maxTokens = &value
	}
	if req.Thinking != nil {
		shape.display = req.Thinking.Display
		shape.thinkingType = req.Thinking.Type
		shape.budget = req.Thinking.BudgetTokens
	}
	return shape
}

func responsesPrompt(req *dto.OpenAIResponsesRequest) distillPrompt {
	var systems []string
	if text := rawText(req.Instructions); text != "" {
		systems = append(systems, text)
	}
	users := responseUserTexts(req.Input)
	return joinPrompt(systems, users)
}

func rawText(raw json.RawMessage) string {
	if len(raw) == 0 || string(raw) == "null" {
		return ""
	}
	var text string
	if err := json.Unmarshal(raw, &text); err == nil {
		return text
	}
	return contentToText(decodeAny(raw))
}

func decodeAny(raw json.RawMessage) any {
	var value any
	if err := json.Unmarshal(raw, &value); err != nil {
		return nil
	}
	return value
}

func responsesShape(req *dto.OpenAIResponsesRequest) distillShape {
	shape := distillShape{tools: countRawTools(req.Tools)}
	if len(req.Input) > 0 && string(req.Input) != "null" {
		shape.messages = 1
	}
	if req.MaxOutputTokens != nil {
		value := int(*req.MaxOutputTokens)
		shape.maxTokens = &value
	}
	if req.Reasoning != nil {
		shape.effort = req.Reasoning.Effort
	}
	return shape
}

func responseUserTexts(raw json.RawMessage) []string {
	if len(raw) == 0 || string(raw) == "null" {
		return nil
	}
	var text string
	if err := json.Unmarshal(raw, &text); err == nil {
		if strings.TrimSpace(text) == "" {
			return nil
		}
		return []string{text}
	}
	var items []map[string]any
	if err := json.Unmarshal(raw, &items); err != nil {
		return nil
	}
	var users []string
	for _, item := range items {
		role, _ := item["role"].(string)
		role = strings.ToLower(role)
		if role != "" && role != "user" {
			continue
		}
		content, ok := item["content"]
		if !ok {
			content = item
		}
		if text := contentToText(content); text != "" {
			users = append(users, text)
		}
	}
	return users
}

// joinPrompt keeps every system and developer message and only the last user
// message that still has text. A user turn with no text is dropped by the
// caller, and tool_result is omitted in contentToText, so a trailing turn
// that is only a tool result falls through to the previous user text.
//
// vm2api #453 replaced hardcoded distillation keywords with this window.
// Those keywords rejected ordinary research chat: self-supervised learning,
// CLIP distillation, and similar work say "distillation" without asking the
// model to hand over its reasoning. A harvest is that intent, not the word.
// Reading only the latest user message still catches a real request, and an
// earlier academic mention no longer rejects every later turn. tool_result
// is skipped because it is tool output, not the user's ask.
// https://github.com/dofastted/vm2api/pull/453
func joinPrompt(systems, users []string) distillPrompt {
	user := ""
	if len(users) > 0 {
		user = users[len(users)-1]
	}
	parts := append(append([]string{}, systems...), user)
	parts = compact(parts)
	return distillPrompt{system: strings.Join(systems, "\n"), user: user, joined: strings.Join(parts, "\n"), parts: parts}
}

// removeHanWhitespace removes whitespace only when both adjacent characters
// are Han, leaving English word boundaries and the original request intact.
func removeHanWhitespace(text string) string {
	runes := []rune(text)
	var out strings.Builder
	out.Grow(len(text))
	for i := 0; i < len(runes); {
		if !unicode.IsSpace(runes[i]) {
			out.WriteRune(runes[i])
			i++
			continue
		}
		start := i
		for i < len(runes) && unicode.IsSpace(runes[i]) {
			i++
		}
		if start > 0 && i < len(runes) && unicode.Is(unicode.Han, runes[start-1]) && unicode.Is(unicode.Han, runes[i]) {
			continue
		}
		out.WriteString(string(runes[start:i]))
	}
	return out.String()
}

func compact(items []string) []string {
	out := make([]string, 0, len(items))
	for _, item := range items {
		if strings.TrimSpace(item) != "" {
			out = append(out, item)
		}
	}
	return out
}

func contentToText(content any) string {
	switch value := content.(type) {
	case nil:
		return ""
	case string:
		return value
	case []any:
		parts := make([]string, 0, len(value))
		for _, item := range value {
			if text := contentToText(item); text != "" {
				parts = append(parts, text)
			}
		}
		return strings.Join(parts, "\n")
	case map[string]any:
		if typ, _ := value["type"].(string); typ == "tool_result" {
			return ""
		}
		if text, ok := value["text"].(string); ok {
			return text
		}
		if nested, ok := value["content"]; ok {
			return contentToText(nested)
		}
	}
	return ""
}

func countAnyTools(tools any) int {
	switch value := tools.(type) {
	case []any:
		return len(value)
	case json.RawMessage:
		return countRawTools(value)
	default:
		return 0
	}
}

func countRawTools(raw json.RawMessage) int {
	if len(raw) == 0 || string(raw) == "null" {
		return 0
	}
	var items []json.RawMessage
	if err := json.Unmarshal(raw, &items); err != nil {
		return 0
	}
	return len(items)
}

func wantsThinking(display, typ string, budget *int, effort string, n *int, floor int) bool {
	if strings.EqualFold(display, "summarized") {
		return true
	}
	if strings.EqualFold(typ, "enabled") && budget != nil && floor > 0 && *budget >= floor {
		return true
	}
	switch strings.ToLower(strings.TrimSpace(effort)) {
	case "high", "xhigh", "max":
		return true
	}
	return n != nil && *n > 1
}

func isHarvest(shape distillShape, structure setting.DistillStructure) bool {
	if structure.RequireNoTools && shape.tools > 0 {
		return false
	}
	if structure.RequireSingleTurn && shape.messages > 1 {
		return false
	}
	if structure.MinMaxTokens <= 0 {
		return false
	}
	if shape.maxTokens != nil && *shape.maxTokens < structure.MinMaxTokens {
		return false
	}
	return wantsThinking(shape.display, shape.thinkingType, shape.budget, shape.effort, shape.n, structure.MinMaxTokens)
}

func containsPhrase(text string, phrases []string) string {
	hay := strings.ToLower(text)
	if hay == "" {
		return ""
	}
	for _, phrase := range phrases {
		needle := strings.ToLower(strings.TrimSpace(phrase))
		if needle != "" && strings.Contains(hay, needle) {
			return phrase
		}
	}
	return ""
}

func normalizeDistillText(text string) string {
	text = strings.ToLower(text)
	text = distillWhitespace.ReplaceAllString(text, " ")
	text = distillBackslashGap.ReplaceAllString(text, `\`)
	return strings.TrimSpace(text)
}

func containsNormalized(text string, phrases []string) string {
	hay := normalizeDistillText(text)
	if hay == "" {
		return ""
	}
	for _, phrase := range phrases {
		needle := normalizeDistillText(phrase)
		if needle != "" && strings.Contains(hay, needle) {
			return phrase
		}
	}
	return ""
}

func contestMatches(pattern, text string) bool {
	pattern = strings.TrimSpace(pattern)
	if pattern == "" || len(pattern) > 400 || strings.TrimSpace(text) == "" {
		return false
	}
	re, err := regexp.Compile("(?i)" + pattern)
	if err != nil {
		return false
	}
	return re.MatchString(text)
}

func matchPattern(text string, patterns []string) bool {
	if strings.TrimSpace(text) == "" {
		return false
	}
	for _, source := range patterns {
		source = strings.TrimSpace(source)
		if source == "" || len(source) > 400 {
			continue
		}
		re, err := regexp.Compile("(?i)" + source)
		if err != nil {
			continue
		}
		for _, loc := range re.FindAllStringIndex(text, -1) {
			if distillNegationTail.MatchString(text[:loc[0]]) {
				continue
			}
			return true
		}
	}
	return false
}
