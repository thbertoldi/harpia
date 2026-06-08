import { Code, ConnectError } from "@connectrpc/connect";

const USER_MESSAGES: Partial<Record<Code, string>> = {
  [Code.Canceled]: "The request was cancelled.",
  [Code.InvalidArgument]:
    "The request is invalid. Check the fields and try again.",
  [Code.Unauthenticated]: "Sign in again to continue.",
  [Code.PermissionDenied]: "You do not have access to perform this action.",
  [Code.NotFound]: "The requested resource was not found.",
  [Code.ResourceExhausted]: "Too many requests. Try again shortly.",
  [Code.Unavailable]:
    "The service is temporarily unavailable. Try again shortly.",
  [Code.DeadlineExceeded]: "The request timed out. Try again.",
  [Code.Internal]: "The service hit an internal error.",
};

export function toUserMessage(error: unknown): string {
  if (error instanceof ConnectError) {
    return USER_MESSAGES[error.code] ?? error.rawMessage;
  }

  if (error instanceof Error) {
    return error.message;
  }

  if (
    error &&
    typeof error === "object" &&
    "message" in error &&
    typeof error.message === "string"
  ) {
    return error.message;
  }

  return "Something went wrong.";
}
