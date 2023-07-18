package metrics

import (
	"context"
	"fmt"
	"github.com/prometheus/client_golang/api"
	v1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"

	"time"
)

type ScalingQueryEvaluator interface {
	Eval(ctx context.Context, prometheusUrl string, query string) (bool, error)
}

func NewQueryEvaluator() ScalingQueryEvaluator {
	return &prometheusQueryEvaluator{}
}

type prometheusQueryEvaluator struct {
}

func (r *prometheusQueryEvaluator) Eval(ctx context.Context, prometheusUrl string, query string) (bool, error) {
	apiClient, err := NewPrometheusClientXX(prometheusUrl)
	if err != nil {
		return false, fmt.Errorf("Unable to create Prometheus client: %v", err)
	}

	result, warnings, err := apiClient.Query(context.Background(), query, time.Now())
	if err != nil { //if the query fails we will not make a scaling decision
		return false, fmt.Errorf("Prometheus query [ %q ] failed with error: %v ", query, err)
	}
	if len(warnings) > 0 { //if there are warnings we will not make a scaling decision
		return false, fmt.Errorf("Warnings received: %v", err)
	}

	if result.Type() != model.ValVector {
		return false, fmt.Errorf("Prometheus result type not a Vector: %v", err)
	} else {
		for _, vector := range result.(model.Vector) {
			if vector.Value != 1 {
				return false, nil
			}
		}
		return true, nil
	}
}

func NewPrometheusClientXX(prometheusEndpoint string) (v1.API, error) {
	client, err := api.NewClient(api.Config{
		Address: prometheusEndpoint,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create client")
	}

	v1api := v1.NewAPI(client)
	return v1api, nil
}

type MockQueryEvaluator struct {
	response bool
	err      error
}

func (r *MockQueryEvaluator) Eval(ctx context.Context, prometheusUrl string, query string) (bool, error) {
	return r.response, r.err
}

func (r *MockQueryEvaluator) SetResponse(response bool, err error) {
	r.response = response
	r.err = err
}
