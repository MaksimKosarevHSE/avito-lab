package middleware

import (
	"context"
	"errors"
	"net/http"

	"github.com/MaksimKosarevHSE/avito-lab/internal/http/domainerror"
	"github.com/getkin/kin-openapi/openapi3"
	"github.com/getkin/kin-openapi/openapi3filter"
	nethttpmiddleware "github.com/oapi-codegen/nethttp-middleware"
)

var (
	rfc9562UUIDValidator = openapi3.NewRegexpFormatValidator(
		openapi3.FormatOfStringForUUIDOfRFC9562,
	)
	errInvalidUUID = errors.New("must be a valid UUID")
)

func OpenAPIValidator(spec *openapi3.T) func(http.Handler) http.Handler {
	return nethttpmiddleware.OapiRequestValidatorWithOptions(
		spec,
		&nethttpmiddleware.Options{
			DoNotValidateServers:  true,
			DiscardValidatedValue: true,
			ErrorHandlerWithOpts:  writeValidationError,
			Options: openapi3filter.Options{
				SchemaValidationOptions: []openapi3.SchemaValidationOption{
					openapi3.DisableSchemaErrorDetails(),
					openapi3.EnableFormatValidation(),
					openapi3.WithStringFormatValidator(
						"uuid",
						openapi3.NewCallbackValidator(validateUUID),
					),
				},
			},
		},
	)
}

func writeValidationError(
	_ context.Context,
	err error,
	w http.ResponseWriter,
	r *http.Request,
	options nethttpmiddleware.ErrorHandlerOpts,
) {
	if options.StatusCode == http.StatusBadRequest {
		domainerror.WriteValidationError(w, r, err)
		return
	}

	domainerror.Write(w, r, err)
}

func validateUUID(value string) error {
	if err := rfc9562UUIDValidator.Validate(value); err != nil {
		return errInvalidUUID
	}

	return nil
}
