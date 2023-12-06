package tool

import (
	"fmt"
	"github.com/alist-org/alist/v3/internal/model"
)

var (
	Tools = make(ToolsManager)
)

type ToolsManager map[string]Tool

func (t ToolsManager) Get(name string) (Tool, error) {
	if tool, ok := t[name]; ok {
		return tool, nil
	}
	return nil, fmt.Errorf("tool %s not found", name)
}

func (t ToolsManager) Add(tool Tool) {
	t[tool.Name()] = tool
}

func (t ToolsManager) Names() []string {
	names := make([]string, 0, len(t))
	for name := range t {
		names = append(names, name)
	}
	return names
}

func (t ToolsManager) Items() []model.SettingItem {
	var items []model.SettingItem
	for _, tool := range t {
		items = append(items, tool.Items()...)
	}
	return items
}
