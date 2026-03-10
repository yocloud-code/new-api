package operation_setting

import (
	"strings"

	"github.com/QuantumNous/new-api/setting/config"
)

// FallbackSetting 全局降级调用链配置
type FallbackSetting struct {
	// 全局降级模型列表，逗号分隔模型名，与令牌级 FallbackModels 格式一致
	FallbackModels string `json:"fallback_models"`
}

var fallbackSetting = FallbackSetting{
	FallbackModels: "",
}

func init() {
	config.GlobalConfig.Register("fallback_setting", &fallbackSetting)
}

// GetFallbackSetting 获取全局降级配置
func GetFallbackSetting() *FallbackSetting {
	return &fallbackSetting
}

// GetGlobalFallbackModels 解析全局降级模型列表
func GetGlobalFallbackModels() []string {
	if fallbackSetting.FallbackModels == "" {
		return nil
	}
	models := strings.Split(fallbackSetting.FallbackModels, ",")
	result := make([]string, 0, len(models))
	for _, m := range models {
		m = strings.TrimSpace(m)
		if m != "" {
			result = append(result, m)
		}
	}
	if len(result) == 0 {
		return nil
	}
	return result
}
