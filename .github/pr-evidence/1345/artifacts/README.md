# A2UI 卡片：示例任务进度

这是一套仅在本地准备的三阶段模板，尚未发送：

1. `a2ui-create.json`：显示“正在处理示例任务 / 示例任务已开始。”
2. `a2ui-step-2.json`：显示“正在处理第 2 步 / 示例任务第 2 步正在执行。”
3. `a2ui-finish.json`：显示“处理完成 / 示例任务已完成。”并设置完成状态。

三份文件必须保持相同的 `surfaceId`。如果复制成另一张并行卡片，请把三份文件中的
`example-task-progress` 一起替换为新的唯一值。

## 发送前本地校验

```bash
cd /private/tmp/dws-1345-evidence-r3/work/a2ui-example-task
./validate-local.sh
```

## 稍后发送

先填写真实接收人。单聊与群聊只能选择一种；不要同时传两个目标参数。

单聊创建：

```bash
cd /private/tmp/dws-1345-evidence-r3/work/a2ui-example-task
dws chat message send-a2ui-card \
  --open-dingtalk-id '<接收人openDingTalkId>' \
  --content "$(jq -c 'map(tojson)' a2ui-create.json)" \
  --format json
```

群聊创建：

```bash
cd /private/tmp/dws-1345-evidence-r3/work/a2ui-example-task
dws chat message send-a2ui-card \
  --conversation-id '<群openConversationId>' \
  --content "$(jq -c 'map(tojson)' a2ui-create.json)" \
  --format json
```

创建成功后，从真实 JSON 返回中保存 `bizId`。然后按顺序更新同一张卡片：

```bash
dws chat message update-a2ui-card \
  --biz-id '<创建返回的bizId>' \
  --flow-status INPUTTING \
  --content "$(jq -c 'map(tojson)' a2ui-step-2.json)" \
  --format json

dws chat message update-a2ui-card \
  --biz-id '<创建返回的bizId>' \
  --flow-status FINISH \
  --content "$(jq -c 'map(tojson)' a2ui-finish.json)" \
  --format json
```

不要把创建命令返回的 `bizId` 猜成接收人或消息 ID；两次更新都必须使用本次创建的真实
`bizId`。若使用特定账号/组织，三次调用都追加同一个 `--profile`。

## 验证边界

已在本机验证的内容：

- `dws chat message send-a2ui-card --help` 与
  `dws chat message update-a2ui-card --help` 中的命令、互斥接收人参数、必填参数、
  `INPUTTING`/`FINISH` 状态及默认创建状态 `PROCESSING`。
- 三份文件是非空 JSON 对象数组，可被 `jq -c 'map(tojson)'` 编码成 CLI 所需的
  JSON 字符串数组，并可逐项无损解码。
- 三阶段 `surfaceId` 一致，创建数据绑定、更新路径和完成标记相互对应。

仍需实际发送验证的内容：

- 接收人或群 ID 是否属于所选 profile，账号认证与权限是否有效。
- 服务端 Catalog 是否接受全部组件和属性，以及创建是否返回有效 `bizId`。
- 钉钉客户端是否按预期渲染三个阶段，进度更新与完结是否作用于同一张卡片。
- 每次更新的真实投递结果、错误码和 trace；不能仅凭本地校验或进程退出码认定送达。

当前没有执行上述任何发送命令。
