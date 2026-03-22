package fetcher

import (
	"fmt"
	"log/slog"

	"github.com/antonmedv/expr"
	"github.com/tidwall/gjson"
)

func (f *Fetcher) transform(body []byte, name string, transform map[string]any) (map[string]any, error) {
	values := make(map[string]any, len(transform)+1)
	values[FieldSource] = name

	for key, value := range transform {
		jsonPath, ok := value.(string)
		if !ok {
			slog.Warn("transform value is not a string, skipping", "key", key, "type", fmt.Sprintf("%T", value))
			continue
		}
		result := gjson.GetBytes(body, jsonPath)
		switch result.Type {
		case gjson.Number:
			values[key] = result.Float()
		case gjson.String:
			values[key] = result.String()
		case gjson.True:
			values[key] = true
		case gjson.False:
			values[key] = false
		case gjson.Null:
			values[key] = nil
		}
	}
	return values, nil
}

func (f *Fetcher) stateChanged(values map[string]any, eventConfig EventConfig) bool {

	if eventConfig.OnChangedFields == nil {
		return true
	}
	f.mutex.RLock()
	lastPayload, exists := f.lastPayload[eventConfig.Name]
	f.mutex.RUnlock()
	if !exists {
		return true
	}
	for _, field := range eventConfig.OnChangedFields {
		if values[field] != lastPayload[field] {
			return true
		}
	}
	return false
}

func (f *Fetcher) publishWhen(values map[string]any, publishWhen string) bool {
	if publishWhen == "" {
		return true
	}

	f.mutex.RLock()
	program, ok := f.exprPrograms[publishWhen]
	f.mutex.RUnlock()
	if ok {
		result, err := expr.Run(program, values)
		if err != nil {
			slog.Error("failed to run publish when", "error", err)
			return false
		}
		resultBool, ok := result.(bool)
		if !ok {
			slog.Error("publish when expression did not return a boolean", "result", result, "type", fmt.Sprintf("%T", result))
			return false
		}
		return resultBool
	}

	program, err := expr.Compile(publishWhen, expr.Env(values))
	if err != nil {
		slog.Error("failed to compile publish when", "error", err)
		return false
	}
	f.mutex.Lock()
	f.exprPrograms[publishWhen] = program
	f.mutex.Unlock()
	result, err := expr.Run(program, values)
	if err != nil {
		slog.Error("failed to run publish when", "error", err)
		return false
	}
	resultBool, ok := result.(bool)
	if !ok {
		slog.Error("publish when expression did not return a boolean", "result", result, "type", fmt.Sprintf("%T", result))
		return false
	}
	return resultBool
}
