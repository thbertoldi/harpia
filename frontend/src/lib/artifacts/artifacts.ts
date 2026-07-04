import { artifactClient } from "$lib/rpc";
import type {
  Artifact,
  ArtifactVersion,
} from "$lib/gen/harpia/artifacts/v1/artifacts_pb";

export async function listArtifacts(args: {
  tenantId: string;
  planExecutionId?: string;
  planConfigurationId?: string;
  artifactTypeKey?: string;
}): Promise<Artifact[]> {
  const artifacts: Artifact[] = [];
  for await (const page of artifactClient.listArtifacts({
    tenantId: args.tenantId,
    planExecutionId: args.planExecutionId,
    planConfigurationId: args.planConfigurationId,
    artifactTypeKey: args.artifactTypeKey,
    pageSize: 100,
    pageToken: "",
  })) {
    artifacts.push(...page.artifacts);
  }
  return artifacts.sort((left, right) =>
    right.updatedAt.localeCompare(left.updatedAt),
  );
}

export async function getArtifact(
  tenantId: string,
  artifactId: string,
): Promise<Artifact | null> {
  try {
    const res = await artifactClient.getArtifact({ tenantId, artifactId });
    return res.artifact ?? null;
  } catch {
    return null;
  }
}

export async function listArtifactVersions(
  tenantId: string,
  artifactId: string,
): Promise<ArtifactVersion[]> {
  const versions: ArtifactVersion[] = [];
  let pageToken = "";
  do {
    const response = await artifactClient.listArtifactVersions({
      tenantId,
      artifactId,
      pageSize: 100,
      pageToken,
    });
    versions.push(...response.versions);
    pageToken = response.nextPageToken;
  } while (pageToken);
  return versions.sort(
    (left, right) => right.versionNumber - left.versionNumber,
  );
}

export async function saveTextArtifactVersion(args: {
  tenantId: string;
  artifactId: string;
  expectedContentHash: string;
  title?: string;
  text: string;
  editSummary?: string;
}) {
  return artifactClient.saveTextArtifactVersion({
    tenantId: args.tenantId,
    artifactId: args.artifactId,
    expectedContentHash: args.expectedContentHash,
    title: args.title ?? "",
    text: args.text,
    editSummary: args.editSummary ?? "",
  });
}
