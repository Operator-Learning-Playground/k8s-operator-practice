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
	"k8s-operator-practice-easy/helper"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	myappv1 "k8s-operator-practice-easy/api/v1"
)

// RedisReconciler reconciles a Redis object
type RedisReconciler struct {
	client.Client
	Scheme *runtime.Scheme
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
func (r *RedisReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	_ = log.FromContext(ctx)

	// TODO(user): your logic here
	redis := &myappv1.Redis{}
	err := r.Get(ctx, req.NamespacedName, redis)
	if err != nil {
		fmt.Println(err)
	} else {
		// 当前正在删
		if !redis.DeletionTimestamp.IsZero() {
			return ctrl.Result{}, r.clearRedis(ctx, redis)
		}
		//开始创建 拟 创建的pod
		// 取到podNames
		podNames := helper.GetRedisPodNames(redis)
		isEdit := false
		// 遍历 拟 创建的pod  挨个创建，如果已经创建则不做处理
		for _, po := range podNames {
			pname, err := helper.CreateRedis(r.Client, redis, po, r.Scheme)
			if err != nil { // 创建pods有错
				return ctrl.Result{}, err
			}
			if pname == "" { // 已经存在 redis pod
				continue
			}
			// 与IsExistInFinalizers 方法是一样的，都是通过遍历，找到
			if controllerutil.ContainsFinalizer(redis, pname) {
				continue
			}
			// 把finalizers字段加入
			redis.Finalizers = append(redis.Finalizers, pname)
			isEdit = true
		}
		//收缩 福本
		if len(redis.Finalizers) > len(podNames) {
			r.EventRecord.Event(redis, corev1.EventTypeNormal, "Upgrade", "副本收缩")
			isEdit = true
			err := r.rmIfSurplus(ctx, podNames, redis)
			if err != nil {
				return ctrl.Result{}, err
			}
		}

		// TODO: 如果修改redis.yaml中的port，会有个问题就是redis crd已经修改，但是container里面的没有修改。

		// 是否发生了pod创建/收缩，如果没发生，就没必要 update资源
		if isEdit {
			// 触发事件event
			r.EventRecord.Event(redis, corev1.EventTypeNormal, "Updated", "更新myredis")
			err := r.Client.Update(ctx, redis)	// 更新，一旦触发update，又会进入协调loop中，所以需要有 if pname == "" 的判断。
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


	return ctrl.Result{}, nil
}

//// SetupWithManager sets up the controller with the Manager.
//func (r *RedisReconciler) SetupWithManager(mgr ctrl.Manager) error {
//	return ctrl.NewControllerManagedBy(mgr).
//		For(&myappv1.Redis{}).
//		Complete(r)
//}

// SetupWithManager sets up the controller with the Manager.
func (r *RedisReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&myappv1.Redis{}).
		Watches(&source.Kind{	// 加入监听。
			Type: &v1.Pod{},
		}, handler.Funcs{
			DeleteFunc: r.podDeleteHandler,
		}).
		Complete(r)
}


func (r *RedisReconciler) clearRedis(ctx context.Context, redis *myappv1.Redis) error {
	podList := redis.Finalizers
	for _, podName := range podList {
		err := r.Client.Delete(ctx, &v1.Pod{
			ObjectMeta: metav1.ObjectMeta{Name: podName, Namespace: redis.Namespace},
		})
		if err != nil {
			fmt.Println("清除POD异常:", err)
		}
	}
	redis.Finalizers = []string{} // finalizers清空
	return r.Client.Update(ctx, redis)

}

func (r *RedisReconciler) podDeleteHandler(event event.DeleteEvent, limitingInterface workqueue.RateLimitingInterface) {
	fmt.Println("被删除的对象名称是", event.Object.GetName())
	for _, ref := range event.Object.GetOwnerReferences() {
		if ref.Kind == "Redis" && ref.APIVersion == "myapp.jtthink.com/v1" {
			// 重新入列，这样删除pod后，就会进入调和loop，发现owerReference还在，会立即创建出新的pod。
			limitingInterface.Add(reconcile.Request{
				NamespacedName: types.NamespacedName{Name: ref.Name,
					Namespace: event.Object.GetNamespace()}})
		}
	}
}

// rmIfSurplus 收缩副本  ['redis0','redis1']   ---> podName ['redis0']
func (r *RedisReconciler) rmIfSurplus(ctx context.Context, poNames []string, redis *myappv1.Redis) error {
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

