package job

import (
	"fmt"
	"reflect"

	"github.com/go-logr/logr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"

	monitorv1alpha1 "github.com/fusion-app/gateway/api/v1alpha1"
)

type MonitorJobManager struct {
	client client.Client
	cache  cache.Cache
	logger logr.Logger
	// key: namespace/name
	jobCache map[string]*MonitorJob
}

func NewSyncJobManager(mgrCache cache.Cache, mgrClient client.Client) *MonitorJobManager {
	return &MonitorJobManager{
		client:   mgrClient,
		cache:    mgrCache,
		logger:   ctrl.Log.WithName("job_manager"),
		jobCache: make(map[string]*MonitorJob),
	}
}

func (m *MonitorJobManager) NewJobOrExist(monitorRef *monitorv1alpha1.ResourceMonitor) *MonitorJob {
	cacheKey := jobCacheKey(monitorRef)
	oldJob, exists := m.jobCache[cacheKey]
	if !exists {
		m.logger.Info("Create MonitorJob")
		newJob := NewMonitorJob(monitorRef, m.logger, m.cache, m.client)
		newJob.Start()
		m.jobCache[cacheKey] = newJob
		return newJob
	}
	// check whether reset old job
	if reflect.DeepEqual(monitorRef.Spec, oldJob.MonitorSpec) {
		m.logger.Info("Use exist MonitorJob")
		return oldJob
	}
	oldJob.Cancel()
	m.logger.Info("Renew old MonitorJob")
	newJob := NewMonitorJob(monitorRef, m.logger, m.cache, m.client)
	newJob.Start()
	m.jobCache[cacheKey] = newJob
	return newJob
}

func (m *MonitorJobManager) CleanJob(monitorRef *monitorv1alpha1.ResourceMonitor) {
	cacheKey := jobCacheKey(monitorRef)
	oldJob, exists := m.jobCache[cacheKey]
	if !exists {
		return
	}
	oldJob.Cancel()
	delete(m.jobCache, cacheKey)
	return
}

func jobCacheKey(monitorRef *monitorv1alpha1.ResourceMonitor) string {
	return fmt.Sprintf("%s/%s", monitorRef.GetNamespace(), monitorRef.GetName())
}
