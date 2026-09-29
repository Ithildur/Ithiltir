package nodeingest

// Error describes an acceptance failure independently of its transport.
type Error struct {
	Code  string
	Cause error
}

func (e *Error) Error() string        { return e.Code }
func (e *Error) Unwrap() error        { return e.Cause }
func unauthorized(err error) *Error   { return &Error{Code: "unauthorized", Cause: err} }
func unavailable(err error) *Error    { return &Error{Code: "service_unavailable", Cause: err} }
func invalidMetrics(err error) *Error { return &Error{Code: "invalid_metrics", Cause: err} }
func invalidStatic(err error) *Error  { return &Error{Code: "invalid_static_payload", Cause: err} }
