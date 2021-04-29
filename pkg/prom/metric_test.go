package prom

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/prometheus/client_golang/api"
	"github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"
)

func TestFetchMetrics(t *testing.T) {
	client, err := api.NewClient(api.Config{
		Address: "http://localhost:9090",
	})
	if err != nil {
		fmt.Printf("Error creating promClient: %v\n", err)
		t.Fail()
	}

	v1api := v1.NewAPI(client)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	result, warnings, err := v1api.Query(ctx, "kubevirt_vmi_memory_available_bytes{name=\"droid-14\"}", time.Now())
	if err != nil {
		fmt.Printf("Error querying Prometheus: %v\n", err)
		t.Fail()
	}
	if len(warnings) > 0 {
		fmt.Printf("Warnings: %v\n", warnings)
	}
	if result.Type() == model.ValVector {
		vec := result.(model.Vector)
		fmt.Printf("Result:\n%v\n", vec[0].Value)
	}
}