# A2UI 展示卡片

使用 `dws chat message send-a2ui-card` 创建卡片，使用 `update-a2ui-card` 更新和完结。组件属性可查阅[钉钉公开 Catalog](https://dingtalk.com/card/a2ui/catalogs/public/catalog.json)。命令参数以当前版本的 `--help` 和 leaf Schema 为准。

本指南以任务进度展示卡片为例，覆盖创建、内容更新和完结，可按任务调整标题和正文。`CollapsiblePanel.title` 使用字符串；修改标题时发送 `updateComponents`。`Markdown.content` 支持数据绑定，正文通过 `updateDataModel` 更新。新增组件或交互能力时，需另行核对当前 DWS 参数、Catalog 契约及实际支持情况。

示例 JSON 使用便于编辑的对象数组，执行命令时用 `jq -c 'map(tojson)'` 转成 `--content` 要求的字符串数组。需要本地安装 `jq`。

## 创建

保存为 `a2ui-create.json`。先用 `createSurface` 指定 Catalog 和初始数据，再用 `updateComponents` 定义组件。示例组件使用 surface 指定的钉钉公开 Catalog。

```json
[
  {
    "version": "v1.0",
    "createSurface": {
      "surfaceId": "example-card",
      "catalogId": "https://dingtalk.com/card/a2ui/catalogs/public/catalog.json",
      "dataModel": {
        "answer": {
          "displayText": "任务已开始。"
        },
        "status": "doing"
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
          "align": "stretch",
          "children": [
            "executionPanel"
          ]
        },
        {
          "id": "executionPanel",
          "component": "CollapsiblePanel",
          "title": "正在处理任务",
          "children": [
            "answer"
          ],
          "fallbackMarkdown": "任务进度"
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

替换接收人占位符后发送：

```bash
dws chat message send-a2ui-card \
  --open-dingtalk-id '<接收人openDingTalkId>' \
  --content "$(jq -c 'map(tojson)' a2ui-create.json)" \
  -f json
```

群聊时将 `--open-dingtalk-id` 替换为 `--conversation-id '<群openConversationId>'`，两者互斥。接收对象使用当前组织中确认的实际 ID。创建状态默认 `PROCESSING`；发送成功后保存实际返回的 `bizId`，用于后续更新。

## 更新

保存为 `a2ui-update.json`。以下消息更新面板标题和正文，可根据任务进度多次调用。替换组件定义时保留该组件所需的 `children` 等字段。

```json
[
  {
    "version": "v1.0",
    "updateComponents": {
      "surfaceId": "example-card",
      "components": [
        {
          "id": "executionPanel",
          "component": "CollapsiblePanel",
          "title": "正在处理第 2 步",
          "children": [
            "answer"
          ],
          "fallbackMarkdown": "任务进度"
        }
      ]
    }
  },
  {
    "version": "v1.0",
    "updateDataModel": {
      "surfaceId": "example-card",
      "path": "/answer/displayText",
      "value": "第 2 步正在执行。"
    }
  },
  {
    "version": "v1.0",
    "updateDataModel": {
      "surfaceId": "example-card",
      "path": "/status",
      "value": "doing"
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

更新要求 surface 已存在。`surfaceId` 与创建时保持一致，`bizId` 使用同一次创建的返回值。

## 完结

保存为 `a2ui-finish.json`，标题和正文填写实际任务结果：

```json
[
  {
    "version": "v1.0",
    "updateComponents": {
      "surfaceId": "example-card",
      "components": [
        {
          "id": "executionPanel",
          "component": "CollapsiblePanel",
          "title": "处理完成",
          "children": [
            "answer"
          ],
          "fallbackMarkdown": "任务进度"
        }
      ]
    }
  },
  {
    "version": "v1.0",
    "updateDataModel": {
      "surfaceId": "example-card",
      "path": "/answer/displayText",
      "value": "任务已完成。"
    }
  },
  {
    "version": "v1.0",
    "updateDataModel": {
      "surfaceId": "example-card",
      "path": "/status",
      "value": "finished"
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

`--flow-status` 控制卡片流转状态；`dataModel.status` 是本示例的业务数据，按需要维护，不能代替命令参数。仅准备文件时，完成文案表示待执行的最终内容。

## 检查与排错

- 组件名、属性类型和绑定方式与对应 Catalog 一致；`children` 引用已有组件，数据绑定与更新路径对应数据模型。
- 三份文件使用同一个 `surfaceId`。示例值 `example-card` 可整体替换；接收对象和 `bizId` 占位符在执行前替换。
- 用 `jq` 检查 JSON；字符串数组逐项 `fromjson` 解码后应与原对象数组一致。
- 实际发送后检查创建结果、客户端展示，以及同一 `bizId` 的更新和完结效果。CLI 参数解析成功不代表服务端已接受组件或客户端已正确渲染。

遇到 `a2ui dws catalog validation failed` 时，对照 Catalog 检查组件属性类型、内层 JSON、Catalog 地址和 surface 创建顺序，保留错误码与 trace 定位。
