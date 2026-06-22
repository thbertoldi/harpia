import type { ChatMessage } from "$lib/chat/types";

export interface ExecutionGroup {
  executionId: string;
  runNumber: number;
  messages: ChatMessage[];
  status: "running" | "completed" | "failed" | "unknown";
}

export type ThreadSection =
  | { kind: "plan-scope"; message: ChatMessage }
  | { kind: "execution"; group: ExecutionGroup };

const TERMINAL_KIND_STATUS: Partial<
  Record<ChatMessage["kind"], ExecutionGroup["status"]>
> = {
  RUN_COMPLETED: "completed",
  RUN_FAILED: "failed",
};

export function buildThreadSections(messages: ChatMessage[]): ThreadSection[] {
  const sections: ThreadSection[] = [];
  const runNumberByExecutionId = new Map<string, number>();
  let nextRunNumber = 1;
  let currentGroup: ExecutionGroup | null = null;

  for (const message of messages) {
    if (message.executionId === "") {
      // Plan-scope — flush any open group and emit as its own section.
      if (currentGroup) {
        sections.push({ kind: "execution", group: currentGroup });
        currentGroup = null;
      }
      sections.push({ kind: "plan-scope", message });
      continue;
    }
    if (currentGroup && currentGroup.executionId !== message.executionId) {
      sections.push({ kind: "execution", group: currentGroup });
      currentGroup = null;
    }
    if (!currentGroup) {
      const existingRunNumber = runNumberByExecutionId.get(message.executionId);
      const runNumber = existingRunNumber ?? nextRunNumber;
      if (existingRunNumber === undefined) {
        runNumberByExecutionId.set(message.executionId, runNumber);
        nextRunNumber += 1;
      }
      currentGroup = {
        executionId: message.executionId,
        runNumber,
        messages: [],
        status: "running",
      };
    }
    currentGroup.messages.push(message);
    const terminalStatus = TERMINAL_KIND_STATUS[message.kind];
    if (terminalStatus) {
      currentGroup.status = terminalStatus;
    }
  }

  if (currentGroup) {
    sections.push({ kind: "execution", group: currentGroup });
  }

  return sections;
}
