# A2UI 展示卡片：脱敏模板

用于创建、增量更新和完结展示卡片。命令为 `send-a2ui-card` / `update-a2ui-card`；旧写法 `--card-engine a2ui` 已迁移为独立命令。

来源：使用者提供的成功示例，命令已对照主干 `9351a819` 核对。此版本替换了业务标识和展示文案，保留原始组件、数据结构与更新顺序。脱敏版经过本地 JSON、引用和命令检查；服务端接受及客户端渲染仍需实际发送验证。本模板的范围是展示与状态更新；表单收集还需有依据的输入组件、提交动作和回调事件契约。

## 按模板生成

先将用户需求整理为卡片主题、处理中提示、进度内容和完成结果，再复制下方三份 JSON，按表替换展示字段。以“示例任务进度”为例：

| 阶段 | 修改位置 | 示例文案 |
|---|---|---|
| 创建 | `createSurface.dataModel.execution.title`；需要初始正文时填写 `createSurface.dataModel.answer.displayText` | 标题“正在处理示例任务”，正文“示例任务已开始。” |
| 增量更新 | `updateDataModel.path` 为 `/execution/title`、`/answer/displayText` 的消息中的 `value` | 标题“正在处理第 2 步”，正文“示例任务第 2 步正在执行。” |
| 完结 | `/execution/title`、`/tools/title` 对应的 `value`；`/answer/text` 和 `/answer/displayText` 同步填写最终正文 | 标题“处理完成”，正文“示例任务已完成。” |

完成结果以实际任务结果为准；只准备文件时将完成文案标明为待执行的模板内容。完结模板中的 `/status`、`/execution/done`、`/answer/done` 和清理消息一起保留，发送完结更新时使用 `--flow-status FINISH`。

- `example-card` 是示例 surfaceId，可整体替换；同一卡片的创建和更新使用同一个值。
- 保留公开 Catalog URL、`version`、组件名及属性类型。模板中 `createSurface.catalogId` 指向钉钉 Catalog，`Column` 显式引用基础 Catalog。新增组件或属性时，先查对应 Catalog 的定义。
- 修改数据路径时，同时修改 `dataModel`、组件的 `path` 绑定和后续 `updateDataModel.path`。
- 接收人和群 ID 来自当前组织中的真实解析结果；后续 `bizId` 取本次发送的实际返回。文档里的 ID 占位符须替换后执行。
- 复用业务案例时，替换姓名、项目名、内部地址和业务文案，删除凭据及真实接收对象。公开协议地址保留原值；脱敏后重新检查引用一致性。

以下 JSON 为便于编辑的**对象数组**。保存为指定文件后，用 `jq -c 'map(tojson)'` 转成 `--content` 要求的**字符串数组**。需要本地安装 `jq`。

## 1. 创建

将以下内容保存为 `a2ui-create.json`。`createSurface` 初始化数据模型，随后 `updateComponents` 定义组件树和数据绑定。

```json
[
  {
    "version": "v1.0",
    "createSurface": {
      "surfaceId": "example-card",
      "catalogId": "https://dingtalk.com/card/a2ui/catalogs/public/catalog.json",
      "dataModel": {
        "answer": {
          "displayText": "",
          "done": false,
          "text": ""
        },
        "card": {},
        "execution": {
          "done": false,
          "messages": {},
          "text": "",
          "textEffect": "shimmer",
          "timelineOrder": null,
          "title": "正在处理示例任务",
          "toolGroups": {},
          "visible": true
        },
        "status": "doing",
        "tools": {
          "calls": {},
          "displayText": "",
          "title": "正在处理",
          "visible": true
        }
      }
    }
  },
  {
    "version": "v1.0",
    "updateComponents": {
      "surfaceId": "example-card",
      "components": [
        {
          "id": "root",
          "component": "Column",
          "catalogId": "https://a2ui.org/specification/v1_0/catalogs/basic/catalog.json",
          "align": "stretch",
          "children": [
            "executionPanel",
            "answer"
          ]
        },
        {
          "id": "executionPanel",
          "component": "CollapsiblePanel",
          "children": [
            "executionTimeline"
          ],
          "fallbackMarkdown": "执行过程",
          "textEffect": {
            "path": "/execution/textEffect"
          },
          "title": {
            "path": "/execution/title"
          },
          "variant": "reasoning",
          "visible": {
            "path": "/execution/visible"
          }
        },
        {
          "id": "executionTimeline",
          "component": "Column",
          "catalogId": "https://a2ui.org/specification/v1_0/catalogs/basic/catalog.json",
          "align": "stretch",
          "children": []
        },
        {
          "id": "answer",
          "component": "Markdown",
          "content": {
            "path": "/answer/displayText"
          }
        }
      ]
    }
  }
]
```

单聊发送（先替换接收人占位符）：

```bash
dws chat message send-a2ui-card \
  --open-dingtalk-id '<接收人openDingTalkId>' \
  --content "$(jq -c 'map(tojson)' a2ui-create.json)" \
  -f json
```

群聊时将 `--open-dingtalk-id` 替换为 `--conversation-id '<群openConversationId>'`，二者互斥。创建状态默认 `PROCESSING`。发送成功后保存实际返回的 `bizId`，用于下面两次更新。

## 2. 增量更新

将以下内容保存为 `a2ui-update.json`：

```json
[
  {
    "version": "v1.0",
    "updateDataModel": {
      "surfaceId": "example-card",
      "path": "/execution/title",
      "value": "正在处理第 2 步"
    }
  },
  {
    "version": "v1.0",
    "updateDataModel": {
      "surfaceId": "example-card",
      "path": "/answer/displayText",
      "value": "示例内容正在写入。"
    }
  }
]
```

```bash
dws chat message update-a2ui-card \
  --biz-id '<本次发送返回的bizId>' \
  --flow-status INPUTTING \
  --content "$(jq -c 'map(tojson)' a2ui-update.json)" \
  -f json
```

按进度重复更新绑定的数据。仅有 `updateDataModel` 的消息需要已有 surface，不能作为此模板的完整创建内容。

## 3. 完结

将以下内容保存为 `a2ui-finish.json`：

```json
[
  {
    "version": "v1.0",
    "updateDataModel": {
      "surfaceId": "example-card",
      "path": "/execution/text",
      "value": ""
    }
  },
  {
    "version": "v1.0",
    "updateDataModel": {
      "surfaceId": "example-card",
      "path": "/execution/timelineOrder",
      "value": []
    }
  },
  {
    "version": "v1.0",
    "updateDataModel": {
      "surfaceId": "example-card",
      "path": "/answer/text",
      "value": "示例任务已完成。"
    }
  },
  {
    "version": "v1.0",
    "updateDataModel": {
      "surfaceId": "example-card",
      "path": "/answer/displayText",
      "value": "示例任务已完成。"
    }
  },
  {
    "version": "v1.0",
    "updateDataModel": {
      "surfaceId": "example-card",
      "path": "/status",
      "value": "finished"
    }
  },
  {
    "version": "v1.0",
    "updateDataModel": {
      "surfaceId": "example-card",
      "path": "/execution/done",
      "value": true
    }
  },
  {
    "version": "v1.0",
    "updateDataModel": {
      "surfaceId": "example-card",
      "path": "/answer/done",
      "value": true
    }
  },
  {
    "version": "v1.0",
    "updateDataModel": {
      "surfaceId": "example-card",
      "path": "/execution/textEffect",
      "value": ""
    }
  },
  {
    "version": "v1.0",
    "updateDataModel": {
      "surfaceId": "example-card",
      "path": "/execution/title",
      "value": "已处理完成"
    }
  },
  {
    "version": "v1.0",
    "updateDataModel": {
      "surfaceId": "example-card",
      "path": "/tools/title",
      "value": "已处理完成"
    }
  },
  {
    "version": "v1.0",
    "updateComponents": {
      "surfaceId": "example-card",
      "components": [
        {
          "id": "executionTimeline",
          "component": "Column",
          "catalogId": "https://a2ui.org/specification/v1_0/catalogs/basic/catalog.json",
          "align": "stretch",
          "children": []
        }
      ]
    }
  }
]
```

```bash
dws chat message update-a2ui-card \
  --biz-id '<本次发送返回的bizId>' \
  --flow-status FINISH \
  --content "$(jq -c 'map(tojson)' a2ui-finish.json)" \
  -f json
```

`--flow-status` 控制卡片流转状态；`dataModel` 中的 `status`、`done` 和文案是模板数据，按消息显式更新。根据实际执行结果写入最终内容，再检查更新结果。

## 生成后检查

1. 对照用户主题检查三份文件的标题和正文，确认“处理中 → 进度更新 → 完成结果”连贯，业务文案已完成脱敏。
2. 检查创建文件同时包含 `createSurface` 和 `updateComponents`；组件 `children` 指向已有组件，数据绑定和更新路径对应创建时的数据模型，三份文件使用同一个 `surfaceId`。
3. 分别解析三份 JSON，并用 `jq -c 'map(tojson)'` 编码；将字符串逐项 `fromjson` 解码后应与原对象数组一致。按本机两条命令的 `--help` 核对参数，更新状态依次为 `INPUTTING`、`FINISH`。

本地准备完成时交付三份 JSON、对应命令和检查结果。实际发送后的验收依次检查创建返回、客户端展示、同一 `bizId` 的进度更新及完结效果；某一步失败时保留该步错误，修正后验证该步。

## 校验失败时

`a2ui dws catalog validation failed` 表示内容未通过服务端 Catalog 校验。先检查内层 JSON、Catalog 地址、组件属性、创建顺序和 surfaceId，再与本模板或相应组件规范逐项比较。CLI 接受外层字符串数组只证明参数可解析，不能证明组件结构有效。保留错误码与 trace 用于定位；验证结论区分发送接受、客户端展示、更新成功和真实回调到达。
