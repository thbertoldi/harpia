# capability-based-binding

## Purpose
Plan steps declare the capabilities they require; `SlotBinding` matches by capability
coverage; capabilities the user did not opt into are excluded from the run and need no binding.

## ADDED Requirements

### Requirement: Step requires capabilities
A `PlanStep` SHALL declare `ExecutorRequirement.required_capabilities`; `default_executor_sku_key`
is a default hint, not the contract.

#### Scenario: step matched by capability
- GIVEN a step requiring `carousel-authoring`
- WHEN the configuration binds an executor
- THEN any agent whose capabilities include `carousel-authoring` may fill it (not only a
  specific named SKU)

### Requirement: Opt-out capabilities are excluded
Capabilities declared **opt-in** on the template/role SHALL be excluded from the run when the
user does not select them; steps whose only required capability is opted out SHALL be skipped
and require no SlotBinding, no overseer, and no installation.

#### Scenario: image not selected does not block
- GIVEN a plan with an opt-in `image-generation` step and a user who did not select images
- WHEN the configuration is validated
- THEN the image step is excluded from the run and the configuration can still reach RUNNABLE
  without binding an image executor

#### Scenario: image selected joins the run
- GIVEN the same plan and a user who selected generated images
- WHEN the configuration is validated
- THEN the image step requires a binding (the Image Generator agent) and runs

### Requirement: RUNNABLE invariant respects opt-out
A configuration SHALL be RUNNABLE only if every step **that will run** has a compatible SlotBinding;
opted-out steps are not "steps that will run".

#### Scenario: opt-out step ignored for RUNNABLE
- GIVEN a plan with an opted-out image step
- WHEN all other steps are bound
- THEN the configuration is RUNNABLE
