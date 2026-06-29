[OPEN] Debug Session: email-env-missing

## 症状
- 重建容器后，邮件推送仍报错：`邮件推送配置不完整： 缺少 PUSH_EMAIL_SMTP_HOST`

## 期望
- 后端容器启动后，`PUSH_EMAIL_SMTP_HOST` 等邮件配置已存在于运行环境中。

## 初始假设
- 假设 1：`deploy_ubuntu.sh` 生成的 `backend.env` 文件中没有写入 `PUSH_EMAIL_SMTP_HOST`。
- 假设 2：`backend.env` 已生成正确，但 `docker-compose.yml` 没有把该文件注入到后端容器。
- 假设 3：容器重建流程没有重新生成或重新挂载最新的 `backend.env`，仍在使用旧文件。
- 假设 4：后端服务读取的并不是容器环境变量，而是其他路径下的 `.env` / 默认值。
- 假设 5：报错来自另一套运行中的后端实例，而不是刚重建的容器实例。

## 待收集证据
- `docker-compose.yml` 中 backend 的 `env_file` / volume / command 配置
- 运行时生成的 `backend.env` 内容
- 当前运行容器中的环境变量
- 健康检查与容器列表，确认实际服务实例

## 已收集证据
- `docker-compose.yml` 的 backend 服务已配置 `env_file: - ${BACKEND_ENV_FILE}`，说明容器设计上会读取生成的 `backend.env`。
- `deploy_ubuntu.sh` 会生成 `backend.env`，且其中已写入邮件配置模板。
- `deploy_ubuntu.sh` 的 `start_application_stack()` 仅在镜像重建或 nginx 配置变化时显式 `--force-recreate`，未把 `backend.env` 内容变化作为 backend 容器重建条件。

## 当前判断
- 假设 1：暂未直接证实或证伪，需要实际部署目录中的 `backend.env` 文件内容。
- 假设 2：已证伪，`docker-compose.yml` 确实会注入 `${BACKEND_ENV_FILE}`。
- 假设 3：高度成立，`backend.env` 更新后 backend 容器可能未被重建，继续使用旧环境变量。
- 假设 4：暂未直接证伪，但当前证据优先支持假设 3。
- 假设 5：暂未直接证伪，需要用户侧运行容器/端口证据。

## 已实施修复
- 在 `deploy_ubuntu.sh` 中新增 `backend.env` 内容哈希跟踪。
- 当 `backend.env` 内容发生变化时，自动将 backend 加入 `--force-recreate` 列表，确保新环境变量进入容器。
