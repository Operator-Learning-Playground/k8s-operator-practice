package k8sutils

import (
	"context"
	"fmt"
	v1 "k8s-operator-practice-easy/api/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

// 判断redis pod 是否 能获取
func IsExistPod(podName string, redis *v1.Redis, client client.Client) bool {
	err := client.Get(context.Background(),
		types.NamespacedName{Namespace: redis.Namespace, Name: podName}, &corev1.Pod{})
	if err != nil {
		klog.Errorf("get pod: %s err: %s", podName, err)
		return false
	}
	return true

}

// GetRedisPodNames 根据副本的数量，生成不同编号的名称，避免命名冲突。(不然生成五个，名都一样会有冲突)
func GetRedisPodNames(redisConfig *v1.Redis) []string {

	podNames := make([]string, redisConfig.Spec.Num)
	// ex: redis-0 redis-1
	for i := 0; i < redisConfig.Spec.Num; i++ {
		podNames[i] = fmt.Sprintf("%s-%d", redisConfig.Name, i)
	}
	klog.Info("pod Names:", podNames)
	return podNames

}

// force 参数 强制创建，不判断是否在finalizers中 存在
// CreateRedis 传入client podName，创建出一个pod。
func CreateRedis(client client.Client,
	redisConfig *v1.Redis, podName string, schema *runtime.Scheme) (string, error) {

	//if  IsExistInFinalizers(podName, redisConfig) {
	//	return "", nil
	//}

	// 判断要创建的pod是否存在，存在就不处理
	if IsExistPod(podName, redisConfig, client) {
		return "", nil
	}

	// 创建pod
	newPod := &corev1.Pod{}
	newPod.Name = podName
	newPod.Namespace = redisConfig.Namespace

	// 镜像
	newPod.Spec.Containers = []corev1.Container{
		{
			//Name:            redisConfig.Name,
			Name:            podName,
			Image:           "redis:5-alpine",
			ImagePullPolicy: corev1.PullIfNotPresent,
			Ports: []corev1.ContainerPort{
				{
					ContainerPort: int32(redisConfig.Spec.Port),
				},
			},
		},
	}

	// own reference 设定此资源的owner，当owner被删除，对应的pod也会被删除
	err := controllerutil.SetControllerReference(redisConfig, newPod, schema)
	if err != nil {
		return "", err
	}

	// 创建pod
	err = client.Create(context.Background(), newPod)
	if err != nil {
		klog.Errorf("create pod: %s err: %s", podName, err)
		return podName, err
	}

	return podName, nil
}
