package push

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/google/uuid"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"

	pushapi "github.com/MaksimKosarevHSE/avito-lab/api/push"
)

var (
	ErrServiceUnavailable = errors.New("push service unavailable 5xx")
	ErrRequestError       = errors.New("push service respond 4xx")
	ErrUnexpectedResponse = errors.New("push service respond unexpected error")
)

type PushHTTPClient struct {
	client    pushapi.ClientWithResponsesInterface
	transport *http.Transport
}

func NewPushHTTPClient(
	httpURL string,
	requestTimeout time.Duration,
) (*PushHTTPClient, error) {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	client := &http.Client{
		Timeout:   requestTimeout,
		Transport: otelhttp.NewTransport(transport),
	}
	generatedClient, err := pushapi.NewClientWithResponses(
		httpURL,
		pushapi.WithHTTPClient(client),
	)
	if err != nil {
		return nil, err
	}

	return &PushHTTPClient{
		client:    generatedClient,
		transport: transport,
	}, nil
}

func (c *PushHTTPClient) CloseIdleConnections() {
	c.transport.CloseIdleConnections()
}

func (c *PushHTTPClient) PushPosition(ctx context.Context, tripID, driverID uuid.UUID) error {
	data := map[string]string{"trip_id": tripID.String()}
	body := pushapi.SendPushJSONRequestBody{
		RequestId:   uuid.NewString(),
		Data:        &data,
		RecipientId: driverID,
		Kind:        pushapi.REQUESTPOSITION,
	}

	response, err := c.client.SendPushWithResponse(ctx, body)
	if err != nil {
		return fmt.Errorf("send push: %w", err)
	}
	status := response.HTTPResponse.StatusCode
	if status == http.StatusAccepted {
		return nil
	}

	switch {
	case status >= http.StatusInternalServerError:
		return fmt.Errorf("%w: status code %d", ErrServiceUnavailable, status)
	case status >= http.StatusBadRequest && status < http.StatusInternalServerError:
		return fmt.Errorf("%w: status code %d", ErrRequestError, status)
	default:
		return fmt.Errorf("%w: status code %d", ErrUnexpectedResponse, status)
	}
}
