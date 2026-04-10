package system

/*
这部分写的还不够成熟，下次至少要这样优化：
规则短路/硬拦截 → 
意图与实体识别 → 
结合上下文做路由 → 
看置信度决定执行/澄清/fallback → 
需要知识时再接检索与生成 → 
持续评估和调参。
*/

import "regexp"

// aiIntentProfile 定义当前文件中的核心数据结构或能力抽象。
type aiIntentProfile struct {
	wantsTaskReport      bool
	wantsProgressInsight bool
	wantsDocSupport      bool
	showScope            bool
	showThinkingSummary  bool
}

var (
	aiLightweightPromptRE = regexp.MustCompile(`^(你好(?:呀|啊)?|您好|hello|hi|嗨|哈喽|在吗|在么|早上好|中午好|下午好|晚上好|谢谢|thanks?|thank you|是的|好的|好|ok|okay|嗯|嗯嗯)[!！,.。?？\s]*$`)
	aiTaskReportPromptRE  = regexp.MustCompile(`任务|汇报|日报|周报|进展|联调|闭环`)
	aiProgressPromptRE    = regexp.MustCompile(`进度|刷题|训练|排名|节奏|建议|最近.*天`)
	aiDocPromptRE         = regexp.MustCompile(`文档|README|架构|页面定位|接入|UI|改造说明|说明`)
	aiScopePromptRE       = regexp.MustCompile(`范围|scope|当前用户|当前组织|白名单|跨组织|跨用户|文档范围`)
)

// isLightweightAIPrompt 负责执行当前函数对应的核心逻辑。
// 参数：
//   - input：当前阶段输入对象。
//
// 返回值：
//   - bool：表示当前操作是否成功、命中或可继续执行。
//
// 核心流程：
//  1. 根据当前输入整理本函数需要的上下文、默认值或依赖。
//  2. 执行该函数对应的核心职责，并把结果传递给下一层或调用方。
//
// 注意事项：
//   - 具体细节需结合函数体与调用方一起理解；当前注释基于函数命名和上下文整理。
func isLightweightAIPrompt(input string) bool {
	return aiLightweightPromptRE.MatchString(input)
}

// detectAIIntent 负责执行当前函数对应的核心逻辑。
// 参数：
//   - input：当前阶段输入对象。
//
// 返回值：
//   - aiIntentProfile：当前函数返回的处理结果。
//
// 核心流程：
//  1. 根据当前输入整理本函数需要的上下文、默认值或依赖。
//  2. 执行该函数对应的核心职责，并把结果传递给下一层或调用方。
//
// 注意事项：
//   - 具体细节需结合函数体与调用方一起理解；当前注释基于函数命名和上下文整理。
func detectAIIntent(input string) aiIntentProfile {
	wantsTaskReport := aiTaskReportPromptRE.MatchString(input)
	wantsProgressInsight := aiProgressPromptRE.MatchString(input)
	wantsDocSupport := aiDocPromptRE.MatchString(input)
	showScope := aiScopePromptRE.MatchString(input)
	showThinkingSummary := wantsTaskReport || wantsProgressInsight || wantsDocSupport || len([]rune(input)) >= 16
	return aiIntentProfile{
		wantsTaskReport:      wantsTaskReport,
		wantsProgressInsight: wantsProgressInsight,
		wantsDocSupport:      wantsDocSupport,
		showScope:            showScope,
		showThinkingSummary:  showThinkingSummary,
	}
}
