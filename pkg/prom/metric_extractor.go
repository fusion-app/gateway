package prom

import (
	"context"
	"fmt"
	"math/rand"
	"sync"
	"time"

	"github.com/go-logr/logr"
	"github.com/prometheus/client_golang/api"
	"github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"
	ctrl "sigs.k8s.io/controller-runtime"

	monitorv1alpha1 "github.com/fusion-app/gateway/api/v1alpha1"
)

type MetricQuery struct {
	Field        string `json:"field"`
	Metric       string `json:"metric"`
	ResName      string `json:"res_name"`
	ResNamespace string `json:"res_namespace"`
}

func newMetricQuery(namespace, name, metric string) string {
	return fmt.Sprintf("%s{namespace=\"%s\",name=\"%s\"}", metric, namespace, name)
}

type MetricResult struct {
	ResName      string             `json:"res_name"`
	ResNamespace string             `json:"res_namespace"`
	Fields       map[string]float64 `json:"fields"`
}

type MetricWorker struct {
	promClient v1.API
	logger     logr.Logger
	queryStore map[string]*MetricQuery

	cancel    context.CancelFunc
	ctx       context.Context
	parentCtx context.Context
	stopped   chan struct{}
	resultCh  chan<- *MetricResult
	mtx       sync.RWMutex
}

func NewMetricWorker(parentCtx context.Context, resultCh chan<- *MetricResult, cfg *monitorv1alpha1.PrometheusDataSource) *MetricWorker {
	logger := ctrl.Log.WithName("metric")

	client, err := api.NewClient(api.Config{
		Address: fmt.Sprintf("%s://%s:%d", cfg.Scheme, cfg.Host, cfg.Port),
	})
	if err != nil {
		logger.Error(err, "Creating promClient failed")
		return nil
	}

	cancelContext, cancelFunc := context.WithCancel(parentCtx)

	return &MetricWorker{
		promClient: v1.NewAPI(client),
		logger:     logger,
		queryStore: make(map[string]*MetricQuery),
		cancel:     cancelFunc,
		ctx:        cancelContext,
		parentCtx:  parentCtx,
		stopped:    make(chan struct{}),
		resultCh:   resultCh,
	}
}

func (h *MetricWorker) AddQuery(namespace, name, field, metric string) {
	query := newMetricQuery(namespace, name, metric)
	h.mtx.RLock()
	_, exists := h.queryStore[query]
	h.mtx.RUnlock()
	if !exists {
		h.mtx.Lock()
		defer h.mtx.Unlock()
		h.queryStore[query] = &MetricQuery{
			Field:        field,
			Metric:       metric,
			ResName:      name,
			ResNamespace: namespace,
		}
	}
}

func (h *MetricWorker) DeleteQuery(namespace, name, metric string) {
	query := newMetricQuery(namespace, name, metric)
	h.mtx.Lock()
	defer h.mtx.Unlock()
	delete(h.queryStore, query)
}

func (h *MetricWorker) Start(interval time.Duration) {
	delay := 100 + rand.Intn(400)
	select {
	case <-time.After(time.Duration(delay) * time.Millisecond):
	case <-h.ctx.Done():
		close(h.stopped)
		return
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

mainLoop:
	for {
		select {
		case <-h.parentCtx.Done():
			close(h.stopped)
			return
		case <-h.ctx.Done():
			break mainLoop
		default:
		}

		start := time.Now()
		h.doMetric()
		h.logger.Info(fmt.Sprintf("Metric loop once, cost %s", time.Now().Sub(start).String()))

		select {
		case <-h.parentCtx.Done():
			close(h.stopped)
			return
		case <-h.ctx.Done():
			break mainLoop
		case <-ticker.C:
		}
	}

	close(h.stopped)
}

func (h *MetricWorker) Stop() {
	h.cancel()
	<-h.stopped
}

func (h *MetricWorker) doMetric() {
	h.mtx.RLock()
	defer h.mtx.RUnlock()
	resultCache := make(map[string]*MetricResult)

	for query, queryObj := range h.queryStore {
		queryRes, warnings, err := h.promClient.Query(h.parentCtx, query, time.Now())
		if err != nil {
			h.logger.Error(err, "Querying Prometheus failed")
			continue
		}
		if len(warnings) > 0 {
			h.logger.Info("Querying Prometheus", "warnings", warnings)
		}
		var queryVal float64
		//h.logger.Info("Querying Prometheus", "query", query, "result", queryRes)
		if queryRes.Type() == model.ValVector {
			vec := queryRes.(model.Vector)
			if len(vec) == 0 {
				continue
			}
			queryVal = float64(vec[0].Value)
		}

		resultCacheKey := fmt.Sprintf("%s/%s", queryObj.ResNamespace, queryObj.ResName)
		if result, exists := resultCache[resultCacheKey]; exists {
			result.Fields[queryObj.Field] = queryVal
		} else {
			fields := make(map[string]float64)
			fields[queryObj.Field] = queryVal
			resultCache[resultCacheKey] = &MetricResult{
				ResName:      queryObj.ResName,
				ResNamespace: queryObj.ResNamespace,
				Fields:       fields,
			}
		}
	}

	for _, result := range resultCache {
		h.resultCh <- result
	}
}
