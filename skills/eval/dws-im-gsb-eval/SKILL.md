---
name: dws-im-gsb-eval
description: 快速运行 DWS IM / lark-cli GSB 核心评测，导出自然语言 Query 清单，检查 DWS Schema、CLI Help、Shortcut 与 Lark Skill/CLI 契约覆盖，并把逐条评测结果汇总为 Markdown 和 JSON 覆盖率报告。用于用户要求执行 IM eval、复核 Query 集覆盖率、比较 DWS 与 lark-cli 路由、分析缺失能力或生成 GSB 评测报告时。
---

# DWS IM GSB Eval

## 快速开始

1. 定位 `dws-im-gsb-core-query-set.md` 所在仓库。优先使用当前仓库；找不到时要求用户传 `--repo-root`。
2. 先运行只读快速检查：

```bash
python3 scripts/dws_im_gsb_eval.py quick \
  --repo-root <dws-repo> \
  --out-dir <dws-repo>/tmp/im-gsb-eval/<run-id>
```

3. 读取生成的 `contract-coverage.md`。任何分母变化、缺失命令或陈旧预期都属于契约漂移，不能用历史的 100% 结论覆盖。
4. 将 `manifest.jsonl` 逐条交给目标 Agent 或 GSB harness。Query 文本必须原样作为用户输入；不得把 `golden.jsonl` 中的预期指令、覆盖标签或答案同时泄漏给被测模型。
5. 让 harness 按 `results.template.jsonl` 的结构写出 `results.jsonl`，然后计分：

```bash
python3 scripts/dws_im_gsb_eval.py score \
  --manifest <run-dir>/manifest.jsonl \
  --golden <run-dir>/golden.jsonl \
  --results <run-dir>/results.jsonl \
  --output <run-dir>/eval-coverage.md \
  --json-output <run-dir>/eval-coverage.json
```

6. 向用户报告契约覆盖率与评测覆盖率；不要把二者合并成一个百分比。

## 运行模式

- `quick`：推荐入口；执行 `prepare` 与 `contract`，不调用业务写接口。
- `prepare`：从 Markdown Query 表生成机器可消费的 manifest 和结果模板。
- `contract`：用当前 `dws schema --all`、`dws shortcut list`、逐路径 `--help`、Shortcut 源码、Lark Skill 与 `lark-cli schema/--help` 计算静态覆盖。
- `score`：汇总真实 harness 结果，输出 Query、检查项与能力标签覆盖率。

## 结果填写约束

每条结果使用以下状态之一：

- `pass`：工具选择、参数和安全判断全部正确；`mode=live` 时执行结果也必须正确。
- `fail`：已经评测但至少一项不正确。
- `blocked_fixture`：账号、权限、群、消息或人工状态不足；不是产品失败。
- `skipped`：经明确评测策略排除。
- `not_run`：尚未评测。

`checks` 包含 `selection`、`parameters`、`safety`、`execution`。`mode=contract` 时前三项必须为布尔值，`execution` 应为 `null`；`mode=live` 时四项都必须为布尔值。不要仅填写 `status=pass` 而省略检查证据。

## 安全边界

- `quick` 和 `contract` 必须保持只读。
- 真实造数和执行前，读取仓库中的 `docs/dws-im-gsb-fixture-plan.md` 并通过对应 Ready Gate。
- 不把账号密码、Token、Cookie、Webhook token 或真实业务 ID 写入 Skill、仓库或报告。
- 发送、成员变更、权限变更和状态修改只在用户授权的测试资源上执行。
- 解散群、外部群升级等不可逆 Query 必须独立确认，且最后执行。
- Lark 原生 API 调用前先查询实时 Schema；本机缺失路径应报告版本漂移，不得把根帮助的退出码当成命令存在。

## 报告解释

- **DWS 当前交付面覆盖率**：当前 Chat Schema、兼容/辅助路径、runnable parent 与已发布 Shortcut 中，被 Query 集表达并通过契约检查的比例。
- **DWS Shortcut 语义覆盖率**：已发布 `P:` 与源码未发布 `H:` 语义中，被 Query 映射的比例。
- **Lark Skill 语义覆盖率**：`L:` 能力是否在当前 Skill 证据中存在。
- **Lark CLI 可执行覆盖率**：当前安装版本实际能解析的 Lark 原生 API/Shortcut 比例。
- **Eval Query 覆盖率**：有 `pass` 或 `fail` 结果的 Query 数 / 全部 Query。
- **能力通过覆盖率**：至少有一条通过 Query 的唯一能力标签数 / 该能力面分母。

当 `blocked_fixture` 存在时，单独报告阻塞数和所需账号/状态；不要将它计为 `fail`，也不要计入已完成 Query。
