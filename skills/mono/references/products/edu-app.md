# 家校应用 (edu-app) 命令参考

## 命令总览

### message (消息管理)

| 命令 | 用途 | 必填参数 |
|------|------|----------|
| `message summary-list` | 查询消息摘要列表 | `--class-id`, `--cid`, `--target-role`, `--status` |

> `--target-role`: guardian(家长) / student(学生)
> `--status`: 0(未处理) / 1(已处理)

### task (任务管理)

| 命令 | 用途 | 必填参数 |
|------|------|----------|
| `task publish-list` | 查询发布的家校任务列表（仅老师） | 无（均可选） |
| `task all-list` | 查询全部家校任务列表（仅老师） | `--biz-id`(班级ID) |
| `task student-list` | 查询学生待办任务列表 | `--students`(JSON数组) |

> `--task-sources` 可选值（逗号分隔）: EDU_HOMEWORK, EDU_CARD, EDU_NOTICE, EDU_SR, EDU_DIPLOMA

### report (成绩单管理)

| 命令 | 用途 | 必填参数 |
|------|------|----------|
| `report get` | 获取成绩单列表 | `--ids`(逗号分隔整数) |
| `report by-teacher` | 查询老师创建的成绩单 | 无（均可选） |
| `report by-class` | 查询班级学生成绩明细 | `--report-id`, `--class-id` |
| `report by-student-list` | 查询学生收到的成绩单 | `--class-id`, `--student-id` |
| `report by-student-detail` | 查询学生成绩明细 | `--report-id`, `--student-id`, `--class-id` |

### notice (通知管理)

| 命令 | 用途 | 必填参数 |
|------|------|----------|
| `notice confirm` | 确认收到通知 | `--notice-id`, `--student-id` |

### circle (班级圈)

| 命令 | 用途 | 必填参数 |
|------|------|----------|
| `circle posts` | 查询学生班级圈动态 | `--class-id`, `--student-id`, `--target-role` |

> `--target-role`: guardian(家长视角) / student(学生视角)
> 返回动态的文字内容、图片URL列表、发布者姓名、发布时间、评论数、点赞数等。

### card (打卡管理)

| 命令 | 用途 | 必填参数 |
|------|------|----------|
| `card update` | 修改打卡任务的标题或内容（仅老师且为创建者） | `--card-id`, `--identifier`, (`--title` 或 `--content` 至少一个) |
| `card end` | 提前结束打卡任务（仅老师且为创建者） | `--card-id` |
| `card list` | 查询孩子/本人打卡列表（含进行中与已完结） | `--status` |
| `card user-statistic` | 查询班级已完成/未完成人员（仅老师/班主任） | `--card-id`, `--task-code`, `--class-id` |
| `card finish-info` | 查询打卡详情及完成进度 | `--card-id`, `--card-biz-id` |

> `--status`: FINISH(已完结) / UNFINISH(进行中)
> `--identifier` 建议格式 `orgId-staffId-UUID`，用于幂等去重
> `card finish-info --target-role`: teacher / headmaster / guardian / student，未传时按 uid 真实身份自动推断
> `card finish-info` 当 `targetRole=guardian` 时，可传 `--student-id` 指定查看某个孩子的进度

## 意图判断

用户说"消息/消息摘要" → message summary-list
用户说"作业/任务/打卡/家校任务" → task 子命令
用户说"成绩/成绩单" → report 子命令
用户说"通知/确认通知" → notice confirm
用户说"班级圈/成长记录/学生动态" → circle posts
用户说"打卡/打卡任务/打卡完成情况/卡片完成情况" → card 子命令

关键区分: task(家校任务/作业) vs report(成绩单)
关键区分: circle(班级圈动态/成长记录) vs message(AI消息总结)

## 核心工作流

### 老师场景
1. 查看发布的任务 → `task publish-list --need-statistic -f json`
2. 查看某班全部任务 → `task all-list --biz-id <classId>`
3. 查看成绩单 → `report by-teacher --status 1`
4. 查看班级成绩明细 → `report by-class --report-id <id> --class-id <classId>`
5. 修改打卡标题/内容 → `card update --card-id <cardId> --identifier <id> --title "新标题"`
6. 提前结束打卡 → `card end --card-id <cardId>`
7. 查看某班打卡完成情况 → `card user-statistic --card-id <cardId> --task-code <taskCode> --class-id <classId> --finish`
8. 查看某班未打卡人员 → `card user-statistic --card-id <cardId> --task-code <taskCode> --class-id <classId>`
9. 查看某打卡完成进度 → `card finish-info --card-id <cardId> --card-biz-id <cardBizId>`

### 家长场景
1. 查看孩子待办 → `task student-list --students '[{"userId":"<uid>","bizId":"<classId>"}]'`
2. 确认通知 → `notice confirm --notice-id <id> --student-id <uid>`
3. 查看孩子班级圈动态 → `circle posts --class-id <classId> --student-id <studentId> --target-role guardian`
4. 查看孩子进行中打卡 → `card list --status UNFINISH`
5. 查看孩子某打卡进度 → `card finish-info --card-id <cardId> --card-biz-id <cardBizId>`

### 学生场景
1. 查看自己的班级圈动态 → `circle posts --class-id <classId> --student-id <studentId> --target-role student`

## 上下文传递表

| 操作 | 从返回中提取 | 用于 |
|------|-------------|------|
| `edu-contact school class-list` | `deptId` | task all-list 的 --biz-id |
| `edu-contact class students` | `userId` | task student-list 的 students.userId |
| `edu-group class-group conversation-id` | `conversationId` | message summary-list 的 --cid |
| `report by-teacher` | `schoolReportId` | report get/by-class/by-student-detail 的 --report-id |
| `edu-contact family children` | `studentUserId`, `classId` | circle posts 的 --student-id, --class-id |
| `task publish-list` | `cardId` | card update/end/finish-info` 的 --card-id |
| `task publish-list` | `taskCode` | card user-statistic 的 --task-code |
