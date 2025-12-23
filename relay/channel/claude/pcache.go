package claude

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/service"
)

// ========== Constants ==========

const (
	PCacheKeyPrefix           = "pcache"
	PCacheFuzzyPrefix         = "pcache_fuzzy" // 模糊匹配前缀
	TTL5m                     = 5 * time.Minute
	TTL1h                     = 1 * time.Hour
	MaxBreakpoints            = 4
	LookbackWindow            = 20
	// TODO: 测试完成后恢复: Default=1024, Opus=4096, Sonnet=1024, Haiku=2048
	MinCacheableTokensDefault = 5
	MinCacheableTokensOpus    = 5
	MinCacheableTokensSonnet  = 5
	MinCacheableTokensHaiku   = 5
)

// PCacheEnabled controls whether local prompt cache simulation is enabled
var PCacheEnabled = false

// PCacheTargetModels lists models that support local prompt cache simulation
var PCacheTargetModels = []string{
	"claude-sonnet-4-5",
	"claude-opus-4-5",
}

// InitPCache initializes the prompt cache module from environment variables
// Environment variables:
//   - PCACHE_ENABLED: "true" to enable local prompt cache simulation
//   - PCACHE_TARGET_MODELS: comma-separated list of target models (optional)
func InitPCache() {
	if os.Getenv("PCACHE_ENABLED") == "true" {
		PCacheEnabled = true
		common.SysLog("pcache: local prompt cache simulation enabled")
	}

	if models := os.Getenv("PCACHE_TARGET_MODELS"); models != "" {
		PCacheTargetModels = strings.Split(models, ",")
		for i := range PCacheTargetModels {
			PCacheTargetModels[i] = strings.TrimSpace(PCacheTargetModels[i])
		}
		common.SysLog(fmt.Sprintf("pcache: target models set to %v", PCacheTargetModels))
	}
}

// IsPCacheTargetModel checks if a model is eligible for local prompt cache
func IsPCacheTargetModel(model string) bool {
	if !PCacheEnabled {
		return false
	}
	modelLower := strings.ToLower(model)
	for _, target := range PCacheTargetModels {
		if strings.Contains(modelLower, strings.ToLower(target)) {
			return true
		}
	}
	return false
}

// ========== Data Structures ==========

type CacheBreakpoint struct {
	Location   string `json:"location"`
	Index      int    `json:"index"`
	BlockIndex int    `json:"block_index"`
	TTL        string `json:"ttl"`
	TokenCount int    `json:"token_count"`
}

type CachePrefix struct {
	Tools    []json.RawMessage `json:"tools,omitempty"`
	System   []json.RawMessage `json:"system,omitempty"`
	Messages []json.RawMessage `json:"messages,omitempty"`
}

type CacheEntry struct {
	Hash       string    `json:"hash"`
	TokenCount int       `json:"token_count"`
	TTL        string    `json:"ttl"`
	CreatedAt  time.Time `json:"created_at"`
	ExpiresAt  time.Time `json:"expires_at"`
	Model      string    `json:"model"`
}

type CacheResult struct {
	Hit                 bool
	CacheReadTokens     int
	CacheCreationTokens int
	CacheCreation5m     int
	CacheCreation1h     int
	InputTokens         int
	BreakpointHits      []bool
}

// ========== Hash Calculation ==========

func CalculatePrefixHash(prefix *CachePrefix, model string) string {
	data := map[string]interface{}{"model": model}
	if len(prefix.Tools) > 0 {
		data["tools"] = prefix.Tools
	}
	if len(prefix.System) > 0 {
		data["system"] = prefix.System
	}
	if len(prefix.Messages) > 0 {
		data["messages"] = prefix.Messages
	}
	jsonBytes, _ := json.Marshal(data)
	hash := sha256.Sum256(jsonBytes)
	return hex.EncodeToString(hash[:])
}

// ========== Prefix Extraction ==========

// ExtractCacheBreakpointsWithTotal 返回 breakpoints、总的本地 tokens、tools tokens
func ExtractCacheBreakpointsWithTotal(req *dto.ClaudeRequest, model string) ([]CacheBreakpoint, int, int) {
	breakpoints := make([]CacheBreakpoint, 0, MaxBreakpoints)
	globalBlockIndex := 0
	totalTokens := 0
	toolsTokens := 0

	// 1. Process tools - 只计算 tokens，不创建缓存 breakpoint，不计入缓存
	if req.Tools != nil {
		tools := req.GetTools()
		for _, tool := range tools {
			toolBytes, _ := json.Marshal(tool)
			tokens := estimateContentTokens(toolBytes, model)
			toolsTokens += tokens
			totalTokens += tokens
			globalBlockIndex++
		}
	}

	// 缓存 tokens 从这里开始计算（不包含 tools）
	cacheableTokens := 0

	// 2. Process system - 只有带 cache_control 标记的才创建 breakpoint
	if req.System != nil && !req.IsStringSystem() {
		systemMedia := req.ParseSystem()
		for i, media := range systemMedia {
			mediaBytes, _ := json.Marshal(media)
			tokens := estimateContentTokens(mediaBytes, model)
			cacheableTokens += tokens
			totalTokens += tokens
			globalBlockIndex++
			if len(media.CacheControl) > 0 {
				if ttl := extractCacheControlTTLFromRaw(media.CacheControl); ttl != "" {
					breakpoints = append(breakpoints, CacheBreakpoint{
						Location: "system", Index: i, BlockIndex: globalBlockIndex,
						TTL: ttl, TokenCount: cacheableTokens, // 只计算 system+messages，不含 tools
					})
				}
			}
		}
	} else if req.System != nil {
		tokens := estimateTokens(req.GetStringSystem(), model)
		cacheableTokens += tokens
		totalTokens += tokens
		globalBlockIndex++
	}

	// 3. Process messages - 只有带 cache_control 标记的才创建 breakpoint
	for i, msg := range req.Messages {
		msgBytes, _ := json.Marshal(msg)
		tokens := estimateContentTokens(msgBytes, model)
		cacheableTokens += tokens
		totalTokens += tokens
		globalBlockIndex++
		if ttl := extractCacheControlFromMessage(&msg); ttl != "" {
			breakpoints = append(breakpoints, CacheBreakpoint{
				Location: "message", Index: i, BlockIndex: globalBlockIndex,
				TTL: ttl, TokenCount: cacheableTokens, // 只计算 system+messages，不含 tools
			})
		}
	}

	if len(breakpoints) > MaxBreakpoints {
		breakpoints = breakpoints[:MaxBreakpoints]
	}

	return breakpoints, totalTokens, toolsTokens
}

func ExtractCacheBreakpoints(req *dto.ClaudeRequest, model string) []CacheBreakpoint {
	breakpoints, _, _ := ExtractCacheBreakpointsWithTotal(req, model)
	return breakpoints
}

func extractCacheControlTTL(data []byte) string {
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(data, &obj); err != nil {
		return ""
	}
	if cc, ok := obj["cache_control"]; ok {
		return extractCacheControlTTLFromRaw(cc)
	}
	return ""
}

func extractCacheControlTTLFromRaw(cacheControl json.RawMessage) string {
	if len(cacheControl) == 0 {
		return ""
	}
	var cc struct {
		Type string `json:"type"`
		TTL  string `json:"ttl"`
	}
	if err := json.Unmarshal(cacheControl, &cc); err != nil {
		return ""
	}
	if cc.Type != "ephemeral" {
		return ""
	}
	if cc.TTL == "" {
		return "5m"
	}
	return cc.TTL
}

func extractCacheControlFromMessage(msg *dto.ClaudeMessage) string {
	if msg.IsStringContent() {
		return ""
	}
	content, _ := msg.ParseContent()
	for _, media := range content {
		if len(media.CacheControl) > 0 {
			return extractCacheControlTTLFromRaw(media.CacheControl)
		}
	}
	return ""
}

// ========== Prefix Building ==========

func BuildPrefixUpToBreakpoint(req *dto.ClaudeRequest, bp CacheBreakpoint) *CachePrefix {
	prefix := &CachePrefix{}

	if req.Tools != nil {
		tools := req.GetTools()
		limit := len(tools)
		if bp.Location == "tools" {
			limit = bp.Index + 1
		}
		for i := 0; i < limit && i < len(tools); i++ {
			toolBytes, _ := json.Marshal(tools[i])
			prefix.Tools = append(prefix.Tools, toolBytes)
		}
	}

	if bp.Location != "tools" && req.System != nil {
		if req.IsStringSystem() {
			sysBytes, _ := json.Marshal(req.GetStringSystem())
			prefix.System = append(prefix.System, sysBytes)
		} else {
			systemMedia := req.ParseSystem()
			limit := len(systemMedia)
			if bp.Location == "system" {
				limit = bp.Index + 1
			}
			for i := 0; i < limit && i < len(systemMedia); i++ {
				mediaBytes, _ := json.Marshal(systemMedia[i])
				prefix.System = append(prefix.System, mediaBytes)
			}
		}
	}

	if bp.Location == "message" {
		for i := 0; i <= bp.Index && i < len(req.Messages); i++ {
			msgBytes, _ := json.Marshal(req.Messages[i])
			prefix.Messages = append(prefix.Messages, msgBytes)
		}
	}

	return prefix
}

// ========== Token Estimation ==========

func estimateTokens(text string, model string) int {
	// 使用 tiktoken 计算
	return service.CountTextToken(text, model)
}

// extractTextContent 从 JSON 中提取实际文本内容，减少 JSON 结构的影响
func extractTextContent(data []byte) string {
	var content strings.Builder

	// 尝试解析为 map
	var obj map[string]interface{}
	if err := json.Unmarshal(data, &obj); err == nil {
		extractTextFromMap(obj, &content)
		return content.String()
	}

	// 尝试解析为数组
	var arr []interface{}
	if err := json.Unmarshal(data, &arr); err == nil {
		for _, item := range arr {
			if m, ok := item.(map[string]interface{}); ok {
				extractTextFromMap(m, &content)
			}
		}
		return content.String()
	}

	return string(data)
}

func extractTextFromMap(obj map[string]interface{}, content *strings.Builder) {
	// 提取常见的文本字段
	textFields := []string{"text", "content", "description", "name", "title"}
	for _, field := range textFields {
		if v, ok := obj[field]; ok {
			switch val := v.(type) {
			case string:
				content.WriteString(val)
				content.WriteString(" ")
			case []interface{}:
				for _, item := range val {
					if s, ok := item.(string); ok {
						content.WriteString(s)
						content.WriteString(" ")
					} else if m, ok := item.(map[string]interface{}); ok {
						extractTextFromMap(m, content)
					}
				}
			}
		}
	}

	// 递归处理嵌套对象
	for key, v := range obj {
		if m, ok := v.(map[string]interface{}); ok {
			// 跳过 schema 等结构性字段
			if key != "input_schema" && key != "properties" && key != "required" {
				extractTextFromMap(m, content)
			}
		}
	}
}

// estimateContentTokens 只计算实际内容的 tokens，减少 JSON 膨胀
func estimateContentTokens(data []byte, model string) int {
	text := extractTextContent(data)
	if text == "" {
		return 0
	}
	return service.CountTextToken(text, model)
}

func GetMinCacheableTokens(model string) int {
	modelLower := strings.ToLower(model)
	if strings.Contains(modelLower, "opus") {
		return MinCacheableTokensOpus
	}
	if strings.Contains(modelLower, "sonnet") {
		return MinCacheableTokensSonnet
	}
	if strings.Contains(modelLower, "haiku") {
		return MinCacheableTokensHaiku
	}
	return MinCacheableTokensDefault
}

// ========== Redis Operations ==========

func GetCacheKey(model string, hash string) string {
	return fmt.Sprintf("%s:%s:%s", PCacheKeyPrefix, model, hash)
}

// GetFuzzyCacheKey 返回基于 system prompt 的模糊缓存键
func GetFuzzyCacheKey(model string, systemHash string) string {
	return fmt.Sprintf("%s:%s:%s", PCacheFuzzyPrefix, model, systemHash)
}

// CalculateSystemHash 计算 system prompt 的哈希（用于模糊匹配）
func CalculateSystemHash(req *dto.ClaudeRequest, model string) string {
	if req.System == nil {
		return ""
	}
	var systemBytes []byte
	if req.IsStringSystem() {
		systemBytes, _ = json.Marshal(req.GetStringSystem())
	} else {
		systemMedia := req.ParseSystem()
		systemBytes, _ = json.Marshal(systemMedia)
	}
	data := map[string]interface{}{"model": model, "system": systemBytes}
	jsonBytes, _ := json.Marshal(data)
	hash := sha256.Sum256(jsonBytes)
	return hex.EncodeToString(hash[:])
}

// StoreFuzzyEntry 存储模糊匹配条目（基于 system prompt）
func StoreFuzzyEntry(model string, systemHash string, tokenCount int, ttl string) error {
	if !common.RedisEnabled || systemHash == "" {
		return nil
	}
	key := GetFuzzyCacheKey(model, systemHash)
	expiry := TTL5m
	if ttl == "1h" {
		expiry = TTL1h
	}
	data := map[string]interface{}{"tokens": tokenCount, "ttl": ttl}
	jsonBytes, _ := json.Marshal(data)
	return common.RedisSet(key, string(jsonBytes), expiry)
}

// GetFuzzyEntry 检查模糊匹配条目是否存在
func GetFuzzyEntry(model string, systemHash string) (int, bool) {
	if !common.RedisEnabled || systemHash == "" {
		return 0, false
	}
	key := GetFuzzyCacheKey(model, systemHash)
	data, err := common.RedisGet(key)
	if err != nil || data == "" {
		return 0, false
	}
	var entry map[string]interface{}
	if err := json.Unmarshal([]byte(data), &entry); err != nil {
		return 0, false
	}
	if tokens, ok := entry["tokens"].(float64); ok {
		return int(tokens), true
	}
	return 0, false
}

func StoreCacheEntry(entry *CacheEntry) error {
	if !common.RedisEnabled {
		return nil // Redis not enabled, skip silently
	}
	key := GetCacheKey(entry.Model, entry.Hash)
	ttl := TTL5m
	if entry.TTL == "1h" {
		ttl = TTL1h
	}
	entry.CreatedAt = time.Now()
	entry.ExpiresAt = entry.CreatedAt.Add(ttl)
	data, err := json.Marshal(entry)
	if err != nil {
		return err
	}
	return common.RedisSet(key, string(data), ttl)
}

func GetCacheEntry(model string, hash string) (*CacheEntry, error) {
	if !common.RedisEnabled {
		return nil, nil // Redis not enabled, return cache miss
	}
	key := GetCacheKey(model, hash)
	data, err := common.RedisGet(key)
	if err != nil {
		return nil, err
	}
	if data == "" {
		return nil, nil
	}
	var entry CacheEntry
	if err := json.Unmarshal([]byte(data), &entry); err != nil {
		return nil, err
	}
	return &entry, nil
}

// ========== Cache Hit Detection ==========

// CheckCacheHits checks which breakpoints have cache hits and returns the result
// Implements Anthropic's cache hit rules:
// 1. Cache entries are checked from longest prefix to shortest
// 2. Once a hit is found, all shorter prefixes are considered hits
// 3. 20-block lookback window is applied
func CheckCacheHits(req *dto.ClaudeRequest, model string, totalInputTokens int) *CacheResult {
	breakpoints := ExtractCacheBreakpoints(req, model)
	result := &CacheResult{
		BreakpointHits: make([]bool, len(breakpoints)),
		InputTokens:    totalInputTokens,
	}

	if len(breakpoints) == 0 {
		return result
	}

	minTokens := GetMinCacheableTokens(model)
	lastHitIndex := -1
	lastHitTokens := 0

	// Check breakpoints from longest to shortest (reverse order)
	for i := len(breakpoints) - 1; i >= 0; i-- {
		bp := breakpoints[i]

		// Skip if below minimum cacheable threshold
		if bp.TokenCount < minTokens {
			continue
		}

		prefix := BuildPrefixUpToBreakpoint(req, bp)
		hash := CalculatePrefixHash(prefix, model)

		entry, err := GetCacheEntry(model, hash)
		if err == nil && entry != nil {
			// Cache hit found
			result.Hit = true
			result.BreakpointHits[i] = true
			lastHitIndex = i
			lastHitTokens = bp.TokenCount

			// All shorter prefixes are also considered hits
			for j := 0; j < i; j++ {
				result.BreakpointHits[j] = true
			}
			break
		}
	}

	// Calculate cache read/write tokens
	if result.Hit {
		result.CacheReadTokens = lastHitTokens

		// Check for any breakpoints after the hit that need cache creation
		for i := lastHitIndex + 1; i < len(breakpoints); i++ {
			bp := breakpoints[i]
			if bp.TokenCount >= minTokens {
				creationTokens := bp.TokenCount - lastHitTokens
				if bp.TTL == "1h" {
					result.CacheCreation1h += creationTokens
				} else {
					result.CacheCreation5m += creationTokens
				}
				result.CacheCreationTokens += creationTokens
				lastHitTokens = bp.TokenCount
			}
		}

		// Remaining input tokens after last breakpoint
		if len(breakpoints) > 0 {
			lastBp := breakpoints[len(breakpoints)-1]
			result.InputTokens = totalInputTokens - lastBp.TokenCount
			if result.InputTokens < 0 {
				result.InputTokens = 0
			}
		}
	} else {
		// No cache hit - 所有 breakpoints 需要创建缓存
		prevTokens := 0
		for _, bp := range breakpoints {
			if bp.TokenCount >= minTokens {
				creationTokens := bp.TokenCount - prevTokens
				if bp.TTL == "1h" {
					result.CacheCreation1h += creationTokens
				} else {
					result.CacheCreation5m += creationTokens
				}
				result.CacheCreationTokens += creationTokens
				prevTokens = bp.TokenCount
			}
		}

		// 剩余非缓存 tokens
		if len(breakpoints) > 0 {
			lastBp := breakpoints[len(breakpoints)-1]
			result.InputTokens = totalInputTokens - lastBp.TokenCount
			if result.InputTokens < 0 {
				result.InputTokens = 0
			}
		}
	}

	return result
}

// StoreCacheBreakpoints stores all cache breakpoints to Redis
func StoreCacheBreakpoints(req *dto.ClaudeRequest, model string) error {
	breakpoints := ExtractCacheBreakpoints(req, model)
	minTokens := GetMinCacheableTokens(model)

	for _, bp := range breakpoints {
		if bp.TokenCount < minTokens {
			continue
		}

		prefix := BuildPrefixUpToBreakpoint(req, bp)
		hash := CalculatePrefixHash(prefix, model)

		entry := &CacheEntry{
			Hash:       hash,
			TokenCount: bp.TokenCount,
			TTL:        bp.TTL,
			Model:      model,
		}

		if err := StoreCacheEntry(entry); err != nil {
			common.SysLog(fmt.Sprintf("pcache: failed to store cache entry: %v", err))
		}
	}

	return nil
}

// ========== Main Entry Point ==========

// ProcessPromptCache is the main entry point for prompt cache processing
// It checks for cache hits, calculates tokens, and stores new cache entries
func ProcessPromptCache(req *dto.ClaudeRequest, model string, totalInputTokens int) *CacheResult {
	result := CheckCacheHits(req, model, totalInputTokens)

	// Store cache entries for future requests (async to not block response)
	go func() {
		if err := StoreCacheBreakpoints(req, model); err != nil {
			common.SysLog(fmt.Sprintf("pcache: failed to store breakpoints: %v", err))
		}
	}()

	return result
}

// applyLocalCacheSimulation applies local prompt cache simulation to usage
// This function uses fully local token calculation, not upstream values
// 策略：多判命中少判创建 - 使用模糊匹配优先判定为命中
func applyLocalCacheSimulation(req *dto.ClaudeRequest, model string, usage *dto.Usage) {
	if req == nil || usage == nil {
		return
	}

	// 完全用本地计算 tokens
	breakpoints, localTotalTokens, toolsTokens := ExtractCacheBreakpointsWithTotal(req, model)
	if localTotalTokens == 0 {
		return
	}

	// 可缓存的 tokens（不含 tools）
	cacheableTotalTokens := localTotalTokens - toolsTokens

	minTokens := GetMinCacheableTokens(model)

	// 计算缓存 tokens
	var cacheReadTokens, cacheCreateTokens, cacheCreate5m, cacheCreate1h int
	var lastCacheTokens int
	var exactHit bool

	// 1. 先尝试精确匹配
	for i := len(breakpoints) - 1; i >= 0; i-- {
		bp := breakpoints[i]
		if bp.TokenCount < minTokens {
			continue
		}

		prefix := BuildPrefixUpToBreakpoint(req, bp)
		hash := CalculatePrefixHash(prefix, model)
		entry, err := GetCacheEntry(model, hash)

		if err == nil && entry != nil {
			// 精确命中
			exactHit = true
			cacheReadTokens = bp.TokenCount
			lastCacheTokens = bp.TokenCount

			// 检查命中点之后是否有新的缓存需要创建
			for j := i + 1; j < len(breakpoints); j++ {
				nextBp := breakpoints[j]
				if nextBp.TokenCount >= minTokens {
					createTokens := nextBp.TokenCount - lastCacheTokens
					if nextBp.TTL == "1h" {
						cacheCreate1h += createTokens
					} else {
						cacheCreate5m += createTokens
					}
					cacheCreateTokens += createTokens
					lastCacheTokens = nextBp.TokenCount
				}
			}
			break
		}
	}

	// 2. 如果精确匹配失败，尝试模糊匹配（基于 system prompt）
	// 策略：如果 system prompt 相同，假设 Anthropic 端可能命中
	if !exactHit && len(breakpoints) > 0 {
		systemHash := CalculateSystemHash(req, model)
		if fuzzyTokens, found := GetFuzzyEntry(model, systemHash); found {
			// 模糊命中：假设第一个断点命中
			// 使用之前存储的 token 数作为读取量
			if len(breakpoints) > 0 && breakpoints[0].TokenCount >= minTokens {
				cacheReadTokens = fuzzyTokens
				if cacheReadTokens > breakpoints[0].TokenCount {
					cacheReadTokens = breakpoints[0].TokenCount
				}
				lastCacheTokens = cacheReadTokens

				// 后续断点需要创建
				for _, bp := range breakpoints {
					if bp.TokenCount > lastCacheTokens && bp.TokenCount >= minTokens {
						createTokens := bp.TokenCount - lastCacheTokens
						if bp.TTL == "1h" {
							cacheCreate1h += createTokens
						} else {
							cacheCreate5m += createTokens
						}
						cacheCreateTokens += createTokens
						lastCacheTokens = bp.TokenCount
					}
				}
			}
		}
	}

	// 3. 如果都没有命中，所有 breakpoints 都需要创建缓存
	if cacheReadTokens == 0 && len(breakpoints) > 0 {
		prevTokens := 0
		for _, bp := range breakpoints {
			if bp.TokenCount >= minTokens {
				createTokens := bp.TokenCount - prevTokens
				if bp.TTL == "1h" {
					cacheCreate1h += createTokens
				} else {
					cacheCreate5m += createTokens
				}
				cacheCreateTokens += createTokens
				prevTokens = bp.TokenCount
				lastCacheTokens = bp.TokenCount
			}
		}

		// 存储模糊匹配条目，供后续请求使用
		if len(breakpoints) > 0 {
			systemHash := CalculateSystemHash(req, model)
			go StoreFuzzyEntry(model, systemHash, breakpoints[0].TokenCount, breakpoints[0].TTL)
		}
	}

	// 非缓存的输入 tokens = tools + (可缓存 tokens - 最后一个缓存断点的 tokens)
	// tools 不缓存，所以算作非缓存输入
	nonCacheTokens := toolsTokens + (cacheableTotalTokens - lastCacheTokens)
	if nonCacheTokens < 0 {
		nonCacheTokens = 0
	}

	// 如果没有缓存活动，不覆盖上游值
	if cacheReadTokens == 0 && cacheCreateTokens == 0 {
		return
	}

	// 存储缓存断点
	go StoreCacheBreakpoints(req, model)

	// 覆盖缓存相关字段
	usage.PromptTokensDetails.CachedTokens = cacheReadTokens
	usage.PromptTokensDetails.CachedCreationTokens = cacheCreateTokens
	usage.ClaudeCacheCreation5mTokens = cacheCreate5m
	usage.ClaudeCacheCreation1hTokens = cacheCreate1h

	// 只有当 nonCacheTokens > 0 时才覆盖 PromptTokens，否则保留上游值
	if nonCacheTokens > 0 {
		usage.PromptTokens = nonCacheTokens
	}
}
