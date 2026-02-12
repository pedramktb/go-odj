package odj

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/go-faster/jx"
	"github.com/ogen-go/ogen/ogenerrors"
	"github.com/ogen-go/ogen/validate"
	"github.com/pedramktb/go-ctxslog"
	"github.com/pedramktb/go-tagerr"
)

// OgenErrorHandler is a custom error handler for the Ogen framework that processes different types of errors
// and generates appropriate HTTP responses. It checks the error type and maps it to a corresponding tagged error,
// which is then logged and returned as a JSON response with the appropriate HTTP status code and error details.
// Errors of type tagerr.Err are returned as-is.
func OgenErrorHandler(ctx context.Context, w http.ResponseWriter, r *http.Request, err error) {
	var (
		dcParamErr *ogenerrors.DecodeParamsError
		dcBodyErr  *ogenerrors.DecodeBodyError
		secErr     *ogenerrors.SecurityError
		ctErr      *validate.InvalidContentTypeError
	)
	var tagErr *tagerr.Err
	switch {
	case errors.As(err, &tagErr):
	case errors.As(err, &dcParamErr),
		errors.As(err, &dcBodyErr),
		errors.As(err, &ctErr):
		tagErr = tagerr.ErrInvalidReq.Wrap(err)
	case errors.As(err, &secErr):
		tagErr = tagerr.ErrNotAuth.Wrap(err)
	default:
		tagErr = tagerr.ErrInternal.Wrap(err)
	}

	if tagErr.Is(tagerr.ErrInternal) {
		ctxslog.FromContext(ctx).ErrorContext(ctx, "internal error", slog.Any("error", tagErr), slog.String("stack_trace", string(tagErr.Stack())))
	}

	ogenWriteErrorJSON(w, tagErr.HTTPCode, tagErr.Tag, tagErr.Error())
}

// OgenEndpointNotFoundErrorHandler is a custom error handler for handling "endpoint not found" errors in the Ogen framework.
func OgenEndpointNotFoundErrorHandler(w http.ResponseWriter, r *http.Request) {
	ogenWriteErrorJSON(w, http.StatusNotFound, "endpoint_not_found", fmt.Sprintf("Requested endpoint [%s] could not be found", r.RequestURI))
}

// OgenMethodNotAllowedErrorHandler is a custom error handler for handling "method not allowed" errors in the Ogen framework.
func OgenMethodNotAllowedErrorHandler(w http.ResponseWriter, r *http.Request, allowed string) {
	ogenWriteErrorJSON(w, http.StatusMethodNotAllowed, "method_not_allowed", "Requested method [%s] is not allowed. Allowed methods are [%s]")
}

func ogenWriteErrorJSON(w http.ResponseWriter, statusCode int, code, detail string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(statusCode)

	e := jx.GetEncoder()
	defer jx.PutEncoder(e)

	e.ObjStart()
	e.FieldStart("code")
	e.StrEscape(code)
	e.FieldStart("detail")
	e.StrEscape(detail)
	e.ObjEnd()

	if _, err := w.Write(e.Bytes()); err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}
