import { error } from "@sveltejs/kit";
import type { PageLoad } from "./$types";
import { getTenant } from "$lib/auth";
import { listArtifacts } from "$lib/artifacts/artifacts";
import { loadThreadMessages } from "$lib/chat/client";
import type { ChatMessage } from "$lib/chat/types";
import { toUserMessage } from "$lib/connect-errors";
import type { Artifact } from "$lib/gen/harpia/artifacts/v1/artifacts_pb";
import { loadPlanExecutionDetail } from "$lib/plans/plan-execution-detail";

export const ssr = false;

export const load: PageLoad = async ({ params }) => {
  const tenant = getTenant();
  if (!tenant?.id) {
    throw error(401, "Not authenticated");
  }

  const { executionId } = params;
  try {
    const detail = await loadPlanExecutionDetail(tenant.id, executionId);
    let activity: ChatMessage[] = [];
    let activityError = "";
    try {
      activity = (
        await loadThreadMessages(
          tenant.id,
          detail.execution.planConfigurationId,
          200,
        )
      ).filter((message) => message.executionId === executionId);
    } catch (err) {
      activityError = toUserMessage(err);
    }

    let artifacts: Artifact[] = [];
    let artifactsError = "";
    try {
      artifacts = await listArtifacts({
        tenantId: tenant.id,
        planExecutionId: executionId,
      });
    } catch (err) {
      artifactsError = toUserMessage(err);
    }

    return {
      tenantId: tenant.id,
      executionId,
      detail,
      activity,
      activityError,
      artifacts,
      artifactsError,
    };
  } catch (err) {
    throw error(404, toUserMessage(err));
  }
};
