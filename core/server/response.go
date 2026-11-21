package server

import (
	stdhttp "net/http"

	"github.com/liu-jiangyuan/kratos-template/pkg/response"

	"github.com/go-kratos/kratos/v3/errors"
	"github.com/go-kratos/kratos/v3/transport/http"
	"google.golang.org/genproto/googleapis/api/httpbody"
)

const jsonContentType = "application/json; charset=utf-8"

// ResponseEncoder wraps normal business responses into the unified
// response.Response envelope ({code: 1, msg: "success", data: ...}) with HTTP 200.
func ResponseEncoder(w stdhttp.ResponseWriter, r *stdhttp.Request, v any) error {
	// Pass through raw HttpBody streams and HTTP redirects unchanged.
	switch v.(type) {
	case *httpbody.HttpBody, **httpbody.HttpBody, http.Redirector:
		return http.DefaultResponseEncoder(w, r, v)
	}

	var payload any
	switch resp := v.(type) {
	case *response.Response:
		payload = resp
	case response.Response:
		payload = resp
	default:
		payload = response.Success(v)
	}

	codec, _ := http.CodecForRequest(r, "Accept")
	data, err := codec.Marshal(payload)
	if err != nil {
		return err
	}
	w.Header().Set("Content-Type", jsonContentType)
	w.WriteHeader(stdhttp.StatusOK)
	_, err = w.Write(data)
	return err
}

// ErrorEncoder wraps business and parameter validation errors into the unified
// response.Response envelope ({code: 0, msg: "...", data: null}) while keeping
// the HTTP protocol status code at 200 OK.
func ErrorEncoder(w stdhttp.ResponseWriter, r *stdhttp.Request, err error) {
	if err == nil {
		return
	}
	var rd http.Redirector
	if errors.As(err, &rd) {
		url, code := rd.Redirect()
		stdhttp.Redirect(w, r, url, code)
		return
	}

	se := errors.FromError(err)
	msg := se.GetMessage()
	if msg == "" {
		msg = err.Error()
	}

	payload := response.Fail(msg)
	codec, _ := http.CodecForRequest(r, "Accept")
	body, marshalErr := codec.Marshal(payload)
	if marshalErr != nil {
		w.WriteHeader(stdhttp.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", jsonContentType)
	w.WriteHeader(stdhttp.StatusOK)
	_, _ = w.Write(body)
}
