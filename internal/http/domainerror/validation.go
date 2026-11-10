package domainerror

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/getkin/kin-openapi/openapi3filter"

	"github.com/MaksimKosarevHSE/avito-lab/api"
)

func (writer *Writer) WriteValidationError(w http.ResponseWriter, r *http.Request, err error) {
	writer.WriteInvalidRequest(w, r, validationDetail(err))
}

func validationDetail(err error) string {
	var maxBytesError *http.MaxBytesError
	if errors.As(err, &maxBytesError) {
		return "request body is too large"
	}

	var requestError *openapi3filter.RequestError
	if errors.As(err, &requestError) && requestError.Parameter != nil {
		return fmt.Sprintf("parameter %q is invalid", requestError.Parameter.Name)
	}

	var schemaError *openapi3.SchemaError
	if errors.As(err, &schemaError) {
		if path := schemaError.JSONPointer(); len(path) > 0 {
			return fmt.Sprintf(
				`field %q: %s`,
				strings.Join(path, "."),
				schemaError.Reason,
			)
		}

		return "request body: " + schemaError.Reason
	}

	if requestError != nil {
		if errors.Is(requestError.Err, openapi3filter.ErrInvalidRequired) {
			return "request body is required"
		}
		if requestError.Reason != "" {
			return requestError.Reason
		}
	}

	var invalidParameter *api.InvalidParamFormatError
	if errors.As(err, &invalidParameter) {
		return fmt.Sprintf("parameter %q has invalid format", invalidParameter.ParamName)
	}

	var tooManyValues *api.TooManyValuesForParamError
	if errors.As(err, &tooManyValues) {
		return fmt.Sprintf("parameter %q must have exactly one value", tooManyValues.ParamName)
	}

	return invalidRequestDetail
}
