package request

import (
	"net/http/httptest"
	"strings"
	"testing"
)

func TestDecodeJSONOrWriteErrorIgnoresUnknownFields(t *testing.T) {
	type input struct {
		Name string `json:"name"`
	}

	req := httptest.NewRequest("POST", "/", strings.NewReader(`{"name":"node","future_field":true}`))
	recorder := httptest.NewRecorder()
	var got input
	if ok := DecodeJSONOrWriteError(recorder, req, &got); !ok {
		t.Fatalf("DecodeJSONOrWriteError() = false, status = %d, body = %q", recorder.Code, recorder.Body.String())
	}
	if got.Name != "node" {
		t.Fatalf("decoded name = %q, want node", got.Name)
	}
}
