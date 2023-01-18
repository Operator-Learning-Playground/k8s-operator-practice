/*
Copyright 2022.

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
	"fmt"
	"k8s-operator-practice-easy/pkg/helper"
	"k8s-operator-practice-easy/pkg/k8sutils"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	myappv1 "k8s-operator-practice-easy/api/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// RedisReconciler reconciles a Redis object
type RedisController struct {
	client.Client
	Scheme      *runtime.Scheme
	EventRecord record.EventRecorder
}

//+kubebuilder:rbac:groups=myapp.jtthink.com,resources=redis,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=myapp.jtthink.com,resources=redis/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=myapp.jtthink.com,resources=redis/finalizers,verbs=update
//+kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=core,resources=pods,verbs=get;list;watch;create;update;patch;delete

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the Redis object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.11.2/pkg/reconcile
func (r *RedisController) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {

	klog.Info("Request.Namespace: ", req.Namespace, "Request.Name: ", req.Name)
	klog.Info("Reconciling the controller start!")

	redis := &myappv1.Redis{}                    // 自定义对象
	// 取出特定对象
	err := r.Get(ctx, req.NamespacedName, redis)
	if err != nil {
		if errors.IsNotFound(err) {

			klog.Errorf("the resource is not found!", err)
			return ctrl.Result{}, nil
		}

		klog.Errorf("get this resource err: ", err)
		return ctrl.Result{}, err
	}

	// 当前正在删
	if !redis.DeletionTimestamp.IsZero() {
		klog.Info("delete the resource")
		return ctrl.Result{}, r.clearRedis(ctx, redis)
	}

	// 开始创建

	// 取到podNames
	podNames := k8sutils.GetRedisPodNames(redis)
	isEdit := false
	// 遍历创建的pod 如果已经创建则不做处理
	for _, po := range podNames {
		pName, err := k8sutils.CreateRedis(r.Client, redis, po, r.Scheme)
		if err != nil { // 创建pods有错

			klog.Errorf("create the resource", err)
			return ctrl.Result{}, err
		}
		if pName == "" { // 已经存在 redis pod
			continue
		}
		// 与IsExistInFinalizers 方法是一样的，都是通过遍历，找到
		if controllerutil.ContainsFinalizer(redis, pName) {
			continue
		}
		// 把finalizers字段加入
		redis.Finalizers = append(redis.Finalizers, pName)
		isEdit = true
	}

	// 当副本数缩小的时候
	if len(redis.Finalizers) > len(podNames) {
		r.EventRecord.Event(redis, corev1.EventTypeNormal, "Upgrade", "副本数收缩")
		isEdit = true
		// 收缩副本方法
		err := r.rmIfSurplus(ctx, podNames, redis)
		if err != nil {
			return ctrl.Result{}, err
		}
	}

	// TODO: 如果修改redis.yaml中的port，会有个问题就是redis crd已经修改，但是container里面的没有修改。

	// 是否发生了pod创建/收缩，如果没发生，就没必要 update资源
	if isEdit {
		// 触发事件event
		r.EventRecord.Event(redis, corev1.EventTypeNormal, "Updated", "update " + req.Name)
		err := r.Client.Update(ctx, redis) // 更新，一旦触发update，又会进入协调loop中，所以需要有 if pname == "" 的判断。
		if err != nil {
			return ctrl.Result{}, err
		}
		redis.Status.RedisNum = len(redis.Finalizers)
		err = r.Status().Update(ctx, redis)
		if err != nil {
			return ctrl.Result{}, err
		}
	}
	return ctrl.Result{}, nil


}


// SetupWithManager sets up the controller with the Manager.
func (r *RedisController) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&myappv1.Redis{}).
		Watches(&source.Kind{ // 加入监听。
			Type: &v1.Pod{},
		}, handler.Funcs{ // 如果有删除事件，重新加入queue
			DeleteFunc: r.podDeleteHandler,
		}).
		Complete(r)
}

func (r *RedisController) clearRedis(ctx context.Context, redis *myappv1.Redis) error {
	// 因为业务逻辑把 pods的名称都放入 Finalizers字段中才能这样做。
	podList := redis.Finalizers
	for _, podName := range podList {
		// 删除子对象
		err := r.Client.Delete(ctx, &v1.Pod{
			ObjectMeta: metav1.ObjectMeta{Name: podName, Namespace: redis.Namespace},
		})
		if err != nil {
			klog.Info("清除POD异常:", err)
		}
	}
	// finalizers清空，对象才能顺利删除
	redis.Finalizers = []string{}
	return r.Client.Update(ctx, redis)

}

func (r *RedisController) podDeleteHandler(event event.DeleteEvent, limitingInterface workqueue.RateLimitingInterface) {
	klog.Info("被删除的对象名称是", event.Object.GetName())
	for _, ref := range event.Object.GetOwnerReferences() {
		if ref.Kind == myappv1.Kind && ref.APIVersion == myappv1.ApiVersion {
			// 重新入列，这样删除pod后，就会进入调和loop，发现ownerReference还在，会立即创建出新的pod。
			limitingInterface.Add(reconcile.Request{
				NamespacedName: types.NamespacedName{Name: ref.Name,
					Namespace: event.Object.GetNamespace()}})
		}
	}
}

// rmIfSurplus 收缩副本  ['redis0','redis1'] ---> podName ['redis0']
func (r *RedisController) rmIfSurplus(ctx context.Context, poNames []string, redis *myappv1.Redis) error {
	// Finalizers 列表 > podName 列表，开始删除
	for i := 0; i < len(redis.Finalizers)-len(poNames); i++ {
		err := r.Client.Delete(ctx, &v1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name: redis.Finalizers[len(poNames)+i], Namespace: redis.Namespace}, // 注意这个name，需要从poNames后面一个开始删除
		})
		if err != nil {
			return err
		}
	}
	redis.Finalizers = poNames

	return nil

}
