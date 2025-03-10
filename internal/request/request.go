package request

import (
	"context"
	"fmt"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

type BaseRequest[T client.Object] struct {
	Cr      T
	Client  client.Client
	Context context.Context
	Scheme  *runtime.Scheme
}

func (r *BaseRequest[T]) Namespace() string {
	return r.Cr.GetNamespace()
}

func (r *BaseRequest[T]) Name() string {
	return r.Cr.GetName()
}

func (r *BaseRequest[T]) Instance() string {
	maxNameLength := validation.LabelValueMaxLength - len(r.Cr.GetUID()) - len("-")
	crName := strings.TrimRight(r.Name()[:min(len(r.Name()), maxNameLength)], "-")
	return fmt.Sprintf("%s-%s", crName, r.Cr.GetUID())
}

// sets the CR as the owner of the object
// object will be garbage collected when CR is deleted
// also, if the object type is being watched by the CR controller (see SetupWithManager function),
// the reconcilation loop will be triggered if the object is updated or deleted
func (r *BaseRequest[T]) SetControllerReference(obj metav1.Object) {
	ctrl.SetControllerReference(r.Cr, obj, r.Scheme)
}

func (r *BaseRequest[T]) CreateOrUpdate(obj client.Object, fn controllerutil.MutateFn) (controllerutil.OperationResult, error) {
	return controllerutil.CreateOrUpdate(r.Context, r.Client, obj, fn)
}

func (r *BaseRequest[T]) UpdateStatus() error {
	return r.Client.Status().Update(r.Context, r.Cr)
}

func (r *BaseRequest[T]) Update(obj client.Object) error {
	return r.Client.Update(r.Context, obj)
}

func (r *BaseRequest[T]) Patch(obj client.Object, fn controllerutil.MutateFn) error {
	before := obj.DeepCopyObject()
	patch := client.MergeFrom(before.(client.Object))
	if err := fn(); err != nil {
		return err
	}

	return r.Client.Patch(r.Context, obj, patch)
}

func (r *BaseRequest[T]) DeleteIfExists(objs ...client.Object) error {
	for _, obj := range objs {
		if err := client.IgnoreNotFound(r.Client.Delete(r.Context, obj)); err != nil {
			return err
		}
	}
	return nil
}

func (r *BaseRequest[T]) List(list client.ObjectList, ops ...client.ListOption) error {
	return r.Client.List(r.Context, list, ops...)
}
