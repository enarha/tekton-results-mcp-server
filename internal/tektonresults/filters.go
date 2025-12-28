package tektonresults

import (
	"fmt"
	"strings"
)

func parseLabelSelector(selector string) (map[string]string, error) {
	result := make(map[string]string)
	if strings.TrimSpace(selector) == "" {
		return result, nil
	}
	pairs := strings.Split(selector, ",")
	for _, pair := range pairs {
		pair = strings.TrimSpace(pair)
		if pair == "" {
			continue
		}
		parts := strings.SplitN(pair, "=", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid label selector %q: expected key=value pairs", pair)
		}
		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])
		if key == "" || value == "" {
			return nil, fmt.Errorf("invalid label selector %q: empty key or value", pair)
		}
		result[key] = value
	}
	return result, nil
}

func matchesLabels(actual map[string]string, expected map[string]string) bool {
	if len(expected) == 0 {
		return true
	}
	for key, want := range expected {
		if actual == nil {
			return false
		}
		if actual[key] != want {
			return false
		}
	}
	return true
}

func buildFilterExpression(kind resourceKind, labels map[string]string, exactName string, uid string) string {
	var parts []string
	if types, ok := resourceTypeFilters[kind]; ok && len(types) > 0 {
		var clauses []string
		for _, t := range types {
			clauses = append(clauses, fmt.Sprintf(`data_type=="%s"`, escapeCELString(t)))
		}
		parts = append(parts, fmt.Sprintf("(%s)", strings.Join(clauses, " || ")))
	}
	// Note: UID filtering is not supported in CEL filter expressions for Records.
	// UID filtering is handled in-memory after fetching records.
	// The uid parameter is kept for API consistency but not used in the CEL expression.
	for key, value := range labels {
		parts = append(parts, fmt.Sprintf(`data.metadata.labels["%s"]=="%s"`, escapeCELString(key), escapeCELString(value)))
	}
	if exactName != "" {
		parts = append(parts, fmt.Sprintf(`data.metadata.name=="%s"`, escapeCELString(exactName)))
	}
	return strings.Join(parts, " && ")
}

func escapeCELString(in string) string {
	in = strings.ReplaceAll(in, `\`, `\\`)
	in = strings.ReplaceAll(in, `"`, `\"`)
	return in
}

func parentForNamespace(ns string) string {
	ns = strings.TrimSpace(ns)
	switch strings.ToLower(ns) {
	case "", "-", "all", "*":
		return "-/results/-"
	default:
		return fmt.Sprintf("%s/results/-", ns)
	}
}
