package ginprometheus

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
)

func TestMiddlewareRequestMetrics(t *testing.T) {
	oldRegisterer, oldGatherer := prometheus.DefaultRegisterer, prometheus.DefaultGatherer
	registry := prometheus.NewRegistry()
	prometheus.DefaultRegisterer, prometheus.DefaultGatherer = registry, registry
	t.Cleanup(func() { prometheus.DefaultRegisterer, prometheus.DefaultGatherer = oldRegisterer, oldGatherer })
	engine := gin.New()
	metrics := NewPrometheus("qualification")
	metrics.ReqCntURLLabelMappingFn = func(c *gin.Context) string { return c.FullPath() }
	metrics.Use(engine)
	engine.GET("/items/:id", func(c *gin.Context) { c.String(http.StatusCreated, "created") })
	for _, path := range []string{"/items/one", "/items/two"} {
		response := httptest.NewRecorder()
		engine.ServeHTTP(response, httptest.NewRequest(http.MethodGet, path, nil))
		if response.Code != http.StatusCreated {
			t.Fatalf("request status = %d", response.Code)
		}
	}
	response := httptest.NewRecorder()
	engine.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), "qualification_requests_total") {
		t.Fatalf("metrics scrape status %d: %s", response.Code, response.Body.String())
	}
	families, err := registry.Gather()
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, family := range families {
		if family.GetName() != "qualification_requests_total" {
			continue
		}
		if len(family.Metric) != 1 {
			t.Fatalf("request series = %d; metrics scrapes must be excluded", len(family.Metric))
		}
		metric := family.Metric[0]
		if metric.GetCounter().GetValue() != 2 {
			t.Fatalf("request count = %v", metric.GetCounter().GetValue())
		}
		labels := map[string]string{}
		for _, label := range metric.Label {
			labels[label.GetName()] = label.GetValue()
		}
		if labels["code"] != "201" || labels["method"] != "GET" || labels["url"] != "/items/:id" {
			t.Fatalf("request labels = %v", labels)
		}
		found = true
	}
	if !found {
		t.Fatal("request counter missing")
	}
}
