package tools

import (
	"context"
	"encoding/json"
	"fmt"
)

// TransitionPhaseTool allows the agent to transition between debugging phases
type TransitionPhaseTool struct {
	plan PhaseManager
}

// PhaseManager is an interface for managing debugging phases
type PhaseManager interface {
	TransitionToPhase(newPhase string, reason string)
	GetPhaseDescription() string
	GetCurrentPhase() string
}

// TransitionPhaseParams defines the parameters for transitioning phases
type TransitionPhaseParams struct {
	NewPhase string `json:"new_phase"`
	Reason   string `json:"reason"`
}

// NewTransitionPhaseTool creates a new TransitionPhaseTool
func NewTransitionPhaseTool(plan PhaseManager) *TransitionPhaseTool {
	return &TransitionPhaseTool{
		plan: plan,
	}
}

// Description returns the tool description
func (t *TransitionPhaseTool) Description() string {
	return `Transition to a new debugging phase. Use this tool when you have completed the current phase and are ready to move to the next.

Debugging phases (in typical order):
1. "problem_definition" - Define the problem clearly: What? When? Where? Who? Impact?
2. "impact_analysis" - Analyze impact scope: Is it occasional or inevitable? How many users affected?
3. "root_cause_hypothesis" - Form hypotheses about root causes based on symptoms
4. "investigation_plan" - Create a detailed investigation plan to verify hypotheses
5. "execution" - Execute the investigation plan and collect evidence
6. "verification" - Verify findings and proposed solutions
7. "reporting" - Generate final report with conclusions and recommendations

Parameters:
- new_phase (required): The phase to transition to (one of the above)
- reason (required): Why you are transitioning to this phase (what was accomplished in the previous phase)`
}

// Execute executes the tool
func (t *TransitionPhaseTool) Execute(ctx context.Context, params *TransitionPhaseParams) (string, error) {
	if params.NewPhase == "" {
		return "", fmt.Errorf("new_phase is required")
	}

	if params.Reason == "" {
		return "", fmt.Errorf("reason is required - explain why you are transitioning to this phase")
	}

	// Validate phase
	validPhases := map[string]bool{
		"problem_definition":    true,
		"impact_analysis":       true,
		"root_cause_hypothesis": true,
		"investigation_plan":    true,
		"execution":             true,
		"verification":          true,
		"reporting":             true,
	}

	if !validPhases[params.NewPhase] {
		return "", fmt.Errorf("invalid phase: %s. Must be one of: problem_definition, impact_analysis, root_cause_hypothesis, investigation_plan, execution, verification, reporting", params.NewPhase)
	}

	currentPhase := t.plan.GetCurrentPhase()
	t.plan.TransitionToPhase(params.NewPhase, params.Reason)

	result := fmt.Sprintf("✨ Phase Transition\n\n")
	result += fmt.Sprintf("From: %s\n", currentPhase)
	result += fmt.Sprintf("To: %s\n", params.NewPhase)
	result += fmt.Sprintf("Reason: %s\n\n", params.Reason)
	result += t.plan.GetPhaseDescription()

	return result, nil
}

// ExecuteString executes the tool with string parameters
func (t *TransitionPhaseTool) ExecuteString(params string) (string, error) {
	var p TransitionPhaseParams
	if err := json.Unmarshal([]byte(params), &p); err != nil {
		return "", fmt.Errorf("failed to parse parameters: %w", err)
	}

	return t.Execute(context.Background(), &p)
}
