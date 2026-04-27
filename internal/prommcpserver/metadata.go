package prommcpserver

import (
	"sort"
	"strings"
)

func coerceMetadataEntries(value any) []map[string]any {
	switch v := value.(type) {
	case []any:
		out := make([]map[string]any, 0, len(v))
		for _, e := range v {
			if m, ok := e.(map[string]any); ok {
				out = append(out, m)
			}
		}
		return out
	case map[string]any:
		return []map[string]any{v}
	default:
		return nil
	}
}

func normalizeMetadataMap(rawData any) map[string][]map[string]any {
	switch raw := rawData.(type) {
	case map[string]any:
		if nested, ok := raw["metadata"]; ok {
			return normalizeMetadataMap(nested)
		}
		if nested, ok := raw["data"]; ok {
			return normalizeMetadataMap(nested)
		}
		normalized := make(map[string][]map[string]any)
		for metricName, entries := range raw {
			entriesCoerced := coerceMetadataEntries(entries)
			if len(entriesCoerced) > 0 {
				normalized[metricName] = entriesCoerced
			}
		}
		if len(normalized) > 0 {
			return normalized
		}
		if mname, ok := raw["metric"].(string); ok {
			return map[string][]map[string]any{mname: {raw}}
		}
	case []any:
		grouped := make(map[string][]map[string]any)
		for _, entry := range raw {
			m, ok := entry.(map[string]any)
			if !ok {
				continue
			}
			metricName, ok := m["metric"].(string)
			if !ok {
				continue
			}
			grouped[metricName] = append(grouped[metricName], m)
		}
		return grouped
	}
	return map[string][]map[string]any{}
}

func metadataMatchesPattern(metricName string, entries []map[string]any, pattern string) bool {
	if containsFold(metricName, pattern) {
		return true
	}
	lp := strings.ToLower(pattern)
	for _, entry := range entries {
		for _, val := range entry {
			if s, ok := val.(string); ok && strings.Contains(strings.ToLower(s), lp) {
				return true
			}
		}
	}
	return false
}

func containsFold(s, sub string) bool {
	if sub == "" {
		return true
	}
	return strings.Contains(strings.ToLower(s), strings.ToLower(sub))
}

func sortedMetricNames(m map[string][]map[string]any) []string {
	names := make([]string, 0, len(m))
	for k := range m {
		names = append(names, k)
	}
	sort.Strings(names)
	return names
}
