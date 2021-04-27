/*
Copyright 2021.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controllers

import (
	"context"
	"time"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	monitorv1alpha1 "github.com/fusion-app/gateway/api/v1alpha1"
	"github.com/fusion-app/gateway/pkg/job"
)

const monitorFinalizer = "monitor.fusion-app.io/finalizer"

// ResourceMonitorReconciler reconciles a ResourceMonitor object
type ResourceMonitorReconciler struct {
	client.Client
	Log        logr.Logger
	Scheme     *runtime.Scheme
	jobManager *job.SyncJobManager
}

func (r *ResourceMonitorReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	monitor := &monitorv1alpha1.ResourceMonitor{}
	err := r.Get(ctx, req.NamespacedName, monitor)
	if err != nil {
		if errors.IsNotFound(err) {
			r.Log.Info("ResourceMonitor not found, ignore")
			return ctrl.Result{}, nil
		}
		return ctrl.Result{
			Requeue:      true,
			RequeueAfter: time.Second * 3,
		}, err
	}

	isMarkedToBeDeleted := monitor.GetDeletionTimestamp() != nil
	if isMarkedToBeDeleted {
		if controllerutil.ContainsFinalizer(monitor, monitorFinalizer) {
			r.jobManager.CleanJob(monitor)
			controllerutil.RemoveFinalizer(monitor, monitorFinalizer)
			err := r.Update(context.TODO(), monitor)
			if err != nil {
				return ctrl.Result{}, err
			}
		}
		return ctrl.Result{}, nil
	}

	if !controllerutil.ContainsFinalizer(monitor, monitorFinalizer) {
		controllerutil.AddFinalizer(monitor, monitorFinalizer)
		err = r.Update(context.TODO(), monitor)
		if err != nil {
			return ctrl.Result{}, err
		}
	}

	r.jobManager.NewJobOrExist(monitor)
	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *ResourceMonitorReconciler) SetupWithManager(mgr ctrl.Manager) error {
	r.jobManager = job.NewSyncJobManager(mgr.GetCache(), mgr.GetClient())
	return ctrl.NewControllerManagedBy(mgr).
		For(&monitorv1alpha1.ResourceMonitor{}).
		WithEventFilter(predicate.GenerationChangedPredicate{}).
		Complete(r)
}
