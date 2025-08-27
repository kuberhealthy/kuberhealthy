package controller

import (
	"context"
	"time"

	khcrdsv2 "github.com/kuberhealthy/crds/api/v2"
	"github.com/kuberhealthy/kuberhealthy/v3/internal/kuberhealthy"
	log "github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/dynamic"
	dynamicinformer "k8s.io/client-go/dynamic/dynamicinformer"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/retry"
	"k8s.io/client-go/util/workqueue"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

// KHCheckController watches KuberhealthyCheck resources and reconciles them.
type KHCheckController struct {
	client.Client
	scheme       *runtime.Scheme
	Kuberhealthy *kuberhealthy.Kuberhealthy
	informer     cache.SharedIndexInformer
	queue        workqueue.RateLimitingInterface
}

// newKHCheckController creates a KHCheckController with an informer watching KuberhealthyCheck resources.
func newKHCheckController(cfg *rest.Config, cl client.Client, scheme *runtime.Scheme) (*KHCheckController, error) {
	dyn, err := dynamic.NewForConfig(cfg)
	if err != nil {
		return nil, err
	}

	gvr := schema.GroupVersionResource{Group: "kuberhealthy.github.io", Version: "v2", Resource: "kuberhealthychecks"}
	factory := dynamicinformer.NewFilteredDynamicSharedInformerFactory(dyn, 0, metav1.NamespaceAll, nil)
	inf := factory.ForResource(gvr).Informer()

	c := &KHCheckController{
		Client:   cl,
		scheme:   scheme,
		informer: inf,
		queue:    workqueue.NewRateLimitingQueue(workqueue.DefaultControllerRateLimiter()),
	}

	inf.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    c.enqueue,
		UpdateFunc: func(_, newObj interface{}) { c.enqueue(newObj) },
		DeleteFunc: c.enqueue,
	})

	c.setupWithManager()

	return c, nil
}

// Start begins running the informer and worker loops.
func (c *KHCheckController) Start(ctx context.Context) {
	go c.informer.Run(ctx.Done())
	if !cache.WaitForCacheSync(ctx.Done(), c.informer.HasSynced) {
		log.Error("controller: cache sync failed")
		return
	}
	go wait.UntilWithContext(ctx, c.runWorker, time.Second)
}

func (c *KHCheckController) runWorker(ctx context.Context) {
	for c.processNextItem(ctx) {
	}
}

func (c *KHCheckController) processNextItem(ctx context.Context) bool {
	obj, shutdown := c.queue.Get()
	if shutdown {
		return false
	}
	defer c.queue.Done(obj)

	req, ok := obj.(ctrl.Request)
	if !ok {
		c.queue.Forget(obj)
		return true
	}

	if _, err := c.Reconcile(ctx, req); err != nil {
		c.queue.AddRateLimited(req)
	} else {
		c.queue.Forget(obj)
	}
	return true
}

func (c *KHCheckController) enqueue(obj interface{}) {
	if obj == nil {
		return
	}
	if tombstone, ok := obj.(cache.DeletedFinalStateUnknown); ok {
		obj = tombstone.Obj
	}
	if m, ok := obj.(metav1.Object); ok {
		req := ctrl.Request{NamespacedName: types.NamespacedName{Name: m.GetName(), Namespace: m.GetNamespace()}}
		c.queue.Add(req)
	}
}

// sanitizeCheck resets metadata fields that should not be sent back to the API server.
func sanitizeCheck(check *khcrdsv2.KuberhealthyCheck) {
	check.ObjectMeta.ManagedFields = nil
	check.ObjectMeta.UID = ""
}

// Reconcile mirrors KuberhealthyCheckReconciler.Reconcile.
func (c *KHCheckController) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log.Debugln("khcheckcontroller: Reconcile")

	var check khcrdsv2.KuberhealthyCheck
	if err := c.Client.Get(ctx, req.NamespacedName, &check); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	finalizer := "kuberhealthy.github.io/finalizer"

	if !check.ObjectMeta.DeletionTimestamp.IsZero() {
		if controllerutil.ContainsFinalizer(&check, finalizer) {
			log.Infoln("khcheckcontroller: FINALIZER DELETE event detected for:", req.Namespace+"/"+req.Name)
			retryErr := retry.RetryOnConflict(retry.DefaultRetry, func() error {
				if err := c.Client.Get(ctx, req.NamespacedName, &check); err != nil {
					return err
				}
				controllerutil.RemoveFinalizer(&check, finalizer)
				sanitizeCheck(&check)
				return c.Client.Update(ctx, &check)
			})
			if retryErr != nil {
				return ctrl.Result{}, retryErr
			}
		}
		return ctrl.Result{}, nil
	}

	if !controllerutil.ContainsFinalizer(&check, finalizer) {
		retryErr := retry.RetryOnConflict(retry.DefaultRetry, func() error {
			if err := c.Client.Get(ctx, req.NamespacedName, &check); err != nil {
				return err
			}
			controllerutil.AddFinalizer(&check, finalizer)
			sanitizeCheck(&check)
			return c.Client.Update(ctx, &check)
		})
		if retryErr != nil {
			return ctrl.Result{}, retryErr
		}
	}

	if err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		if err := c.Client.Get(ctx, req.NamespacedName, &check); err != nil {
			return err
		}
		sanitizeCheck(&check)
		return c.Client.Status().Update(ctx, &check)
	}); err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}
