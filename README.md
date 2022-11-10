## 使用kubebuilder + kustomize实现k8s简易控制器

### 本项目实现思路：
**通过k8s自定义资源CRD的方式**
1. 利用**kubebuilder** + **kustomize**自动生成、管理应用配置实例。
2. 以deployment形式布署到集群中。
operator = CRD + wehook + controller 
   
### 控制器：
k8s控制器对资源进行监听，如果有add/update/delete事件，会触发Reconcile方法响应。

即为：**Reconcile loop**

### 项目步骤
#### 第一步：kubebuilder初始化
```bigquery
kubebuilder init --domain jtthink.com
kubebuilder create api --group myapp --version v1 --kind Redis
```
### 第二步：创建一个资源对象，用于测试
```bigquery
# test/redis.yaml
apiVersion: myapp.jtthink.com/v1
kind: Redis
metadata:
  name: myredis
spec:
    #这里是自定义的地方
```
接著可以在 /api下查看对应文件里的内容
```bigquery
api
└── v1
    ├── groupversion_info.go
    ├── redis_types.go  # 主要关注对象 
    └── zz_generated.deepcopy.go
```

### 第三步：创建CRD
```bigquery
make install 
# 如果报错，可以直接使用
[root@vm-0-12-centos k8s-easy-operator]# kustomize build config/crd | kubectl apply -f -
customresourcedefinition.apiextensions.k8s.io/redis.myapp.jtthink.com created
```
### 第四步：编写controller逻辑
目录：**controllers/redis_controller.go**
```bigquery
// 在这里编写controller逻辑
func (r *RedisReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	_ = log.FromContext(ctx)

	// TODO(user): your logic here
	
	return ctrl.Result{}, nil
}
```
### 第五步：运行controller
```bigquery
make run 
[root@vm-0-12-centos k8s-easy-operator]# make run
/root/k8s-easy-operator/bin/controller-gen rbac:roleName=manager-role crd webhook paths="./..." output:crd:artifacts:config=config/crd/bases
/root/k8s-easy-operator/bin/controller-gen object:headerFile="hack/boilerplate.go.txt" paths="./..."
go fmt ./...
go vet ./...
go run ./main.go

```