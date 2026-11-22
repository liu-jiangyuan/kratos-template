package response

const (
	// CodeFail indicates a failed response (0).
	CodeFail = 0
	// CodeSuccess indicates a successful response (1).
	CodeSuccess = 1

	// DefaultSuccessMsg is the default message for a successful response.
	DefaultSuccessMsg = "success"
)

// Response defines the fixed structure for API response payloads.
type Response struct {
	Code    int    `json:"code"`    // 成功：1   失败：0
	Message string `json:"message"` // success || 失败的原因或者提示
	Data    any    `json:"data"`    // 具体的数据格式
}

// New creates a Response with the specified code, message, and data.
func New(code int, message string, data any) *Response {
	return &Response{
		Code:    code,
		Message: message,
		Data:    data,
	}
}

// Success builds a successful Response (Code=1, Message="success") carrying data.
func Success(data any) *Response {
	return &Response{
		Code:    CodeSuccess,
		Message: DefaultSuccessMsg,
		Data:    data,
	}
}

// SuccessWithMsg builds a successful Response (Code=1) with a custom message and data.
func SuccessWithMsg(message string, data any) *Response {
	if message == "" {
		message = DefaultSuccessMsg
	}
	return &Response{
		Code:    CodeSuccess,
		Message: message,
		Data:    data,
	}
}

// Fail builds a failed Response (Code=0, Data=nil) with the given failure reason or prompt.
func Fail(message string) *Response {
	return &Response{
		Code:    CodeFail,
		Message: message,
		Data:    nil,
	}
}

// FailWithData builds a failed Response (Code=0) with a failure message and extra data.
func FailWithData(message string, data any) *Response {
	return &Response{
		Code:    CodeFail,
		Message: message,
		Data:    data,
	}
}

// FromError builds a failed Response (Code=0) from err, or a successful Response if err is nil.
func FromError(err error) *Response {
	if err == nil {
		return Success(nil)
	}
	return Fail(err.Error())
}
