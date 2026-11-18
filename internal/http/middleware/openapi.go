package middleware

import (
	"context"
	"errors"
	"net/http"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/getkin/kin-openapi/openapi3filter"
	nethttpmiddleware "github.com/oapi-codegen/nethttp-middleware"

	"github.com/MaksimKosarevHSE/avito-lab/internal/http/domainerror"
)

var (
	rfc9562UUIDValidator = openapi3.NewRegexpFormatValidator(
		openapi3.FormatOfStringForUUIDOfRFC9562,
	)
	errInvalidUUID = errors.New("must be a valid UUID")
)

func OpenAPIValidator(
	spec *openapi3.T,
	problems *domainerror.Writer,
) func(http.Handler) http.Handler {
	return nethttpmiddleware.OapiRequestValidatorWithOptions(
		spec,
		&nethttpmiddleware.Options{
			DoNotValidateServers:  true,
			DiscardValidatedValue: true,
			ErrorHandlerWithOpts: func(
				ctx context.Context,
				err error,
				w http.ResponseWriter,
				r *http.Request,
				options nethttpmiddleware.ErrorHandlerOpts,
			) {
				writeValidationError(ctx, err, w, r, options, problems)
			},
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
	problems *domainerror.Writer,
) {
	if options.StatusCode == http.StatusBadRequest {
		problems.WriteValidationError(w, r, err)
		return
	}

	problems.Write(w, r, err)
}

func validateUUID(value string) error {
	if err := rfc9562UUIDValidator.Validate(value); err != nil {
		return errInvalidUUID
	}

	return nil
}
