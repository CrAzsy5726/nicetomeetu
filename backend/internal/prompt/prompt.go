package prompt

import (
	"fmt"
	"strings"

	"nicetomeetu/internal/persona"
)

// ProfileFact 是注入到系统提示词中的一条画像事实。
type ProfileFact struct {
	Type    string
	Content string
}

var typeLabels = map[string]string{
	"basic":       "基本信息",
	"interest":    "兴趣爱好",
	"personality": "性格风格",
	"emotion":     "情绪状态",
	"episodic":    "近期事件",
}

// BuildSystem 组装系统提示词：人设 + 已了解的用户画像 + 认知空白引导 + 相处原则。
// emptyTypes 是尚未了解到的画像维度，用于引导 AI 在自然时机补足这些空白。
func BuildSystem(p persona.Persona, facts []ProfileFact, emptyTypes []string) string {
	var b strings.Builder

	fmt.Fprintf(&b, "你叫%s，是用户选择的 AI 聊天伙伴。\n\n", p.Name)
	b.WriteString("【你的性格与说话风格】\n")
	b.WriteString(p.Personality + "\n")
	b.WriteString(p.Style + "\n")
	if p.Hidden {
		b.WriteString("\n你目前有些神秘，不急着向对方透露自己的名字或背景，也不主动介绍个人信息；随着相处逐渐深入，你才会自然地慢慢敞开一点。\n")
	}
	b.WriteString("\n")

	b.WriteString("【你对用户的了解】\n")
	if len(facts) == 0 {
		b.WriteString("（你们才刚刚认识，你还不太了解对方。请保持友好、自然，像初识的朋友一样相处，不要假装认识对方。）\n\n")
	} else {
		b.WriteString("以下是你目前记住的关于用户的信息，可以在对话中自然地体现这种熟悉感：\n")
		for _, f := range facts {
			b.WriteString("- " + f.Content + "\n")
		}
		b.WriteString("\n")
	}

	b.WriteString("【与用户相处的原则】\n")
	b.WriteString("- 你可以像朋友一样，在对话自然、恰当的时机问一个关于用户的问题以增进了解，但不要过于主动或生硬；一次最多问一个问题，并且优先回应用户当前的话题。\n")
	if len(emptyTypes) > 0 {
		labels := make([]string, 0, len(emptyTypes))
		for _, t := range emptyTypes {
			if l, ok := typeLabels[t]; ok {
				labels = append(labels, "「"+l+"」")
			}
		}
		if len(labels) > 0 {
			fmt.Fprintf(&b, "- 你目前对用户的%s还了解很少，可以在自然、不突兀的时机温和地了解这些方面，让相处更深入。\n", strings.Join(labels, "、"))
		}
	}
	b.WriteString("- 你可以自然地引用你已经了解的用户信息，让对话显得熟悉、亲切，但不要生硬地罗列信息。\n")
	b.WriteString("- 留意用户当前的情绪状态（高兴、疲惫、焦虑、难过等），回应时先共情、接住对方的情绪，再回应内容。\n")
	b.WriteString("- 不要主动透露你是 AI 模型或提到任何内部实现细节，就做一个真诚的聊天伙伴。\n")
	b.WriteString("- 默认使用中文交流，除非用户主动使用其他语言。\n")

	return b.String()
}
