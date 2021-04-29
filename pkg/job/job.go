package job

import (
	"context"
	"time"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	toolscache "k8s.io/client-go/tools/cache"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"

	monitorv1alpha1 "github.com/fusion-app/gateway/api/v1alpha1"
	"github.com/fusion-app/gateway/pkg/msg"
	"github.com/fusion-app/gateway/pkg/prom"
	"github.com/fusion-app/gateway/pkg/utils"
)

type MonitorJob struct {
	MonitorSpec *monitorv1alpha1.ResourceMonitorSpec

	monitorGVK  schema.GroupVersionKind
	monitorName string
	interestGVK schema.GroupVersionKind

	ctx    context.Context
	cancel context.CancelFunc

	msgStore     *msg.MessageStore
	metricWorker *prom.MetricWorker
	resultCh     <-chan *prom.MetricResult
	logger       logr.Logger
	mgrCache     cache.Cache
	mgrClient    client.Client
}

func NewMonitorJob(ref *monitorv1alpha1.ResourceMonitor, logger logr.Logger, mgrCache cache.Cache, mgrClient client.Client) *MonitorJob {
	jobContext, jobCancel := context.WithCancel(context.TODO())
	interestGVK := ref.Spec.Selector.GVK
	if ref.Spec.MsgBuilder.MsgSource.PrometheusSource == nil {
		return nil
	}
	resultCh := make(chan *prom.MetricResult)
	worker := prom.NewMetricWorker(jobContext, resultCh, ref.Spec.MsgBuilder.MsgSource.PrometheusSource)
	return &MonitorJob{
		MonitorSpec: ref.Spec.DeepCopy(),
		monitorGVK:  ref.GroupVersionKind(),
		monitorName: ref.GetName(),
		interestGVK: schema.GroupVersionKind{
			Group:   interestGVK.Group,
			Version: interestGVK.Version,
			Kind:    interestGVK.Kind,
		},
		ctx:          jobContext,
		cancel:       jobCancel,
		msgStore:     msg.NewMsgStore(ref),
		metricWorker: worker,
		resultCh:     resultCh,
		logger:       logger,
		mgrCache:     mgrCache,
		mgrClient:    mgrClient,
	}
}

func (j *MonitorJob) listRelatedResource() (*unstructured.UnstructuredList, error) {
	objList := &unstructured.UnstructuredList{}
	objList.SetAPIVersion(j.interestGVK.GroupVersion().String())
	objList.SetKind(j.interestGVK.Kind + "List")
	listOpt := client.MatchingLabels{}
	for k, v := range j.MonitorSpec.Selector.Labels {
		listOpt[k] = v
	}
	err := j.mgrClient.List(j.ctx, objList, listOpt)
	if err != nil {
		return nil, err
	}
	return objList, nil
}

func (j *MonitorJob) Start() {
	j.updateResourceStatus()
	informer, err := j.mgrCache.GetInformerForKind(context.TODO(), j.interestGVK)
	if err != nil {
		j.logger.Error(err, "Build informer failed")
		return
	}
	informer.AddEventHandler(toolscache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			u := utils.ToUnstructured(obj)
			if !j.isRelated(u) {
				return
			}
			if u.GetKind() == "VirtualMachineInstance" {
				j.metricWorker.AddQuery(u.GetNamespace(), u.GetName(), "mem_use", "kubevirt_vmi_memory_resident_bytes")
				j.metricWorker.AddQuery(u.GetNamespace(), u.GetName(), "cpu_sec", "kubevirt_vmi_vcpu_seconds")
			}
			j.msgStore.OnResourceAdd(obj, u)
			j.updateResourceStatus()
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			oldU := utils.ToUnstructured(oldObj)
			newU := utils.ToUnstructured(newObj)
			if !j.isRelated(oldU) && !j.isRelated(newU) {
				return
			}
			j.msgStore.OnResourceUpdate(newObj, newU)
		},
		DeleteFunc: func(obj interface{}) {
			u := utils.ToUnstructured(obj)
			if !j.isRelated(u) {
				return
			}

			if u.GetKind() == "VirtualMachineInstance" {
				j.metricWorker.DeleteQuery(u.GetNamespace(), u.GetName(), "kubevirt_vmi_memory_resident_bytes")
				j.metricWorker.DeleteQuery(u.GetNamespace(), u.GetName(), "kubevirt_vmi_vcpu_seconds")
			}
			j.msgStore.OnResourceDel(obj, u)
			j.updateResourceStatus()
		},
	})

	go func() {
		for {
			select {
			case <-j.ctx.Done():
				return
			case metricResult := <-j.resultCh:
				j.logger.Info("Metric done", "result", metricResult)
				j.msgStore.OnMetricUpdate(metricResult)
			}
		}
	}()
	go j.metricWorker.Start(time.Second * 20)

}

func (j *MonitorJob) Cancel() {
	j.metricWorker.Stop()
	j.cancel()
}

func (j *MonitorJob) updateResourceStatus() {
	objList, err := j.listRelatedResource()
	if err != nil {
		j.logger.Error(err, "List interest resources failed")
		return
	}

	monitor := &monitorv1alpha1.ResourceMonitor{}
	monitor.SetName(j.monitorName)

	patch := client.MergeFrom(monitor.DeepCopy())
	monitor.Status.Selected = len(objList.Items)

	if err := j.mgrClient.Status().Patch(j.ctx, monitor, patch); err != nil {
		j.logger.Error(err, "Update monitor status failed")
		return
	}
}

func (j *MonitorJob) isRelated(u *unstructured.Unstructured) bool {
	selector := j.MonitorSpec.Selector
	return u != nil &&
		u.GetNamespace() == selector.Namespace &&
		utils.MatchesLabelSelector(u.GetLabels(), selector.Labels)
}
