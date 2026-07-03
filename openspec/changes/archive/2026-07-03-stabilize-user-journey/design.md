## Context

The system is suffering from "model whiplash" because we are trying to build the new conversational model (ADR-017) while the old standalone surfaces still exist. The state heuristics are guessing which one is active, leading to an incoherent user journey. We need a Stabilization Sequence to freeze the invariants and establish a single operational contract before delegating further work to subagents.

## Goals / Non-Goals

**Goals:**
- Establish a single, coherent user journey based on ADR-017.
- Remove old, standalone surfaces that conflict with the conversational model.
- Implement the 1:N Thread ↔ PlanConfiguration relationship in the UI.
- Create the new `/runs` destination for managing executions.
- Implement the final-artifact side preview in the chat.
- Implement the hybrid conversational configuration with a structured approval card.

**Non-Goals:**
- Cross-thread plan merging or moving a plan between conversations.
- Marketplace / partner-authored recurring plans.
- Real-time collaborative editing of a conversation.
- Modifying the backend data model (this is purely a frontend UI/UX stabilization).

## Decisions

1.  **Surface Cuts & Navigation Lock**: We will delete the `/plans/configurations`, `/plans/executions`, and `/artifacts` routes. The main navigation will be locked to Home (`/`), Gallery (`/plans`), Runs (`/runs`), and Conversation (`/chat/[threadId]`). This forces the user into the conversational model and eliminates the confusion of multiple surfaces representing the same thing.
2.  **State Canonicalization**: The chat UI will use chips/tabs at the top to represent the 1:N relationship between a thread and its plans. The active plan will be strictly derived from the selected tab. `origin_thread_id` will be used to link plans back to their origin thread.
3.  **Runs Panel**: The `/runs` route will group executions by plan. This provides a single place to view and manage all executions, including recurring plans. Light edits for recurring plans will happen here, while structural edits will redirect to the chat.
4.  **Final-Artifact Side Preview**: A split-pane/side-drawer will be added to the chat to display the final artifact of a plan. This keeps the chat stream clean and focused on progress, while providing a dedicated space for the final output.
5.  **Hybrid Conversational Configuration**: The "leave a note" box will be replaced with a real conversational composer and a structured approval card. This ensures that plans are only executed after explicit user approval, providing a reliable backstop for the LLM materialization path.

## Risks / Trade-offs

-   **Risk**: Removing the standalone routes might break existing workflows for users who rely on them.
    -   **Mitigation**: The new conversational model and Runs panel should provide a superior alternative. We will communicate the changes clearly and provide guidance on the new workflows.
-   **Risk**: The hybrid conversational configuration might be complex to implement and require significant LLM integration.
    -   **Mitigation**: We will start with a simple implementation and iterate based on user feedback. The structured approval card provides a safety net.
