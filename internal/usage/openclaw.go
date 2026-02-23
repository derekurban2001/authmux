package usage

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

func collectOpenClawEvents(ctx context.Context, timezone string) ([]NormalizedEvent, []string, error) {
	if _, err := exec.LookPath("openclaw"); err != nil {
		return nil, []string{"openclaw binary not found in PATH; skipping OpenClaw usage import"}, nil
	}

	cmd := exec.CommandContext(ctx, "openclaw", "status", "--json", "--usage")
	out, err := cmd.Output()
	if err != nil {
		// status may return non-zero in some setups; treat as soft-fail
		return nil, []string{fmt.Sprintf("openclaw status --json --usage failed: %v", err)}, nil
	}

	var payload map[string]any
	if err := json.Unmarshal(out, &payload); err != nil {
		return nil, []string{fmt.Sprintf("openclaw usage payload parse failed: %v", err)}, nil
	}

	usageObj, _ := payload["usage"].(map[string]any)
	if usageObj == nil {
		return nil, []string{"openclaw usage payload has no usage section"}, nil
	}

	events := []NormalizedEvent{}
	notes := []string{}
	added := 0

	providers, _ := usageObj["providers"].(map[string]any)
	for provider, pvAny := range providers {
		pv, _ := pvAny.(map[string]any)
		if pv == nil {
			continue
		}

		sessions, _ := pv["sessions"].(map[string]any)
		recent, _ := sessions["recent"].([]any)
		for i, rAny := range recent {
			r, _ := rAny.(map[string]any)
			if r == nil {
				continue
			}
			inTok := int64(getFloatAny(r, "in", "input", "inputTokens", "inTokens"))
			outTok := int64(getFloatAny(r, "out", "output", "outputTokens", "outTokens"))
			cost := getFloatAny(r, "costUsd", "costUSD", "cost")
			if inTok == 0 && outTok == 0 && cost == 0 {
				continue
			}
			ts := firstNonEmpty(getStringAny(r, "updatedAt", "timestamp", "time"), time.Now().UTC().Format(time.RFC3339))
			sessionKey := firstNonEmpty(getStringAny(r, "sessionKey", "session", "key"), fmt.Sprintf("%s-recent-%d", provider, i+1))
			model := getStringAny(r, "model")

			events = append(events, NormalizedEvent{
				ID:                    fmt.Sprintf("openclaw-%s-%s-%d", sanitize(provider), sanitize(sessionKey), i+1),
				TimestampUTC:          ts,
				DateLocal:             dateLocal(ts, timezone),
				Tool:                  ToolOpenClaw,
				ProfileID:             "openclaw/" + provider,
				ProfileName:           provider,
				IsProfilexManaged:     false,
				SourceRoot:            "openclaw-status",
				SourceFile:            "openclaw:status",
				SessionID:             sessionKey,
				Project:               "",
				Model:                 model,
				IsFallbackModel:       false,
				InputTokens:           inTok,
				CachedInputTokens:     0,
				OutputTokens:          outTok,
				ReasoningOutputTokens: 0,
				CacheCreationTokens:   0,
				CacheReadTokens:       0,
				RawTotalTokens:        inTok + outTok,
				NormalizedTotalTokens: inTok + outTok,
				ObservedCostUSD:       cost,
				CalculatedCostUSD:     cost,
				EffectiveCostUSD:      cost,
				CostModeUsed:          CostModeDisplay,
			})
			added++
		}
	}

	if added == 0 {
		notes = append(notes, "openclaw usage returned no recent session rows")
	} else {
		notes = append(notes, fmt.Sprintf("openclaw usage imported %d recent session rows", added))
	}
	return events, notes, nil
}

func sanitize(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	if s == "" {
		return "x"
	}
	repl := strings.NewReplacer(" ", "-", "/", "-", "\\", "-", ":", "-", "|", "-", "\t", "-")
	s = repl.Replace(s)
	for strings.Contains(s, "--") {
		s = strings.ReplaceAll(s, "--", "-")
	}
	s = strings.Trim(s, "-")
	if s == "" {
		return "x"
	}
	return s
}
