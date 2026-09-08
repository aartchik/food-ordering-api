package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"food-ordering-api/internal/models"
)

type kitchenClient struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client
}

type apiError struct {
	Status int
	Body   string
}

func (e *apiError) Error() string {
	return fmt.Sprintf("kitchen API returned %d: %s", e.Status, e.Body)
}

func (c *kitchenClient) syncCatalog(ctx context.Context, input *models.RestaurantCatalogInput) error {
	return c.doJSON(ctx, http.MethodPut, "/v1/partner/catalog", input, nil)
}

func (c *kitchenClient) listOrders(ctx context.Context) ([]*models.OrderView, error) {
	var response struct {
		Orders []*models.OrderView `json:"orders"`
	}
	path := "/v1/partner/orders?page_size=100&sort=-created_at"
	if err := c.doJSON(ctx, http.MethodGet, path, nil, &response); err != nil {
		return nil, err
	}
	return response.Orders, nil
}

func (c *kitchenClient) updateOrderStatus(ctx context.Context, orderID int64, input *models.OrderStatusUpdateInput) error {
	path := fmt.Sprintf("/v1/partner/orders/%d/status", orderID)
	return c.doJSON(ctx, http.MethodPatch, path, input, nil)
}

func (c *kitchenClient) doJSON(ctx context.Context, method, path string, input, output any) (err error) {
	var body io.Reader
	if input != nil {
		data, marshalErr := json.Marshal(input)
		if marshalErr != nil {
			return marshalErr
		}
		body = bytes.NewReader(data)
	}

	request, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, body)
	if err != nil {
		return err
	}
	request.Header.Set("Authorization", "Bearer "+c.apiKey)
	request.Header.Set("Accept", "application/json")
	if input != nil {
		request.Header.Set("Content-Type", "application/json")
	}

	response, err := c.httpClient.Do(request)
	if err != nil {
		return err
	}
	defer func() {
		if closeErr := response.Body.Close(); closeErr != nil {
			err = errors.Join(err, fmt.Errorf("close response body: %w", closeErr))
		}
	}()

	if response.StatusCode < 200 || response.StatusCode >= 300 {
		data, readErr := io.ReadAll(io.LimitReader(response.Body, 4096))
		if readErr != nil {
			return readErr
		}
		return &apiError{Status: response.StatusCode, Body: strings.TrimSpace(string(data))}
	}
	if output == nil || response.StatusCode == http.StatusNoContent {
		return nil
	}
	if err := json.NewDecoder(response.Body).Decode(output); err != nil {
		return fmt.Errorf("decode kitchen API response: %w", err)
	}
	return nil
}
