# A2UI 展示卡片

使用 `dws chat message send-a2ui-card` 创建卡片，使用 `update-a2ui-card` 更新和完结。

下面以任务进度卡片为例，布局、组件 ID、数据路径和文案可按用户需求调整，并保持引用一致。一次性展示结果时，可创建后直接完结；需要持续展示进度时，再发送中间更新。

本例的 `CollapsiblePanel.title` 使用字符串，通过 `updateComponents` 修改；`Markdown.content` 绑定正文，通过 `updateDataModel` 更新。其他组件属性按需查阅[钉钉公开 Catalog](https://dingtalk.com/card/a2ui/catalogs/public/catalog.json)。

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

允许转发时，在创建命令中加上 `--support-forward`，默认 `false`。

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

`--flow-status` 控制卡片流转状态；示例中的 `dataModel.status` 用于记录业务状态，可按需要维护。

## 组件注解

`--a2ui-annotations` 通过 surface 和组件 ID 关联 `--content` 中的 A2UI 消息。例如：`--a2ui-annotations '[{"surfaceId":"example-card","componentId":"answer","type":"artifact"}]'`。

- `surfaceId` 对应 `createSurface.surfaceId` 和 `updateComponents.surfaceId`。
- `componentId` 对应该 surface 中 `updateComponents.components[].id`，按需要关联的实际组件填写。
- `type` 是注解类型，按实际支持的类型和用途选择；此处以 `artifact` 为例。

示例中的 `answer` 是上面卡片定义的组件 ID，可随组件定义调整。

注解直接传 JSON 对象数组；卡片消息的 `--content` 为 JSON 字符串数组。更新时可引用此前已创建的 surface 和组件。

该参数可选，支持 `[]`；省略时，创建请求不携带注解字段，更新请求沿用原有的空数组。

## 检查与排错

- 组件名、属性类型和绑定方式与对应 Catalog 一致；`children` 引用已有组件，数据绑定与更新路径对应数据模型。
- 三份文件使用同一个 `surfaceId`。示例值 `example-card` 可整体替换；接收对象和 `bizId` 占位符在执行前替换。
- 用 `jq` 检查 JSON；字符串数组逐项 `fromjson` 解码后应与原对象数组一致。
- 发送后核对创建结果和客户端展示，再检查同一 `bizId` 的更新和完结效果。

遇到 `a2ui dws catalog validation failed` 时，对照 Catalog 检查组件属性类型、内层 JSON、Catalog 地址和 surface 创建顺序，保留错误码与 trace 定位。
