package http

type (
	BackstopError struct {
		ErrorID string `json:"error_id"`
		Errors  Errors `json:"errors"`
	}

	Errors []struct {
		Message string `json:"message"`
		Code    int    `json:"code"`
	}
)
