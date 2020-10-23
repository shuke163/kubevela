/*


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

package autoscalers

import (
	"context"
	"flag"
	"fmt"
	"path/filepath"
	"reflect"
	"time"

	cpv1alpha1 "github.com/crossplane/crossplane-runtime/apis/core/v1alpha1"
	"github.com/crossplane/crossplane-runtime/pkg/event"
	"github.com/crossplane/oam-kubernetes-runtime/pkg/oam/util"
	oamutil "github.com/crossplane/oam-kubernetes-runtime/pkg/oam/util"
	"github.com/go-logr/logr"
	kedav1alpha1 "github.com/kedacore/keda/api/v1alpha1"
	"github.com/oam-dev/kubevela/api/v1alpha1"
	"github.com/oam-dev/kubevela/pkg/controller/common"
	"github.com/pkg/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	restclient "k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
	"k8s.io/utils/pointer"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const (
	SpecWarningTargetWorkloadNotSet                = "Spec.targetWorkload is not set"
	SpecWarningStartAtTimeFormat                   = "startAt is not in the right format, which should be like `12:01`"
	SpecWarningStartAtTimeRequired                 = "spec.triggers.condition.startAt: Required value"
	SpecWarningDurationTimeRequired                = "spec.triggers.condition.duration: Required value"
	SpecWarningReplicasRequired                    = "spec.triggers.condition.replicas: Required value"
	SpecWarningDurationTimeNotInRightFormat        = "spec.triggers.condition.duration: not in the right format"
	SpecWarningSumOfStartAndDurationMoreThan24Hour = "the sum of the start hour and the duration hour has to be less than 24 hours."
)

var (
	scaledObjectKind       = reflect.TypeOf(kedav1alpha1.ScaledObject{}).Name()
	scaledObjectAPIVersion = "keda.k8s.io/v1alpha1"
)

// ReconcileWaitResult is the time to wait between reconciliation.
var ReconcileWaitResult = reconcile.Result{RequeueAfter: 30 * time.Second}

// AutoscalerReconciler reconciles a Autoscaler object
type AutoscalerReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
	record event.Recorder
	config *restclient.Config
	ctx    context.Context
}

// +kubebuilder:rbac:groups=standard.oam.dev,resources=autoscalers,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=standard.oam.dev,resources=autoscalers/status,verbs=get;update;patch
func (r *AutoscalerReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	log := r.Log.WithValues("autoscaler", req.NamespacedName)
	log.Info("Reconciling Autoscaler...")

	var scaler v1alpha1.Autoscaler
	if err := r.Get(r.ctx, req.NamespacedName, &scaler); err != nil {
		return ReconcileWaitResult, client.IgnoreNotFound(err)
	}
	log.Info("Retrieved trait Autoscaler", "APIVersion", scaler.APIVersion, "Kind", scaler.Kind)

	// find the resource object to record the event to, default is the parent appConfig.
	eventObj, err := util.LocateParentAppConfig(r.ctx, r.Client, &scaler)
	if err != nil {
		log.Error(err, "Failed to find the parent resource", "Autoscaler", scaler.Name)
		return util.ReconcileWaitResult, util.PatchCondition(r.ctx, r, &scaler,
			cpv1alpha1.ReconcileError(fmt.Errorf(util.ErrLocateAppConfig)))
	}
	if eventObj == nil {
		// fallback to workload itself
		log.Info("There is no parent resource", "Autoscaler", scaler.Name)
		eventObj = &scaler
	}

	// Fetch the instance to which the trait refers to
	workload, err := oamutil.FetchWorkload(r.ctx, r, log, &scaler)
	if err != nil {
		log.Error(err, "Error while fetching the workload", "workload reference",
			scaler.GetWorkloadReference())
		r.record.Event(&scaler, event.Warning(common.ErrLocatingWorkload, err))
		return oamutil.ReconcileWaitResult,
			oamutil.PatchCondition(r.ctx, r, &scaler,
				cpv1alpha1.ReconcileError(errors.Wrap(err, common.ErrLocatingWorkload)))
	}

	ownerReference := metav1.OwnerReference{
		APIVersion:         scaler.APIVersion,
		Kind:               scaler.Kind,
		UID:                scaler.GetUID(),
		Name:               scaler.Name,
		Controller:         pointer.BoolPtr(true),
		BlockOwnerDeletion: pointer.BoolPtr(true),
	}

	// Fetch the child resources list from the corresponding workload
	resources, err := util.FetchWorkloadChildResources(r.ctx, log, r, workload)
	if err != nil {
		log.Error(err, "Error while fetching the workload child resources", "workload", workload.UnstructuredContent())
		r.record.Event(eventObj, event.Warning(util.ErrFetchChildResources, err))
		return util.ReconcileWaitResult, util.PatchCondition(r.ctx, r, &scaler,
			cpv1alpha1.ReconcileError(fmt.Errorf(util.ErrFetchChildResources)))
	}
	resources = append(resources, workload)

	targetWorkloadSetFlag := false
	for _, res := range resources {
		resPatch := client.MergeFrom(res.DeepCopyObject())
		refs := res.GetOwnerReferences()
		for i, r := range refs {
			if *r.Controller {
				refs[i].Controller = pointer.BoolPtr(false)
			}
		}
		refs = append(refs, ownerReference)
		res.SetOwnerReferences(refs)
		if err := r.Patch(r.ctx, res, resPatch, client.FieldOwner(scaler.GetUID())); err != nil {
			log.Error(err, "Failed to set ownerReference for child resource")
			return util.ReconcileWaitResult,
				util.PatchCondition(r.ctx, r, &scaler, cpv1alpha1.ReconcileError(
					errors.Wrap(err, "Failed to set ownerReference for child resource")))
		}
		if !targetWorkloadSetFlag && (res.GetKind() == "Deployment" || res.GetKind() == "StatefulSet") {
			scaler.Spec.TargetWorkload = v1alpha1.TargetWorkload{
				APIVersion: res.GetAPIVersion(),
				Kind:       res.GetKind(),
				Name:       res.GetName(),
			}
			targetWorkloadSetFlag = true
		}
	}

	// if there is no child resource or no child resource kind is deployment or statefuset, set the workload as target workload
	if len(resources) == 1 && !targetWorkloadSetFlag {
		scaler.Spec.TargetWorkload = v1alpha1.TargetWorkload{
			APIVersion: workload.GetAPIVersion(),
			Kind:       workload.GetKind(),
			Name:       workload.GetName(),
		}
	}

	namespace := req.NamespacedName.Namespace
	if err := r.scaleByKEDA(scaler, namespace, log); err != nil {
		return ReconcileWaitResult, err
	}

	return ctrl.Result{}, nil
}

func (r *AutoscalerReconciler) SetupWithManager(mgr ctrl.Manager) error {
	if err := r.buildConfig(); err != nil {
		return err
	}
	r.ctx = context.Background()
	r.record = event.NewAPIRecorder(mgr.GetEventRecorderFor("Autoscaler")).
		WithAnnotations("controller", "Autoscaler")
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.Autoscaler{}).
		Complete(r)
}

func (r *AutoscalerReconciler) buildConfig() error {
	var kubeConfig *string
	if home := homedir.HomeDir(); home != "" {
		kubeConfig = flag.String("kubeConfig", filepath.Join(home, ".kube", "config"), "kubeConfig file")
	}
	flag.Parse()
	config, err := clientcmd.BuildConfigFromFlags("", *kubeConfig)
	if err != nil {
		return err
	}
	r.config = config
	return nil
}

// Setup adds a controller that reconciles MetricsTrait.
func Setup(mgr ctrl.Manager) error {
	r := AutoscalerReconciler{
		Client: mgr.GetClient(),
		Log:    ctrl.Log.WithName("Autoscaler"),
		Scheme: mgr.GetScheme(),
	}
	return r.SetupWithManager(mgr)
}