package plans

import (
	"bytes"
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"sort"
	"strings"

	"github.com/harpia/control-plane/internal/database"
	"github.com/jackc/pgx/v5/pgxpool"
	"gopkg.in/yaml.v3"
)

//go:embed templates/*.yaml
var embeddedPlanTemplateFS embed.FS

type planTemplateSeed struct {
	Key             string                       `yaml:"key"`
	Name            string                       `yaml:"name"`
	Description     string                       `yaml:"description"`
	Vertical        string                       `yaml:"vertical"`
	Version         int32                        `yaml:"version"`
	Steps           []planTemplateStepSeed       `yaml:"steps"`
	Edges           []planTemplateEdgeSeed       `yaml:"edges"`
	InputParameters []templateInputParameterSeed `yaml:"input_parameters" json:"inputParameters"`
}

type planTemplateStepSeed struct {
	Key                   string         `yaml:"key"`
	Title                 string         `yaml:"title"`
	Description           string         `yaml:"description"`
	InputArtifactTypeID   string         `yaml:"input_artifact_type"`
	OutputArtifactTypeID  string         `yaml:"output_artifact_type"`
	ExecutorRequirement   map[string]any `yaml:"executor_requirement"`
	DefaultExecutorSKUKey string         `yaml:"default_executor_sku_key"`
}

type planTemplateEdgeSeed struct {
	FromStepKey string `yaml:"from_step_key"`
	ToStepKey   string `yaml:"to_step_key"`
}

type templateInputParameterSeed struct {
	Key              string                            `yaml:"key" json:"key"`
	Label            string                            `yaml:"label" json:"label"`
	Description      string                            `yaml:"description" json:"description"`
	Type             string                            `yaml:"type" json:"type"`
	Required         bool                              `yaml:"required" json:"required"`
	DefaultValueJSON string                            `yaml:"defaultValueJson" json:"defaultValueJson,omitempty"`
	OptionsJSON      string                            `yaml:"optionsJson" json:"optionsJson,omitempty"`
	RuntimeMappings  []templateInputRuntimeMappingSeed `yaml:"runtimeMappings" json:"runtimeMappings,omitempty"`
}

type templateInputRuntimeMappingSeed struct {
	Target    string `yaml:"target" json:"target"`
	StepKey   string `yaml:"stepKey" json:"stepKey,omitempty"`
	InputName string `yaml:"inputName" json:"inputName,omitempty"`
	JSONPath  string `yaml:"jsonPath" json:"jsonPath,omitempty"`
	PolicyKey string `yaml:"policyKey" json:"policyKey,omitempty"`
}

func loadPlanTemplateCatalog(files map[string][]byte, artifactTypeKeys, executorSKUKeys map[string]struct{}) ([]planTemplateSeed, error) {
	names := make([]string, 0, len(files))
	for name := range files {
		names = append(names, name)
	}
	sort.Strings(names)

	templates := make([]planTemplateSeed, 0, len(files))
	for _, name := range names {
		var seed planTemplateSeed
		decoder := yaml.NewDecoder(bytes.NewReader(files[name]))
		decoder.KnownFields(true)
		if err := decoder.Decode(&seed); err != nil {
			return nil, fmt.Errorf("decode plan template %q: %w", name, err)
		}
		templates = append(templates, seed)
	}
	if err := validatePlanTemplateCatalog(templates, artifactTypeKeys, executorSKUKeys); err != nil {
		return nil, err
	}
	return templates, nil
}

func validatePlanTemplateCatalog(templates []planTemplateSeed, artifactTypeKeys, executorSKUKeys map[string]struct{}) error {
	seenTemplates := make(map[string]struct{}, len(templates))
	for i := range templates {
		template := &templates[i]
		template.Key = strings.TrimSpace(template.Key)
		if template.Key == "" {
			return fmt.Errorf("plan template at index %d: key is required", i)
		}
		if _, ok := seenTemplates[template.Key]; ok {
			return fmt.Errorf("duplicate template key %q", template.Key)
		}
		seenTemplates[template.Key] = struct{}{}

		if err := validatePlanTemplateSeed(template, artifactTypeKeys, executorSKUKeys); err != nil {
			return fmt.Errorf("plan template %q: %w", template.Key, err)
		}
	}
	return nil
}

func validatePlanTemplateSeed(template *planTemplateSeed, artifactTypeKeys, executorSKUKeys map[string]struct{}) error {
	if strings.TrimSpace(template.Name) == "" {
		return fmt.Errorf("name is required")
	}
	if template.Version <= 0 {
		return fmt.Errorf("version must be positive")
	}
	if len(template.Steps) == 0 {
		return fmt.Errorf("at least one step is required")
	}

	stepKeys := make(map[string]int, len(template.Steps))
	for i := range template.Steps {
		step := &template.Steps[i]
		step.Key = strings.TrimSpace(step.Key)
		if step.Key == "" {
			return fmt.Errorf("step at position %d: key is required", i+1)
		}
		if _, ok := stepKeys[step.Key]; ok {
			return fmt.Errorf("duplicate step key %q", step.Key)
		}
		stepKeys[step.Key] = i

		if _, ok := artifactTypeKeys[step.InputArtifactTypeID]; !ok {
			return fmt.Errorf("step %q: unknown artifact type %q", step.Key, step.InputArtifactTypeID)
		}
		if _, ok := artifactTypeKeys[step.OutputArtifactTypeID]; !ok {
			return fmt.Errorf("step %q: unknown artifact type %q", step.Key, step.OutputArtifactTypeID)
		}
		if step.DefaultExecutorSKUKey != "" {
			if _, ok := executorSKUKeys[step.DefaultExecutorSKUKey]; !ok {
				return fmt.Errorf("step %q: unknown executor sku %q", step.Key, step.DefaultExecutorSKUKey)
			}
		}
		if i > 0 && template.Steps[i-1].OutputArtifactTypeID != step.InputArtifactTypeID {
			return fmt.Errorf("adjacent steps %q and %q have artifact mismatch: %q != %q",
				template.Steps[i-1].Key, step.Key, template.Steps[i-1].OutputArtifactTypeID, step.InputArtifactTypeID)
		}
	}

	if err := validateTemplateDAG(template, stepKeys); err != nil {
		return err
	}
	return validateTemplateInputParameters(template, stepKeys)
}

func validateTemplateDAG(template *planTemplateSeed, stepKeys map[string]int) error {
	adjacent := make(map[string][]string, len(template.Steps))
	for _, edge := range template.Edges {
		from := strings.TrimSpace(edge.FromStepKey)
		to := strings.TrimSpace(edge.ToStepKey)
		if _, ok := stepKeys[from]; !ok {
			return fmt.Errorf("edge %q -> %q references unknown step %q", from, to, from)
		}
		if _, ok := stepKeys[to]; !ok {
			return fmt.Errorf("edge %q -> %q references unknown step %q", from, to, to)
		}
		adjacent[from] = append(adjacent[from], to)
	}

	const (
		unvisited = 0
		visiting  = 1
		visited   = 2
	)
	states := make(map[string]int, len(template.Steps))
	var visit func(string) error
	visit = func(key string) error {
		switch states[key] {
		case visiting:
			return fmt.Errorf("cycle detected at step %q", key)
		case visited:
			return nil
		}
		states[key] = visiting
		for _, next := range adjacent[key] {
			if err := visit(next); err != nil {
				return err
			}
		}
		states[key] = visited
		return nil
	}
	for _, step := range template.Steps {
		if err := visit(step.Key); err != nil {
			return err
		}
	}

	reachable := make(map[string]struct{}, len(template.Steps))
	var walk func(string)
	walk = func(key string) {
		if _, ok := reachable[key]; ok {
			return
		}
		reachable[key] = struct{}{}
		for _, next := range adjacent[key] {
			walk(next)
		}
	}
	walk(template.Steps[0].Key)
	for _, step := range template.Steps {
		if _, ok := reachable[step.Key]; !ok {
			return fmt.Errorf("step %q is unreachable from %q", step.Key, template.Steps[0].Key)
		}
	}
	return nil
}

func validateTemplateInputParameters(template *planTemplateSeed, stepKeys map[string]int) error {
	seen := make(map[string]struct{}, len(template.InputParameters))
	for i := range template.InputParameters {
		param := &template.InputParameters[i]
		param.Key = strings.TrimSpace(param.Key)
		if param.Key == "" {
			return fmt.Errorf("input parameter at index %d: key is required", i)
		}
		if _, ok := seen[param.Key]; ok {
			return fmt.Errorf("duplicate input parameter key %q", param.Key)
		}
		seen[param.Key] = struct{}{}
		for j := range param.RuntimeMappings {
			if err := validateRuntimeMapping(param.Key, &param.RuntimeMappings[j], stepKeys); err != nil {
				return err
			}
		}
	}
	return nil
}

func validateRuntimeMapping(paramKey string, mapping *templateInputRuntimeMappingSeed, stepKeys map[string]int) error {
	target := strings.TrimSpace(mapping.Target)
	switch target {
	case "TEMPLATE_INPUT_RUNTIME_TARGET_SEED_ARTIFACT":
		if _, ok := stepKeys[mapping.StepKey]; !ok {
			return fmt.Errorf("input parameter %q runtime mapping references unknown step %q", paramKey, mapping.StepKey)
		}
		if strings.TrimSpace(mapping.InputName) == "" {
			return fmt.Errorf("input parameter %q runtime mapping seed artifact inputName is required", paramKey)
		}
	case "TEMPLATE_INPUT_RUNTIME_TARGET_SLOT_BINDING":
		if _, ok := stepKeys[mapping.StepKey]; !ok {
			return fmt.Errorf("input parameter %q runtime mapping references unknown step %q", paramKey, mapping.StepKey)
		}
	case "TEMPLATE_INPUT_RUNTIME_TARGET_BEHAVIOR_POLICY":
		switch mapping.PolicyKey {
		case "publish_approval_mode", "elicitation_timeout_behavior":
			return nil
		default:
			return fmt.Errorf("input parameter %q runtime mapping references unknown behavior policy %q", paramKey, mapping.PolicyKey)
		}
	default:
		return fmt.Errorf("input parameter %q runtime mapping has unknown target %q", paramKey, target)
	}
	return nil
}

func inputParametersJSON(params []templateInputParameterSeed) (json.RawMessage, error) {
	if params == nil {
		return json.RawMessage("[]"), nil
	}
	raw, err := json.Marshal(params)
	if err != nil {
		return nil, fmt.Errorf("marshal input parameters: %w", err)
	}
	return raw, nil
}

func EnsurePlanTemplates(ctx context.Context, pool *pgxpool.Pool) error {
	files, err := embeddedPlanTemplateFiles()
	if err != nil {
		return err
	}
	return reconcilePlanTemplateFiles(ctx, pool, files)
}

func embeddedPlanTemplateFiles() (map[string][]byte, error) {
	paths, err := fs.Glob(embeddedPlanTemplateFS, "templates/*.yaml")
	if err != nil {
		return nil, fmt.Errorf("glob embedded plan templates: %w", err)
	}
	if len(paths) == 0 {
		return nil, fmt.Errorf("no embedded plan templates found")
	}
	files := make(map[string][]byte, len(paths))
	for _, path := range paths {
		body, err := embeddedPlanTemplateFS.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("read embedded plan template %q: %w", path, err)
		}
		files[path] = body
	}
	return files, nil
}

func reconcilePlanTemplateFiles(ctx context.Context, q database.Querier, files map[string][]byte) error {
	artifactTypes, err := loadReferenceKeys(ctx, q, "artifact_types")
	if err != nil {
		return err
	}
	executorSKUs, err := loadReferenceKeys(ctx, q, "executor_skus")
	if err != nil {
		return err
	}
	templates, err := loadPlanTemplateCatalog(files, artifactTypes, executorSKUs)
	if err != nil {
		return err
	}
	return reconcilePlanTemplateCatalog(ctx, q, templates)
}

func reconcilePlanTemplateCatalog(ctx context.Context, q database.Querier, templates []planTemplateSeed) error {
	if len(templates) == 0 {
		return fmt.Errorf("at least one plan template is required")
	}
	artifactTypes, err := loadReferenceKeys(ctx, q, "artifact_types")
	if err != nil {
		return err
	}
	executorSKUs, err := loadReferenceKeys(ctx, q, "executor_skus")
	if err != nil {
		return err
	}
	if err := validatePlanTemplateCatalog(templates, artifactTypes, executorSKUs); err != nil {
		return err
	}

	keys := make([]string, 0, len(templates))
	for i := range templates {
		template := &templates[i]
		keys = append(keys, template.Key)
		if err := upsertPlanTemplateSeed(ctx, q, template); err != nil {
			return err
		}
	}
	if _, err := q.Exec(ctx, `DELETE FROM plan_templates WHERE NOT (key = ANY($1))`, keys); err != nil {
		return fmt.Errorf("remove absent plan templates: %w", err)
	}
	return nil
}

func loadReferenceKeys(ctx context.Context, q database.Querier, table string) (map[string]struct{}, error) {
	if table != "artifact_types" && table != "executor_skus" {
		return nil, fmt.Errorf("unsupported reference table %q", table)
	}
	rows, err := q.Query(ctx, "SELECT key FROM "+table)
	if err != nil {
		return nil, fmt.Errorf("load %s keys: %w", table, err)
	}
	defer rows.Close()

	keys := make(map[string]struct{})
	for rows.Next() {
		var key string
		if err := rows.Scan(&key); err != nil {
			return nil, fmt.Errorf("scan %s key: %w", table, err)
		}
		keys[key] = struct{}{}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate %s keys: %w", table, err)
	}
	return keys, nil
}

func upsertPlanTemplateSeed(ctx context.Context, q database.Querier, template *planTemplateSeed) error {
	inputParameters, err := inputParametersJSON(template.InputParameters)
	if err != nil {
		return fmt.Errorf("template %q: %w", template.Key, err)
	}

	var templateID string
	if err := q.QueryRow(ctx, `
		INSERT INTO plan_templates (key, name, description, vertical, version, input_parameters)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (key) DO UPDATE SET
			name = EXCLUDED.name,
			description = EXCLUDED.description,
			vertical = EXCLUDED.vertical,
			version = EXCLUDED.version,
			input_parameters = EXCLUDED.input_parameters,
			updated_at = now()
		RETURNING id
	`, template.Key, template.Name, template.Description, template.Vertical, template.Version, inputParameters).Scan(&templateID); err != nil {
		return fmt.Errorf("upsert plan template %q: %w", template.Key, err)
	}

	if _, err := q.Exec(ctx, `DELETE FROM plan_template_step_dependencies WHERE plan_template_id = $1`, templateID); err != nil {
		return fmt.Errorf("delete dependencies for template %q: %w", template.Key, err)
	}
	if _, err := q.Exec(ctx, `DELETE FROM plan_template_steps WHERE plan_template_id = $1`, templateID); err != nil {
		return fmt.Errorf("delete steps for template %q: %w", template.Key, err)
	}

	for i := range template.Steps {
		step := &template.Steps[i]
		requirement := json.RawMessage("{}")
		if len(step.ExecutorRequirement) > 0 {
			raw, err := json.Marshal(step.ExecutorRequirement)
			if err != nil {
				return fmt.Errorf("marshal executor requirement for step %q: %w", step.Key, err)
			}
			requirement = raw
		}
		if _, err := q.Exec(ctx, `
			INSERT INTO plan_template_steps (
				plan_template_id, key, title, description,
				input_artifact_type_id, output_artifact_type_id,
				executor_requirement, default_executor_sku_key, position
			)
			VALUES ($1, $2, $3, $4, $5, $6, $7, NULLIF($8, ''), $9)
		`, templateID, step.Key, step.Title, step.Description, step.InputArtifactTypeID, step.OutputArtifactTypeID, requirement, step.DefaultExecutorSKUKey, i+1); err != nil {
			return fmt.Errorf("insert step %q for template %q: %w", step.Key, template.Key, err)
		}
	}

	for _, edge := range template.Edges {
		if _, err := q.Exec(ctx, `
			INSERT INTO plan_template_step_dependencies (plan_template_id, from_step_key, to_step_key)
			VALUES ($1, $2, $3)
		`, templateID, edge.FromStepKey, edge.ToStepKey); err != nil {
			return fmt.Errorf("insert edge %q -> %q for template %q: %w", edge.FromStepKey, edge.ToStepKey, template.Key, err)
		}
	}
	return nil
}
