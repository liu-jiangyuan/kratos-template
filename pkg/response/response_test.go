package response

import (
	"encoding/json"
	"errors"
	"testing"
)

func TestResponseSuccessAndFail(t *testing.T) {
	okResp := Success(map[string]string{"id": "123"})
	if okResp.Code != CodeSuccess || okResp.Message != DefaultSuccessMsg {
		t.Fatalf("unexpected success response: %+v", okResp)
	}
	raw, err := json.Marshal(okResp)
	if err != nil {
		t.Fatalf("json.Marshal(okResp) error = %v", err)
	}
	if string(raw) != `{"code":1,"message":"success","data":{"id":"123"}}` {
		t.Fatalf("unexpected success JSON: %s", string(raw))
	}

	failResp := Fail("invalid parameter")
	if failResp.Code != CodeFail || failResp.Message != "invalid parameter" || failResp.Data != nil {
		t.Fatalf("unexpected fail response: %+v", failResp)
	}
	rawFail, err := json.Marshal(failResp)
	if err != nil {
		t.Fatalf("json.Marshal(failResp) error = %v", err)
	}
	if string(rawFail) != `{"code":0,"message":"invalid parameter","data":null}` {
		t.Fatalf("unexpected fail JSON: %s", string(rawFail))
	}

	errResp := FromError(errors.New("database error"))
	if errResp.Code != CodeFail || errResp.Message != "database error" {
		t.Fatalf("unexpected FromError response: %+v", errResp)
	}
}
