package k8sconfig

import (
	"k8s-operator-practice-easy/pkg/common"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog/v2"
	"log"
	"os"
)

// 集群外部使用
func K8sRestConfig() *rest.Config {
	// 读取配置
	if os.Getenv("Release") == "1" {
		klog.Info("run in the cluster")
		return k8sRestConfigInPod()
	}

	path := common.GetWd()
	config, err := clientcmd.BuildConfigFromFlags("", path+"/resources/config")
	if err != nil {
		log.Fatal(err)
	}
	config.Insecure = true

	return config

}

// 集群内部POD里使用
func k8sRestConfigInPod() *rest.Config {

	config, err := rest.InClusterConfig()
	if err != nil {
		log.Fatal(err)
	}
	return config
}
