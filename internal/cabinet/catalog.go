package cabinet

import (
	"sort"
	"strings"
	"time"
)

func (c *Cabinet) Catalog() []Tool {
	c.mu.Lock()
	defer c.mu.Unlock()
	result := make([]Tool, 0, len(c.tools))
	for id, state := range c.tools {
		result = append(result, Tool{ID: id, Name: displayName(id), Category: category(id), Location: "社区服务站 A 区", State: state, Condition: "ready", UpdatedAt: time.Now()})
	}
	sort.Slice(result, func(i, j int) bool { return result[i].ID < result[j].ID })
	return result
}
func (c *Cabinet) Find(query string) []Tool {
	query = strings.ToLower(strings.TrimSpace(query))
	result := c.Catalog()
	if query == "" {
		return result
	}
	filtered := result[:0]
	for _, tool := range result {
		if strings.Contains(strings.ToLower(tool.ID+tool.Name+tool.Category), query) {
			filtered = append(filtered, tool)
		}
	}
	return filtered
}
func displayName(id string) string {
	switch id {
	case "drill-01":
		return "家用电钻"
	case "ladder-01":
		return "折叠梯"
	default:
		return "共享工具"
	}
}
func category(id string) string {
	if strings.Contains(id, "drill") {
		return "电动工具"
	}
	return "家居工具"
}
