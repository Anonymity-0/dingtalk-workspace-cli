# 白板整页替换与清空

仅在用户明确要求整页重绘或清空时读取本页。overwrite 会替换整个单页白板，不是
局部节点更新。

## 执行顺序

1. 对同一 `nodeId/partId` 执行一次 `+query`，保留完整当前快照并汇总将被删除的
   节点。
2. 向用户说明目标、overwrite 影响和新节点数量，按 Runtime gate 取得确认。
3. 一次提交完整终态；不要先清空再追加。
4. `+update verified=true` 后，对同一 `nodeId/partId` 再执行一次最终 `+query`。
5. 最终 query 与预期终态一致后才交付；缺失、不一致或失败只能报告
   partial/failure。

```bash
# 旧版快照
dws whiteboard +query --node <DOC_NODE_ID> --part-id <PART_ID> --format json
dws whiteboard +update --node <DOC_NODE_ID> --part-id <PART_ID> \
  --source @overwrite.json --format json
# overwrite 后的最终快照
dws whiteboard +query --node <DOC_NODE_ID> --part-id <PART_ID> --format json
```

清空整页的更新文件：

```json
{
  "overwrite": true,
  "source": {
    "schemaVersion": "1.0",
    "catalogVersion": "dml-v1",
    "nodes": []
  }
}
```

只有用户明确要求清空时才允许空数组。整页重绘使用相同信封，但 `nodes` 必须包含
完整终态。超时、commit-unknown 或读回不一致时不要自动重放 overwrite；保留旧快照
和完整新 Payload，并报告不确定状态。
