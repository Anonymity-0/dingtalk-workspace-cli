# Minutes 低频意图与产品边界

本文件只承接不在根 Skill Golden Route 展开的低频能力。命令参数或 Safety 不确定时读取对应 compact leaf Schema；不要因此加载 Minutes 全量 Catalog。

## 低频能力路由

| 用户意图 | 推荐入口 | 关键边界 |
|---|---|---|
| “把发言人1改成张三” | `dws minutes +speaker-replace --id <taskUuid> --from "发言人1" --to "张三" [--target-uid <UID>]` | 这是逐字稿里的昵称替换，不是 speaker_id 与用户身份的系统级重绑；先完整预检源昵称，按 Runtime confirmation 执行并读回验证 |
| “把这篇听记里的 A/B/C 批量替换” | `dws minutes +replace-batch ...` | 先 dry-run/预检每一组 replacement；逐项执行和验证，部分失败返回非零，不把失败项丢掉 |
| “下载这条听记的音频/视频” | `dws minutes +download --id <taskUuid> --output <相对路径>` | 媒体 URL 是短期签名地址；默认直接安全下载。只有用户明确只要链接时使用 `--url-only` |
| “把多条听记媒体下载到目录” | `dws minutes +download --ids <uuid1,uuid2> --output-dir <相对目录>` | 最多 50 个，逐项保留成功与失败；禁止目录穿越和静默覆盖 |
| “把摘要、关键词、完整逐字稿、待办归档成一包” | `dws minutes +export-pack --id <taskUuid> --output <新目录>` | 逐字稿必须完整；所有必需产物验证后才原子发布目录；目标目录已存在时拒绝覆盖 |
| “归档时也带媒体” | `dws minutes +export-pack ... --include-media` | manifest 不保存签名 URL；媒体未就绪导致归档不完整时必须明确失败 |
| “按标签找听记/查语音备忘” | 对应 `minutes tag ...` / `minutes audio-memo list` 原子命令 | 属于长尾查询；先读取精确 leaf Schema，不把标签 ID 或分页参数猜出来 |

## 目标匹配

- 用户给了 `taskUuid` 或听记 URL：直接解析和使用真实 ID，不再按标题搜索。
- 用户给了标题：优先精确标题；没有精确命中时可以返回标题包含或语义相关候选。候选足够接近且唯一时可继续；差异明显、多个候选都合理或搜索分页未完成时必须让用户选择。
- 不要求用户口述标题必须与服务端字符逐字相同，但也不能把“语义相关”当成“已确认目标”。任何写操作前都要确保目标唯一。
- 用户明确说“最新一条”才使用 `+latest`；它不是通用消歧器，也不能用于录音绑定。

## 内容形态边界

| 需要的结果 | 使用 |
|---|---|
| 只在对话里查看摘要/逐字稿/待办 | Minutes 读取命令 |
| 形成可持续编辑的钉钉文档 | 先用 Minutes 读取真实内容，再切 `dingtalk-doc` 创建或编辑 |
| 把行动项变成可分派任务 | 先用 `+action-items`，再切 `dingtalk-todo` |
| 给别人发摘要文本 | 切 `dingtalk-chat`；不要把授予听记权限误当成发送消息 |
| 管理会议时间或会议室 | `dingtalk-calendar` |
| 管理普通云盘文件 | `dingtalk-drive`；听记媒体下载和听记上传仍由 Minutes 负责 |

## 写入与确认

- 发言人替换、批量文本替换都改变现有听记内容，按 Runtime confirmation 执行；dry-run 只显示计划，不写远端。
- 下载和导出写入本地工作目录，不改变远端听记，但必须使用安全相对路径、no-clobber 和原子发布语义。
- 任一批量流程只要存在失败项，就按 partial/非零交付完整 ledger；不能只汇报成功项。
