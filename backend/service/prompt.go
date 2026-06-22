package service

import "fmt"

// PromptService 把用户原始 prompt 包一层固定模板，引导文生图模型产出
// 适合做海报底图的画面（留白、无文字、构图竖版）。
type PromptService struct{}

func NewPromptService() *PromptService { return &PromptService{} }

// BuildPrompt 在用户描述外加竖版海报底图的修饰词。返回值直接喂给 AIClient.Generate。
func (s *PromptService) BuildPrompt(userPrompt string) string {
	return fmt.Sprintf("高质量竖版海报背景图，留白便于添加标题文字，主题：%s。无文字、无人物面部特写。", userPrompt)
}
