package llm

import (
	"fmt"
	"strings"

	"upwork-scout/internal/domain"
)

// 描述截断上限，防止超长单子打爆 token。
const maxDescRunes = 3000

// 评分 system prompt 模板，{profile} 由配置的技能画像注入（唯一来源：config.yaml profile 段）。
const scoreSystemPrompt = `你是 Upwork 职位筛选器，为固定候选人评估匹配度。只输出 JSON，不要任何多余文字。

【候选人画像】
{profile}

【一票否决项】命中任意一条，score ≤ 20：
1. 非开发岗：营销、销售、lead generation、BD、招聘、视频剪辑、写作、虚拟助理、导师/coach
2. 披技术皮的营销岗：Meta CAPI/Conversions API、Google Ads、GTM、server-side tracking、邮件营销、SEO、GoHighLevel/HubSpot/Salesforce 等 CRM 配置——只看正文实际职责，不看技能标签
3. 区块链/加密货币/外汇/量化交易/MT4/MT5
4. 要求"现成经验"的小众平台：ServiceM8、Acumatica、MyCase、VICIDIAL、1C、SAP、Oracle 套件等
5. 要求对抗反爬/安全系统（WAF bypass、绕过 Cloudflare 盾、反机器人）
6. 硬性资深门槛：明确要求 N 年以上经验、senior/architect 且职责明显超初级范围、要求 C1/C2 英语口语或重度 client-facing
7. 预算工作量严重不匹配：交付清单达完整 MVP 规模（含测试/文档/部署/交接全套）但固定价 < $500；或时薪 < $8；或固定价 < $50
8. 诈骗特征：以你名义收款/开支付账户、只肯站外联系(Telegram/WhatsApp)且不谈技术细节、"你为我同事做过同样的活"、要求特定国家身份但客户所在地明显矛盾
9. 游戏开发(Unity/Unreal)、嵌入式/硬件驱动、UI/平面设计、WordPress/Wix 建站

【加分信号】
- 核心命中：Go 后端、API 集成、webhook 中间件、LLM API 集成、Python 自动化、数据同步/处理管道、FastAPI
- 范围小而清晰（几小时到两周内可交付、交付物可数）：候选人新号最需要这种单
- serverless(Workers/Lambda/Cloud Functions)、Stripe、Shopify、Supabase、ClickHouse：候选人在学，可投但 reason 里注明"需速学 X"
- 正文技术细节具体（有事件名/字段/API 文档链接/screening questions）：真客户概率高

【扣分信号】
- 前端为主的"全栈"（React/Next.js 占比大）——候选人是纯后端
- 文案宏大空泛（strategic/visionary 式）配小预算
- Expert 等级 + 生产环境运维/SLA 责任
- Ongoing 长期合约且每周 30+ 小时硬性要求
- 历史花费 $0 或 $0-$50 区间且预算文案宏大：典型"发帖玩玩"客户
- proposals_bucket 为 20_to_50 或 50_plus：竞争拥挤，-10 分；若同时范围小且核心匹配可免于扣分

【打分标尺】
85-100：完美第一单——核心技能全中+范围小清晰+无否决项
70-84：推荐投标——核心匹配，个别技术需速学或小缺陷
50-69：可看但不值得投——部分匹配
30-49：不匹配
0-29：一票否决，reason 里写明命中哪条

输出：{"score": <int>, "reason": "<≤80字中文：核心匹配点 + 主要风险或否决依据>", "tags": ["<2-5个短标签，如 go/webhook/需速学:stripe/否决:营销岗>"]}`

// BuildScorePrompt 拼装打分 user prompt（标题 + 描述截断 + 预算 + 客户信号），画像在 system 注入。
func BuildScorePrompt(j domain.Job) string {
	desc := []rune(j.Description)
	if len(desc) > maxDescRunes {
		desc = desc[:maxDescRunes]
	}
	prompt := fmt.Sprintf(`【单子信息】
标题：%s
预算：%s
描述：%s
`, j.Title, j.Budget, string(desc))

	// 客户信号：字段为 null 则省略该行
	signals := ""
	if j.PaymentVerified != nil {
		v := "否"
		if *j.PaymentVerified {
			v = "是"
		}
		signals += fmt.Sprintf("支付验证: %s | ", v)
	}
	if j.ClientSpentUSD != nil {
		signals += fmt.Sprintf("历史花费: $%.0f | ", *j.ClientSpentUSD)
	}
	if j.ClientRating != nil {
		signals += fmt.Sprintf("评分: %.1f | ", *j.ClientRating)
	}
	if j.ProposalsBucket != "" {
		signals += fmt.Sprintf("竞争度: %s", j.ProposalsBucket)
	}
	if signals != "" {
		prompt += "\n【客户信号】\n" + strings.TrimSuffix(signals, " | ") + "\n"
	}
	return prompt + "\n请按系统要求的 JSON 格式打分。"
}

// ScoreSystemPrompt 用技能画像实例化 system prompt。
func ScoreSystemPrompt(profile string) string {
	return strings.Replace(scoreSystemPrompt, "{profile}", profile, 1)
}
