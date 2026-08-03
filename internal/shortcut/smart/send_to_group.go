// Copyright 2026 Alibaba Group
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package smart

import (
	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/shortcut"
	chatshortcut "github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/shortcut/chat"
	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/shortcut/targetresolver"
)

// SendToGroup: message a group by its NAME, no openConversationId juggling.
//
// Steps: search groups by name → resolve to a single openConversationId
// (disambiguate on multiple matches, never guess) → send a markdown message.
// Replaces `chat search --query <群名>` (copy openConversationId) →
// `chat +messages-send --group <openConversationId>`.
//
// Note: the group lookup uses `search_groups` (im server, keyword search over
// group NAMES) — NOT `search_common_groups`, which searches by member nicknames
// and cannot locate a group by its title.
//
//	dws chat +send-to-group --group 项目冲刺 --text "今天 5 点前提交进度"
var SendToGroup = shortcut.Shortcut{
	Service:     "chat",
	Command:     "+send-to-group",
	Product:     "chat",
	Description: "按群名直接给群发消息（自动搜群解析 openConversationId）",
	Intent: "当你只知道群的名字、想直接往这个群里发一条消息而不想先手动查群 ID 时使用；" +
		"内部先按群名搜索群聊解析出唯一 openConversationId 再发送，群名匹配到多个群时会列出候选让你区分、绝不自行假定。会真实发出群消息。",
	Risk: shortcut.RiskWrite,
	Flags: []shortcut.Flag{
		{Name: "group", Type: shortcut.FlagString, Desc: "群名称（搜群关键词，用群名里连续的核心词）", Required: true},
		{Name: "text", Type: shortcut.FlagString, Desc: "消息内容（支持 Markdown）", Required: true},
		shortcut.AIMessageTagFlag(),
	},
	Tips: []string{`dws chat +send-to-group --group 项目冲刺 --text "今天 5 点前提交进度"`},
	Execute: func(rt *shortcut.RuntimeContext) error {
		groupName := rt.Str("group")
		text := rt.Str("text")

		resolved, err := targetresolver.ResolveChat(rt, groupName)
		if err != nil {
			return err
		}
		return chatshortcut.ExecuteResolvedUserMarkdown(
			rt,
			chatshortcut.ResolvedUserMessageTarget{
				GroupID: resolved.Selected.OpenConversationID,
			},
			text,
		)
	},
}

// sendGroupMatch is a single group candidate resolved from a name search.
type sendGroupMatch struct {
	id   string
	name string
}

// extractGroupsForSend pulls {openConversationId, title} out of a search_groups
// response. The result may be a bare list, or nested under
// result/result.items/result.groups (field names per chat search's real
// response shape), and the name field may be "title" or "name".
func extractGroupsForSend(data map[string]any) []sendGroupMatch {
	chats := targetresolver.ExtractChats(data)
	out := make([]sendGroupMatch, 0, len(chats))
	for _, chat := range chats {
		out = append(out, sendGroupMatch{id: chat.OpenConversationID, name: chat.Name})
	}
	return out
}

// preferExactGroupMatches keeps name-based routing ambiguity-safe while
// avoiding a common false ambiguity from substring search. If the server
// returns exactly one group whose title equals the requested name, that exact
// group wins over prefix/suffix matches. Duplicate rows for the same
// openConversationId are collapsed before selection.
func preferExactGroupMatches(groups []sendGroupMatch, query string) []sendGroupMatch {
	chats := make([]targetresolver.Chat, 0, len(groups))
	for _, group := range groups {
		chats = append(chats, targetresolver.Chat{OpenConversationID: group.id, Name: group.name})
	}
	selected := targetresolver.PreferExactChats(chats, query)
	out := make([]sendGroupMatch, 0, len(selected))
	for _, chat := range selected {
		out = append(out, sendGroupMatch{id: chat.OpenConversationID, name: chat.Name})
	}
	return out
}

func sendGroupLabels(groups []sendGroupMatch) []string {
	chats := make([]targetresolver.Chat, 0, len(groups))
	for _, group := range groups {
		chats = append(chats, targetresolver.Chat{OpenConversationID: group.id, Name: group.name})
	}
	return targetresolver.ChatLabels(chats)
}

func init() {
	shortcut.Register(SendToGroup)
}
