## 1. Surface Cuts & Navigation Lock

- [x] 1.1 Delete the `/plans/configurations` route and its contents.
- [x] 1.2 Delete the `/plans/executions` route and its contents.
- [x] 1.3 Delete the `/artifacts` route and its contents.
- [x] 1.4 Update the main navigation component to only include Home, Gallery, Runs, and Conversation.
- [x] 1.5 Remove any buttons or links in the UI that navigate to the deleted standalone routes.
- [x] 1.6 Add redirects for the deleted routes to fallback to Home or a relevant active route.

## 2. State Canonicalization (1:N Thread ↔ Plan)

- [x] 2.1 Update the `ChatPageData` and `ChatPage` component to support multiple `PlanConfiguration`s per thread.
- [x] 2.2 Implement chips/tabs UI at the top of the conversation view to switch between plans.
- [x] 2.3 Update the active plan derivation logic to strictly use the selected chip/tab.
- [x] 2.4 Implement fallback logic to select the most recently created plan if no tab is selected.
- [x] 2.5 Ensure `origin_thread_id` is correctly populated and used as the source of truth for a plan's origin.

## 3. Runs Panel

- [x] 3.1 Create the new `/runs` route and page component.
- [x] 3.2 Implement data fetching to retrieve all executions and group them by `PlanConfiguration`.
- [x] 3.3 Build the UI to display grouped executions, including status and details.
- [x] 3.4 Add links from each plan/execution in the Runs panel back to its origin thread (`/chat/[threadId]`).
- [x] 3.5 Implement inline editing for recurring plans (sources, schedule, tone) within the Runs panel.
- [x] 3.6 Implement redirection to the origin thread for structural edits.

## 4. Final-Artifact Side Preview

- [x] 4.1 Implement a split-pane or side-drawer layout in the conversation view.
- [x] 4.2 Update the artifact rendering logic to display the final artifact in the side preview.
- [x] 4.3 Update the chat stream rendering to suppress intermediate artifacts and only show progress messages.
- [x] 4.4 Add a close button to the side preview to allow the user to hide it and expand the chat area.

## 5. Hybrid Conversational Configuration

- [x] 5.1 Replace the static "leave a note" box with a conversational composer component.
- [x] 5.2 Implement the logic to process free-text input and fill smart defaults for plan configuration.
- [x] 5.3 Implement the assistant's ability to ask clarifying questions in the chat stream.
- [x] 5.4 Build the structured approval card component to display pre-filled configuration details.
- [x] 5.5 Implement the execution gate to ensure a plan only transitions to RUNNABLE/SCHEDULED after explicit approval via the card.
