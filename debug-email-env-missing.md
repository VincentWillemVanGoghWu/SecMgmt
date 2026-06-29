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
- `cmd/server/main.go` 中 `resolveRootDir()` 在容器里会返回当前工作目录 `/app`。
- `internal/config/config.go` 中 `config.Load()` 在尝试读取 `/app/.env` 后，还会执行 `viper.AutomaticEnv()`，因此后端支持直接读取容器进程环境变量，并不强依赖 `/app/.env` 文件存在。
- 用户提供的服务器终端截图显示：`cat /opt/secmgmt/runtime/generated/backend.env | grep PUSH_EMAIL` 没有任何输出，说明服务器上实际生成的 `backend.env` 中不存在 `PUSH_EMAIL_*` 配置。
- 用户后续手工追加后再次 `cat /opt/secmgmt/runtime/generated/backend.env | grep PUSH_EMAIL` 有输出，但每行前面都带有前导空格，说明文件中很可能写成了 ` PUSH_EMAIL_...` 而不是 `PUSH_EMAIL_...`。

## 当前判断
- 假设 1：已证实，服务器上实际生成的 `backend.env` 缺少 `PUSH_EMAIL_*` 配置。
- 假设 2：已证伪，`docker-compose.yml` 确实会注入 `${BACKEND_ENV_FILE}`。
- 假设 3：可能同时存在，但不是当前首要根因；在 `backend.env` 本身缺失邮件配置前，容器重建与否不影响此次报错。
- 假设 4：基本证伪，代码支持直接读取容器环境变量，读取方式本身没有问题。
- 假设 5：高度成立，手工追加时写入了带前导空格的键名，导致 `env_file` 无法正确导出 `PUSH_EMAIL_*`。

## 已实施修复
- 在 `deploy_ubuntu.sh` 中新增 `backend.env` 内容哈希跟踪。
- 当 `backend.env` 内容发生变化时，自动将 backend 加入 `--force-recreate` 列表，确保新环境变量进入容器。
