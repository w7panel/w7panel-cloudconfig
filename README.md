# w7panel-cloudconfig

配置中心独立服务，结构参考 `w7panel-ckm`：

- Go 后端和 controller-runtime 控制器
- `ui/` Vue 3 + Arco 前端
- `charts/w7panel-cloudconfig` Helm Chart

## 功能

- 使用 Cluster-scoped `CloudConfig` CRD 管理全局共享配置，不携带 namespace。
- 支持公共配置和多 version 配置，固定按“继承 < 当前公共 < 当前选中 version”覆盖。
- 配置可继承另一个全局配置的指定 version，当前配置同名项覆盖继承项。
- 配置更新后标记 24 小时内更新状态，并传播到继承者。
- 部署策略支持 Deployment、StatefulSet、DaemonSet 的指定容器。
- 支持环境变量和配置文件两种部署方式。
- 支持手动应用和自动部署。
- 自动部署失败按 1、2、4、8、15 分钟指数退避，配置再次变化时立即重试。

部署策略目标仍是 namespaced 资源，每条策略需要明确设置目标 namespace。

## CRD 作用域变更

`CloudConfig` 已从 Namespaced 改为 Cluster scope。Helm 不会在 upgrade 时自动更新 `crds/` 下已经安装的 CRD；开发期旧数据不迁移，升级前需要删除旧 CloudConfig 资源和旧 CRD，再重新安装 Chart。

## 开发验证

```bash
env GOPATH=/tmp/go-path-cloudconfig GOCACHE=/tmp/go-build-cache-cloudconfig GOMODCACHE=/tmp/go-path-cloudconfig/pkg/mod go test ./...
cd ui && env npm_config_cache=/tmp/npm-cache-cloudconfig npm install && npm run build
helm template test charts/w7panel-cloudconfig
```
