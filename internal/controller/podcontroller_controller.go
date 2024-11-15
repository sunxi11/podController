/*
Copyright 2024.

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

package controller

import (
	"context"
	"fmt"
	"github.com/sunxi11/podController/internal/types"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/informers"
	v1 "k8s.io/client-go/informers/core/v1"
	v1list "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	kubebincomv1 "github.com/sunxi11/podController/api/v1"
)

func NewPodcontrollerReconciler(client client.Client, scheme *runtime.Scheme, informer v1.PodInformer, dpuinformer informers.GenericInformer, ctx context.Context) *PodcontrollerReconciler {
	controller := &PodcontrollerReconciler{
		Client:      client,
		Scheme:      scheme,
		PodLister:   informer.Lister(),
		DpuSfLister: dpuinformer.Lister(),
		PodsSynced:  informer.Informer().HasSynced,
		DpuSynced:   dpuinformer.Informer().HasSynced,
		ctx:         ctx,
	}

	return controller
}

// PodcontrollerReconciler reconciles a Podcontroller object
type PodcontrollerReconciler struct {
	client.Client
	Scheme      *runtime.Scheme
	PodLister   v1list.PodLister
	DpuSfLister cache.GenericLister
	PodsSynced  cache.InformerSynced
	DpuSynced   cache.InformerSynced
	ctx         context.Context
	//podLister   v1listers.
}

func (p *PodcontrollerReconciler) SyncResouce() error {
	if ok := cache.WaitForCacheSync(p.ctx.Done(), p.PodsSynced); !ok {
		return fmt.Errorf("failed to wait for caches to sync")
	}

	if ok := cache.WaitForCacheSync(p.ctx.Done(), p.DpuSynced); !ok {
		return fmt.Errorf("failed to wait for caches to sync")
	}

	fmt.Printf("SyncResouce complete\n")

	<-p.ctx.Done()

	return nil
}

//+kubebuilder:rbac:groups=kubebin.com.my.domain,resources=podcontrollers,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=kubebin.com.my.domain,resources=podcontrollers/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=kubebin.com.my.domain,resources=podcontrollers/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the Podcontroller object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.15.0/pkg/reconcile
func (r *PodcontrollerReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	if !r.PodsSynced() {
		logger.Info("Pod informer is not yet synced")
		return ctrl.Result{Requeue: true}, nil
	}

	if !r.DpuSynced() {
		logger.Info("Dpu informer is not yet synced")
		return ctrl.Result{Requeue: true}, nil
	}

	podcontroller := &kubebincomv1.Podcontroller{}
	err := r.Get(ctx, req.NamespacedName, podcontroller)
	if err != nil {
		if errors.IsNotFound(err) {
			logger.Info("podcontroller resource not found. Ignoring.")
			return ctrl.Result{}, nil
		}
		logger.Error(err, "Failed to get podcontroller resource")
		return ctrl.Result{}, err
	}

	targetPodName := podcontroller.Spec.PodTargetRef
	targetSfName := podcontroller.Spec.NicTargetRef

	logger.Info("get target podName", "targetPodName", targetPodName)
	logger.Info("get target SfName", "targetSfName", targetSfName)

	sfobjList, err := r.DpuSfLister.ByNamespace(podcontroller.Namespace).List(labels.Everything())
	if err != nil {
		logger.Error(err, "Failed to list sf resource")
		return ctrl.Result{}, err
	}

	sfNameList := make([]string, 0)
	for _, item := range sfobjList {
		meteobj, err := meta.Accessor(item)
		if err != nil {
			logger.Info("Failed to get sf resource")
			continue
		}
		sfNameList = append(sfNameList, meteobj.GetName())
	}

	logger.Info("get sfList", "sfList", sfNameList)

	pod, err := r.PodLister.Pods(podcontroller.Namespace).Get(targetPodName)
	if err != nil {
		if errors.IsNotFound(err) {
			logger.Info("pod resource not found. Ignoring.")
			return ctrl.Result{}, nil
		}
	}

	podCopy := pod.DeepCopy()
	podCopy.ResourceVersion = ""

	//check pod
	if curSf, ok := pod.Annotations[types.Annotations_DpuSf_Network]; !ok {

		podCopy.Annotations[types.Annotations_DpuSf_Network] = targetSfName
		podCopy.Name = pod.Name + targetSfName
		podCopy.Spec.NodeName = "zjlab105-poweredge-r740"

		err := r.Create(ctx, podCopy)
		if err != nil {
			logger.Error(err, "Failed to create pod resource")
			return ctrl.Result{}, err
		}

		err = r.Delete(ctx, pod)
		if err != nil {
			logger.Error(err, "Failed to delete pod resource")
			return ctrl.Result{}, err
		}

		return ctrl.Result{}, err

	} else {
		//检查 Status
		if curSf != targetSfName {

			podCopy.Annotations[types.Annotations_DpuSf_Network] = targetSfName
			podCopy.Name = pod.Name + targetSfName
			err := r.Create(ctx, podCopy)
			if err != nil {
				logger.Error(err, "Failed to create pod resource")
				return ctrl.Result{}, err
			}

			err = r.Delete(ctx, pod)
			if err != nil {
				logger.Error(err, "Failed to delete pod resource")
				return ctrl.Result{}, err
			}
		}

		logger.Info("pod resource is right. Ignoring.")

	}

	// TODO(user): your logic here

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *PodcontrollerReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&kubebincomv1.Podcontroller{}).
		Complete(r)
}
