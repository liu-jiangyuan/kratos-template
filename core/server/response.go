package server

import (
	"encoding/json"
	stdhttp "net/http"

	"github.com/liu-jiangyuan/kratos-template/pkg/response"

	"github.com/go-kratos/kratos/v3/encoding"
	"github.com/go-kratos/kratos/v3/errors"
	"github.com/go-kratos/kratos/v3/transport/http"
	"google.golang.org/genproto/googleapis/api/httpbody"
)

const jsonContentType = "application/json; charset=utf-8"

// marshalJSON always uses the JSON codec. The response envelope is a plain Go
// struct, so it cannot be encoded by codecs such as protojson. Negotiating on
// Accept would make requests with "Accept: application/protojson" fail; the
// HTTP API contract is JSON regardless of what the client asks for.
func marshalJSON(v any) ([]byte, error) {
	if codec := encoding.GetCodec("json"); codec != nil {
		return codec.Marshal(v)
	}
	return json.Marshal(v)
}

func writeJSON(w stdhttp.ResponseWriter, status int, v any) error {
	body, err := marshalJSON(v)
	if err != nil {
		return err
	}
	w.Header().Set("Content-Type", jsonContentType)
	w.WriteHeader(status)
	_, err = w.Write(body)
	return err
}

// ResponseEncoder wraps normal business responses into the unified
// response.Response envelope ({code: 1, message: "success", data: ...}) with HTTP 200.
// Raw HttpBody streams and HTTP redirects are passed through unchanged.
func ResponseEncoder(w stdhttp.ResponseWriter, r *stdhttp.Request, v any) error {
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

	return writeJSON(w, stdhttp.StatusOK, payload)
}

// ErrorEncoder keeps the unified body envelope for every error:
//
//   - business/handler errors (validation, logic, not found, conflict, ...)
//     keep HTTP 200 with code=0;
//   - protocol, auth and server errors use the real HTTP status
//     (400/401/403/429/499/500/501/502/503/504) with code=0.
//
// The rule is documented in README.md. The mapping is based on the HTTP status
// carried by *errors.Error; Kratos stores the HTTP status in Error.Code.
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
	status := errorHTTPStatus(se)
	msg := errorMessage(status, se, err)
	_ = writeJSON(w, status, response.Fail(msg))
}

// errorHTTPStatus decides the HTTP status for an error.
//
// 400, 404, 409, 412 and 422 are business-level codes produced by handlers or
// the validation middleware, so they are flattened to 200. A 400 caused by the
// request codec (malformed body, wrong query type, ...) is a protocol error and
// keeps 400. Other 4xx/5xx statuses (auth, throttling, internal, gateway, ...)
// are preserved.
func errorHTTPStatus(se *errors.Error) int {
	if se == nil {
		return stdhttp.StatusInternalServerError
	}
	if se.Reason == "CODEC" {
		return stdhttp.StatusBadRequest
	}
	switch se.Code {
	case stdhttp.StatusBadRequest,
		stdhttp.StatusNotFound,
		stdhttp.StatusConflict,
		stdhttp.StatusPreconditionFailed,
		stdhttp.StatusUnprocessableEntity:
		return stdhttp.StatusOK
	}
	if se.Code >= 400 && se.Code <= 599 {
		return int(se.Code)
	}
	return stdhttp.StatusInternalServerError
}

// errorMessage returns a client-safe message. 5xx messages are never taken from
// the underlying error, which may contain SQL or other internal details.
func errorMessage(status int, se *errors.Error, err error) string {
	switch status {
	case stdhttp.StatusInternalServerError:
		return "internal server error"
	case stdhttp.StatusNotImplemented:
		return "not implemented"
	case stdhttp.StatusBadGateway:
		return "bad gateway"
	case stdhttp.StatusServiceUnavailable:
		return "service unavailable"
	case stdhttp.StatusGatewayTimeout:
		return "gateway timeout"
	case 499:
		return "client closed request"
	}
	if se != nil {
		if msg := se.GetMessage(); msg != "" {
			return msg
		}
	}
	if err != nil {
		return err.Error()
	}
	return "request failed"
}

// EnvelopeNotFoundHandler is the router-level 404 handler. Requests that match
// no route never enter the application handler chain, so they keep the real 404
// status while still using the unified body shape.
func EnvelopeNotFoundHandler() stdhttp.Handler {
	return stdhttp.HandlerFunc(func(w stdhttp.ResponseWriter, _ *stdhttp.Request) {
		_ = writeJSON(w, stdhttp.StatusNotFound, response.Fail("not found"))
	})
}

// EnvelopeMethodNotAllowedHandler is the router-level 405 handler.
func EnvelopeMethodNotAllowedHandler() stdhttp.Handler {
	return stdhttp.HandlerFunc(func(w stdhttp.ResponseWriter, _ *stdhttp.Request) {
		_ = writeJSON(w, stdhttp.StatusMethodNotAllowed, response.Fail("method not allowed"))
	})
}
