import {
  Code,
  ConnectError,
  type Interceptor,
  type StreamRequest,
  type UnaryRequest,
} from "@connectrpc/connect";
import { createConnectTransport } from "@connectrpc/connect-web";
import { isSpanContextValid, trace } from "@opentelemetry/api";
import { getSession, getTenant } from "$lib/auth";

const MAX_RETRY_ATTEMPTS = 3;
const BASE_RETRY_DELAY_MS = 150;
const MAX_RETRY_DELAY_MS = 1_000;

function wait(ms: number): Promise<void> {
  return new Promise((resolve) => setTimeout(resolve, ms));
}

function retryDelay(attempt: number): number {
  const backoff = BASE_RETRY_DELAY_MS * 2 ** attempt;
  const jitter = Math.floor(Math.random() * BASE_RETRY_DELAY_MS);
  return Math.min(MAX_RETRY_DELAY_MS, backoff + jitter);
}

type ConnectRequest = UnaryRequest | StreamRequest;

function rpcName(req: ConnectRequest) {
  return `${req.service.typeName}/${req.method.name}`;
}

function logConnectError(req: ConnectRequest, error: unknown): void {
  const connectError = ConnectError.from(error);
  if (connectError.code === Code.Canceled) {
    return;
  }

  console.error("[connect]", rpcName(req), {
    code: Code[connectError.code],
    message: connectError.rawMessage,
  });
}

function shouldRetryUnavailable(
  error: unknown,
  attempt: number,
  signal: AbortSignal,
): boolean {
  return (
    !signal.aborted &&
    ConnectError.from(error).code === Code.Unavailable &&
    attempt < MAX_RETRY_ATTEMPTS - 1
  );
}

async function* logStreamErrors<T>(
  source: AsyncIterable<T>,
  req: ConnectRequest,
): AsyncIterable<T> {
  try {
    yield* source;
  } catch (error) {
    logConnectError(req, error);
    throw error;
  }
}

async function* retryUnavailableStream<T>(
  req: StreamRequest,
  next: Parameters<Interceptor>[0],
  initial: AsyncIterable<T>,
): AsyncIterable<T> {
  let source = initial;

  for (let attempt = 0; ; attempt += 1) {
    try {
      yield* source;
      return;
    } catch (error) {
      if (!shouldRetryUnavailable(error, attempt, req.signal)) {
        throw error;
      }

      await wait(retryDelay(attempt));
      const retryResponse = await next(req);
      if (!retryResponse.stream) {
        throw new ConnectError(
          "Retry returned a unary response",
          Code.Internal,
        );
      }
      source = retryResponse.message as AsyncIterable<T>;
    }
  }
}

const tenantHeaderInterceptor: Interceptor = (next) => async (req) => {
  const tenantId = getTenant()?.id ?? getSession()?.tenant?.id ?? "default";
  req.header.set("X-Tenant-ID", tenantId);
  return next(req);
};

const authInterceptor: Interceptor = (next) => async (req) => {
  const token = getSession()?.tokens.access_token;
  if (token) {
    req.header.set("Authorization", `Bearer ${token}`);
  }
  return next(req);
};

const traceInterceptor: Interceptor = (next) => async (req) => {
  const spanContext = trace.getActiveSpan()?.spanContext();
  if (spanContext && isSpanContextValid(spanContext)) {
    const flags = spanContext.traceFlags.toString(16).padStart(2, "0");
    req.header.set(
      "traceparent",
      `00-${spanContext.traceId}-${spanContext.spanId}-${flags}`,
    );
  }
  return next(req);
};

const retryUnavailableInterceptor: Interceptor = (next) => async (req) => {
  for (let attempt = 0; ; attempt += 1) {
    try {
      const response = await next(req);
      if (response.stream) {
        const streamRequest = req as StreamRequest;
        return {
          ...response,
          message: retryUnavailableStream(
            streamRequest,
            next,
            response.message,
          ),
        };
      }
      return response;
    } catch (error) {
      if (!shouldRetryUnavailable(error, attempt, req.signal)) {
        throw error;
      }

      await wait(retryDelay(attempt));
    }
  }
};

const errorLoggingInterceptor: Interceptor = (next) => async (req) => {
  try {
    const response = await next(req);
    if (response.stream) {
      return {
        ...response,
        message: logStreamErrors(response.message, req),
      };
    }
    return response;
  } catch (error) {
    logConnectError(req, error);
    throw error;
  }
};

export const transport = createConnectTransport({
  baseUrl: "/",
  interceptors: [
    errorLoggingInterceptor,
    retryUnavailableInterceptor,
    traceInterceptor,
    authInterceptor,
    tenantHeaderInterceptor,
  ],
});
