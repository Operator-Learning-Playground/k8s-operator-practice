package k8sutils

import v1 "k8s-operator-practice-easy/api/v1"

// IsExistInFinalizers 之前业务逻辑：当创建某一pod时，把名字加入Finalizers字段中。
// 可代表集群中有哪些pod。
func IsExistInFinalizers(podName string, redis *v1.Redis) bool {
	for _, po := range redis.Finalizers {
		if podName == po {
			return true
		}
	}
	return false
}
