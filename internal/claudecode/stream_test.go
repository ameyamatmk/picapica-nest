package claudecode

import (
	"testing"
)

func TestParseLine_ToolUse(t *testing.T) {
	line := []byte(`{"type":"assistant","message":{"role":"assistant","content":[{"type":"text","text":"検索します"},{"type":"tool_use","id":"tu_1","name":"WebSearch","input":{"query":"test"}}]}}`)

	event, ok := parseLine(line)
	if !ok {
		t.Fatal("expected ok=true for tool_use line")
	}
	if event.Type != "tool_use" {
		t.Errorf("expected Type=tool_use, got %q", event.Type)
	}
	if event.ToolName != "WebSearch" {
		t.Errorf("expected ToolName=WebSearch, got %q", event.ToolName)
	}
}

func TestParseLine_Result(t *testing.T) {
	line := []byte(`{"type":"result","subtype":"success","result":"検索結果のテキスト","total_cost_usd":0.01,"duration_ms":5000}`)

	event, ok := parseLine(line)
	if !ok {
		t.Fatal("expected ok=true for result line")
	}
	if event.Type != "result" {
		t.Errorf("expected Type=result, got %q", event.Type)
	}
	if event.Content != "検索結果のテキスト" {
		t.Errorf("expected Content=検索結果のテキスト, got %q", event.Content)
	}
}

func TestParseLine_SystemInit(t *testing.T) {
	line := []byte(`{"type":"system","subtype":"init","session_id":"sess_123","tools":["WebSearch"]}`)

	_, ok := parseLine(line)
	if ok {
		t.Error("expected ok=false for system init line")
	}
}

func TestParseLine_AssistantTextOnly(t *testing.T) {
	line := []byte(`{"type":"assistant","message":{"role":"assistant","content":[{"type":"text","text":"テキストのみ"}]}}`)

	_, ok := parseLine(line)
	if ok {
		t.Error("expected ok=false for text-only assistant line")
	}
}

func TestParseLine_InvalidJSON(t *testing.T) {
	line := []byte(`not json at all`)

	_, ok := parseLine(line)
	if ok {
		t.Error("expected ok=false for invalid JSON")
	}
}

func TestParseLine_EmptyMessage(t *testing.T) {
	line := []byte(`{"type":"assistant"}`)

	_, ok := parseLine(line)
	if ok {
		t.Error("expected ok=false for assistant without message")
	}
}

func TestParseLine_MultipleToolUse(t *testing.T) {
	// 複数の tool_use がある場合、最初のものを返す
	line := []byte(`{"type":"assistant","message":{"role":"assistant","content":[{"type":"tool_use","id":"tu_1","name":"WebSearch","input":{}},{"type":"tool_use","id":"tu_2","name":"WebFetch","input":{}}]}}`)

	event, ok := parseLine(line)
	if !ok {
		t.Fatal("expected ok=true for multiple tool_use line")
	}
	if event.ToolName != "WebSearch" {
		t.Errorf("expected first tool WebSearch, got %q", event.ToolName)
	}
}

func TestParseLine_ToolUseWithoutName(t *testing.T) {
	line := []byte(`{"type":"assistant","message":{"role":"assistant","content":[{"type":"tool_use","id":"tu_1","input":{}}]}}`)

	_, ok := parseLine(line)
	if ok {
		t.Error("expected ok=false for tool_use without name")
	}
}
