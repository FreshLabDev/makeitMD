// SPDX-License-Identifier: Apache-2.0
package metrics

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHandlerRendersPrometheusCounters(t *testing.T) {
	ConversionsSent.Inc()
	recorder := httptest.NewRecorder()
	Handler().ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d", recorder.Code)
	}
	for _, want := range []string{"# TYPE makeitmd_conversions_sent_total counter", "makeitmd_conversions_sent_total 1"} {
		if !strings.Contains(recorder.Body.String(), want) {
			t.Fatalf("metrics missing %q:\n%s", want, recorder.Body.String())
		}
	}
}
