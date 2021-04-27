package job

import (
	"context"
	"encoding/json"

	"github.com/go-logr/logr"
	"github.com/wI2L/jsondiff"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	toolscache "k8s.io/client-go/tools/cache"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"

	monitorv1alpha1 "github.com/fusion-app/gateway/api/v1alpha1"
	"github.com/fusion-app/gateway/pkg/message"
	"github.com/fusion-app/gateway/pkg/utils"
)

type MonitorJob struct {
	MonitorSpec *monitorv1alpha1.ResourceMonitorSpec

	monitorGVK        schema.GroupVersionKind
	monitorName       string
	interestGVKSchema schema.GroupVersionKind

	cancelFunc    context.CancelFunc
	cancelContext context.Context

	msgHandler message.MsgHandler
	logger     logr.Logger
	mgrCache   cache.Cache
	mgrClient  client.Client
}

func NewMonitorJob(ref *monitorv1alpha1.ResourceMonitor, logger logr.Logger, mgrCache cache.Cache, mgrClient client.Client) *MonitorJob {
	cancelContext, cancelFunc := context.WithCancel(context.TODO())
	interestGVK := ref.Spec.Selector.GVK
	return &MonitorJob{
		MonitorSpec: ref.Spec.DeepCopy(),
		monitorGVK:  ref.GroupVersionKind(),
		monitorName: ref.GetName(),
		interestGVKSchema: schema.GroupVersionKind{
			Group:   interestGVK.Group,
			Version: interestGVK.Version,
			Kind:    interestGVK.Kind,
		},
		cancelContext: cancelContext,
		cancelFunc:    cancelFunc,
		msgHandler:    message.NewMsgHandlerOrExist(ref.Spec.MsgBackendSpec),
		logger:        logger,
		mgrCache:      mgrCache,
		mgrClient:     mgrClient,
	}
}

func (j *MonitorJob) listRelatedResource() (*unstructured.UnstructuredList, error) {
	objList := &unstructured.UnstructuredList{}
	objList.SetAPIVersion(j.interestGVKSchema.GroupVersion().String())
	objList.SetKind(j.interestGVKSchema.Kind + "List")
	listOpt := client.MatchingLabels{}
	for k, v := range j.MonitorSpec.Selector.Labels {
		listOpt[k] = v
	}
	err := j.mgrClient.List(j.cancelContext, objList, listOpt)
	if err != nil {
		return nil, err
	}
	return objList, nil
}

func (j *MonitorJob) Start() {
	j.updateResourceStatus()
	informer, err := j.mgrCache.GetInformerForKind(context.TODO(), j.interestGVKSchema)
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

			objRawData, err := json.Marshal(obj)
			if err != nil {
				return
			}
			msg := &message.Message{
				Op: message.NewResource,
				Meta: &message.ResourceMeta{
					Namespace: u.GetNamespace(),
					Name:      u.GetName(),
				},
				Data: objRawData,
			}
			_ = j.msgHandler.Publish(msg)
			j.updateResourceStatus()
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			oldU := utils.ToUnstructured(oldObj)
			newU := utils.ToUnstructured(newObj)
			if !j.isRelated(oldU) && !j.isRelated(newU) {
				return
			}

			patch, err := jsondiff.Compare(oldObj, newObj)
			if err != nil {
				return
			}
			patchData, err := json.MarshalIndent(patch, "", "    ")
			if err != nil {
				return
			}
			msg := &message.Message{
				Op: message.UpdateResource,
				Meta: &message.ResourceMeta{
					Namespace: newU.GetNamespace(),
					Name:      newU.GetName(),
				},
				Data: patchData,
			}
			_ = j.msgHandler.Publish(msg)
		},
		DeleteFunc: func(obj interface{}) {
			u := utils.ToUnstructured(obj)
			if !j.isRelated(u) {
				return
			}

			objRawData, err := json.Marshal(obj.(ctrl.ObjectMeta))
			if err != nil {
				return
			}
			msg := &message.Message{
				Op: message.DelResource,
				Meta: &message.ResourceMeta{
					Namespace: u.GetNamespace(),
					Name:      u.GetName(),
				},
				Data: objRawData,
			}
			_ = j.msgHandler.Publish(msg)
			j.updateResourceStatus()
		},
	})
}

func (j *MonitorJob) Cancel() {
	j.cancelFunc()
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

	if err := j.mgrClient.Status().Patch(j.cancelContext, monitor, patch); err != nil {
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
