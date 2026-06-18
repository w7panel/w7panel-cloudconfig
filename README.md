# w7panel-cloudconfig

配置中心独立插件应用，结构参考 `w7panel-ckm`：

- Go 后端和 controller-runtime 控制器
- `ui/` Vue 3 + Arco 前端
- `charts/w7panel-cloudconfig` Helm Chart 和 MicroApp 注册

## 功能

- 管理共享配置项，支持公共配置和多 version 配置。
- 配置可继承另一个配置的指定 version，当前配置同名项覆盖继承项。
- 配置更新后标记 24 小时内更新状态，并传播到继承者。
- 部署策略支持 Deployment、StatefulSet、DaemonSet 的指定容器。
- 支持环境变量和配置文件两种部署方式。
- 支持手动应用和自动部署。

## 开发验证

```bash
env GOPATH=/tmp/go-path-cloudconfig GOCACHE=/tmp/go-build-cache-cloudconfig GOMODCACHE=/tmp/go-path-cloudconfig/pkg/mod go test ./...
cd ui && env npm_config_cache=/tmp/npm-cache-cloudconfig npm install && npm run build
helm template test charts/w7panel-cloudconfig
```
