package orchestration

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"ollama_agent_go/internal/agent/roles"
	"ollama_agent_go/internal/providers"
	"ollama_agent_go/internal/types"
)

// Planner uses an LLM to decompose a high-level goal into a TaskGraph.
type Planner struct {
	provider providers.Provider
	model    string
	maxTasks int
}

// NewPlanner creates a Planner backed by the given provider/model pair.
func NewPlanner(p providers.Provider, model string, maxTasks int) *Planner {
	if maxTasks <= 0 {
		maxTasks = 5
	}
	return &Planner{provider: p, model: model, maxTasks: maxTasks}
}

type planTask struct {
	ID        string   `json:"id"`
	Goal      string   `json:"goal"`
	Role      string   `json:"role"`
	DependsOn []string `json:"depends_on"`
}

type planResponse struct {
	Tasks []planTask `json:"tasks"`
}

const planningSystemPrompt = `You are a task decomposition assistant. Decompose goals into concrete sub-tasks.
Return ONLY valid JSON matching this schema exactly:
{"tasks": [{"id": "t1", "goal": "...", "role": "research|reasoning|action|data|communication", "depends_on": []}]}
Do not include any text outside the JSON object.`

// Plan decomposes goal into a TaskGraph by calling the LLM.
// Falls back to a single-task graph if the LLM response cannot be parsed.
func (pl *Planner) Plan(ctx context.Context, goal string) (*TaskGraph, error) {
	userMsg := fmt.Sprintf(
		"Decompose into at most %d sub-tasks:\n\n%s",
		pl.maxTasks, goal,
	)

	req := types.ChatRequest{
		Model: pl.model,
		Messages: []types.Message{
			{Role: "system", Content: planningSystemPrompt},
			{Role: "user", Content: userMsg},
		},
	}

	resp, err := pl.provider.Chat(ctx, req)
	if err != nil {
		return fallbackGraph(goal), nil
	}

	g, parseErr := parsePlan(resp.Message.Content)
	if parseErr != nil {
		return fallbackGraph(goal), nil
	}
	return g, nil
}

func parsePlan(raw string) (*TaskGraph, error) {
	// Strip markdown code fences if present.
	raw = strings.TrimSpace(raw)
	if strings.HasPrefix(raw, "```") {
		raw = raw[strings.Index(raw, "\n")+1:]
		if idx := strings.LastIndex(raw, "```"); idx >= 0 {
			raw = raw[:idx]
		}
		raw = strings.TrimSpace(raw)
	}

	var pr planResponse
	if err := json.Unmarshal([]byte(raw), &pr); err != nil {
		return nil, fmt.Errorf("planner: parse plan JSON: %w", err)
	}
	if len(pr.Tasks) == 0 {
		return nil, fmt.Errorf("planner: empty task list")
	}

	g := &TaskGraph{}
	for _, pt := range pr.Tasks {
		g.Add(&Task{
			ID:        pt.ID,
			Goal:      pt.Goal,
			Role:      roles.ParseRoleString(pt.Role),
			DependsOn: pt.DependsOn,
			Status:    TaskPending,
		})
	}
	return g, nil
}

// fallbackGraph wraps the entire goal in a single General task.
func fallbackGraph(goal string) *TaskGraph {
	g := &TaskGraph{}
	g.Add(&Task{ID: "t1", Goal: goal, Role: roles.RoleGeneral, Status: TaskPending})
	return g
}
