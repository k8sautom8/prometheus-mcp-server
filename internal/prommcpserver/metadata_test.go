package prommcpserver

import (
	"testing"
)

func TestCoerceMetadataEntries(t *testing.T) {
	t.Run("dict", func(t *testing.T) {
		got := coerceMetadataEntries(map[string]any{"type": "gauge", "help": "Up"})
		if len(got) != 1 || got[0]["type"] != "gauge" {
			t.Fatalf("got %#v", got)
		}
	})
	t.Run("unsupported", func(t *testing.T) {
		if coerceMetadataEntries("x") != nil {
			t.Fatal()
		}
		if coerceMetadataEntries(42) != nil {
			t.Fatal()
		}
	})
}

func TestNormalizeMetadataMap(t *testing.T) {
	t.Run("skips non dict list entries", func(t *testing.T) {
		got := normalizeMetadataMap([]any{
			"not_a_dict",
			map[string]any{"metric": "up", "type": "gauge"},
		})
		if len(got) != 1 || len(got["up"]) != 1 {
			t.Fatalf("got %#v", got)
		}
	})
	t.Run("empty for bad dict", func(t *testing.T) {
		got := normalizeMetadataMap(map[string]any{"foo": "bar", "baz": 42.0})
		if len(got) != 0 {
			t.Fatalf("got %#v", got)
		}
	})
}

func TestMetadataMatchesPattern(t *testing.T) {
	if !metadataMatchesPattern("http_requests_total", []map[string]any{{"type": "counter"}}, "http") {
		t.Fatal()
	}
	if metadataMatchesPattern("up", []map[string]any{{"type": "gauge", "help": "availability"}}, "http") {
		t.Fatal()
	}
}

func TestSortedMetricNamesPagination(t *testing.T) {
	m := map[string][]map[string]any{
		"metric_c": {{}},
		"metric_a": {{}},
		"metric_b": {{}},
	}
	names := sortedMetricNames(m)
	if names[0] != "metric_a" || names[1] != "metric_b" || names[2] != "metric_c" {
		t.Fatalf("got %v", names)
	}
}
