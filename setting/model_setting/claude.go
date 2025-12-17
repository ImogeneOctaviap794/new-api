package model_setting

import (
	"net/http"

	"github.com/QuantumNous/new-api/setting/config"
)

type CacheControlInjectionPoint struct {
	Location string `json:"location"`
	Role     string `json:"role,omitempty"`
	Index    *int   `json:"index,omitempty"`
	Ttl      string `json:"ttl,omitempty"`
}

//var claudeHeadersSettings = map[string][]string{}
//
//var ClaudeThinkingAdapterEnabled = true
//var ClaudeThinkingAdapterMaxTokens = 8192
//var ClaudeThinkingAdapterBudgetTokensPercentage = 0.8

// ClaudeSettings 定义Claude模型的配置
type ClaudeSettings struct {
	HeadersSettings                       map[string]map[string][]string `json:"model_headers_settings"`
	CacheControlInjectionPoints           map[string][]CacheControlInjectionPoint `json:"cache_control_injection_points"`
	DefaultMaxTokens                      map[string]int                 `json:"default_max_tokens"`
	ThinkingAdapterEnabled                bool                           `json:"thinking_adapter_enabled"`
	ThinkingAdapterBudgetTokensPercentage float64                        `json:"thinking_adapter_budget_tokens_percentage"`
}

// 默认配置
var defaultClaudeSettings = ClaudeSettings{
	HeadersSettings:             map[string]map[string][]string{},
	CacheControlInjectionPoints: map[string][]CacheControlInjectionPoint{},
	ThinkingAdapterEnabled:      true,
	DefaultMaxTokens: map[string]int{
		"default": 8192,
	},
	ThinkingAdapterBudgetTokensPercentage: 0.8,
}

// 全局实例
var claudeSettings = defaultClaudeSettings

func init() {
	// 注册到全局配置管理器
	config.GlobalConfig.Register("claude", &claudeSettings)
}

// GetClaudeSettings 获取Claude配置
func GetClaudeSettings() *ClaudeSettings {
	// check default max tokens must have default key
	if _, ok := claudeSettings.DefaultMaxTokens["default"]; !ok {
		claudeSettings.DefaultMaxTokens["default"] = 8192
	}
	if claudeSettings.CacheControlInjectionPoints == nil {
		claudeSettings.CacheControlInjectionPoints = map[string][]CacheControlInjectionPoint{}
	}
	return &claudeSettings
}

func (c *ClaudeSettings) GetCacheControlInjectionPoints(model string) []CacheControlInjectionPoint {
	if c.CacheControlInjectionPoints == nil {
		return nil
	}
	if points, ok := c.CacheControlInjectionPoints[model]; ok {
		return points
	}
	if points, ok := c.CacheControlInjectionPoints["default"]; ok {
		return points
	}
	return nil
}

func (c *ClaudeSettings) WriteHeaders(originModel string, httpHeader *http.Header) {
	if headers, ok := c.HeadersSettings[originModel]; ok {
		for headerKey, headerValues := range headers {
			// get existing values for this header key
			existingValues := httpHeader.Values(headerKey)
			existingValuesMap := make(map[string]bool)
			for _, v := range existingValues {
				existingValuesMap[v] = true
			}

			// add only values that don't already exist
			for _, headerValue := range headerValues {
				if !existingValuesMap[headerValue] {
					httpHeader.Add(headerKey, headerValue)
				}
			}
		}
	}
}

func (c *ClaudeSettings) GetDefaultMaxTokens(model string) int {
	if maxTokens, ok := c.DefaultMaxTokens[model]; ok {
		return maxTokens
	}
	return c.DefaultMaxTokens["default"]
}
