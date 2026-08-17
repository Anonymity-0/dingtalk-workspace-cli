# 导入本地文件：`+import` Golden Route

## 唯一推荐入口

```bash
dws doc +import --file ./report.docx --format json
dws doc +import --file ./report.docx --folder <FOLDER_ID> --format json
dws doc +import --file ./notes.md --workspace <WORKSPACE_ID> --name "会议纪要" --format json
```

`+import` 一次完成创建会话、上传、确认转换和终态轮询。支持 `doc/docx/xls/xlsx/md/txt/xmind/mark`，文件大小上限 20MB。返回 `taskId` 只是稳定任务标识；若 shortcut 同时返回终态 `success` 及已完成 `steps`，不需要再调用原子状态查询。只有终态缺失、`unknown`、超时或中断才进入恢复流程。

## 本地文件边界

- `--file` 只接受当前工作目录内已存在的相对路径；禁止绝对路径、`..` 或符号链接逃逸。
- `--folder` 与 `--workspace` 都是可选位置；都不传时导入默认根目录，两者都有时优先 `--folder`。
- CLI 负责上传和格式转换。不要先用 Python/Office 库解析文件，不要安装本地依赖来伪造在线导入结果，也不要手写 HTTP 上传。
- 白名单外格式（如 HTML/PDF）自动改走原文件上传，返回 `fallback=upload`、`converted=false`；不得报告成已经转换为可编辑在线文档。
- “在线改/协作编辑/转在线文档”属于导入转换；“存着/归档/保留原文件/不要转换”属于 `dingtalk-drive` 纯上传。目标为文档空间时，纯上传使用 `drive upload --workspace <WORKSPACE_ID>`，不要因容器叫“文档空间”就误报为在线文档。

## 失败处理

- 发起前的格式、大小或路径校验失败：修正输入后再执行。
- 已返回 `taskId` 后超时或中断：保留该 `taskId`，读取精确恢复命令 Schema 后只查询原任务；禁止重新提交导入。
- 返回状态未知时原样报告，不把本地文件内容改走 `+create`，因为这会改变格式保真和任务语义。
- 白名单外格式如果目标是钉盘而非文档空间，切换到 `dingtalk-drive` 上传。

正常导入不得手工编排原子 `doc import` 子步骤。只有 shortcut 未公开必要的恢复参数时，才按精确 leaf Schema 使用原子查询命令。
