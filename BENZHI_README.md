# FeatureStore 特征存储服务

FeatureStore 是一个自建的特征存储基础设施示例，按实体键组织特征表，
支持特征版本发布与回滚、在线读取缓存、批量导入合并、特征回填计算、
在线/离线一致性校验和 TTL 过期清理。服务同时提供一个浏览器特征浏览页面。

## 构建

```bash
go build -mod=vendor ./...
go vet -mod=vendor ./...
go test -mod=vendor ./...
```

## 启动

```bash
go run -mod=vendor ./cmd/featurestore -addr 127.0.0.1:8080 -shards 16
```

启动后访问：

- 健康检查：`http://127.0.0.1:8080/api/health`
- 特征浏览页面：`http://127.0.0.1:8080/web/browse.html`

## Docker 多架构构建

```bash
bash build_benzhi_docker.sh featurestore linux/amd64
bash build_benzhi_docker.sh featurestore linux/arm64
```

容器内支持 `go build -mod=vendor ./...`、`go test -mod=vendor ./...`、
`go vet -mod=vendor ./...` 与上述启动命令。
