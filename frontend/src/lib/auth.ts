import { env } from "$env/dynamic/public";

const ZITADEL_CONFIG = {
  issuer: env.PUBLIC_ZITADEL_ISSUER ?? "http://localhost:8085",
  clientId: env.PUBLIC_ZITADEL_CLIENT_ID ?? "",
  redirectUri: "http://localhost:5173/auth/callback",
  scope: "openid profile email",
};

export function isZitadelConfigured(): boolean {
  return ZITADEL_CONFIG.clientId.length > 0;
}

export interface User {
  sub: string;
  email: string;
  name: string;
  picture?: string;
  role?: string;
}

export interface Tenant {
  id: string;
  name: string;
}

export const DEV_TENANT: Tenant = { id: "dev", name: "Dev Workspace" };

export interface Session {
  user: User;
  tenant: Tenant | null;
  tokens: {
    access_token: string;
    id_token: string;
  };
}

function base64url(buffer: Uint8Array): string {
  return btoa(String.fromCharCode(...buffer))
    .replace(/=/g, "")
    .replace(/\+/g, "-")
    .replace(/\//g, "_");
}

function generateCodeVerifier(): string {
  const array = new Uint8Array(32);
  crypto.getRandomValues(array);
  return base64url(array);
}

async function generateCodeChallenge(verifier: string): Promise<string> {
  const encoder = new TextEncoder();
  const data = encoder.encode(verifier);
  const digest = await crypto.subtle.digest("SHA-256", data);
  return base64url(new Uint8Array(digest));
}

function generateState(): string {
  const array = new Uint8Array(16);
  crypto.getRandomValues(array);
  return base64url(array);
}

export async function login(opts?: { login_hint?: string }): Promise<void> {
  const codeVerifier = generateCodeVerifier();
  const codeChallenge = await generateCodeChallenge(codeVerifier);
  const state = generateState();

  sessionStorage.setItem("code_verifier", codeVerifier);
  sessionStorage.setItem("oauth_state", state);

  const params = new URLSearchParams({
    client_id: ZITADEL_CONFIG.clientId,
    redirect_uri: ZITADEL_CONFIG.redirectUri,
    response_type: "code",
    scope: ZITADEL_CONFIG.scope,
    code_challenge: codeChallenge,
    code_challenge_method: "S256",
    state,
  });

  if (opts?.login_hint) {
    params.set("login_hint", opts.login_hint);
  }

  window.location.href = `${ZITADEL_CONFIG.issuer}/oauth/v2/authorize?${params}`;
}

export async function handleCallback(code: string): Promise<Session> {
  const codeVerifier = sessionStorage.getItem("code_verifier");

  if (!codeVerifier) {
    throw new Error("Missing code verifier — restart the login flow");
  }

  const tokenBody = new URLSearchParams({
    grant_type: "authorization_code",
    code,
    redirect_uri: ZITADEL_CONFIG.redirectUri,
    client_id: ZITADEL_CONFIG.clientId,
    code_verifier: codeVerifier,
  });

  const tokenResponse = await fetch(`${ZITADEL_CONFIG.issuer}/oauth/v2/token`, {
    method: "POST",
    headers: { "Content-Type": "application/x-www-form-urlencoded" },
    body: tokenBody,
  });

  if (!tokenResponse.ok) {
    const err = await tokenResponse.text();
    throw new Error(`Token exchange failed: ${err}`);
  }

  const tokens = await tokenResponse.json();

  const userResponse = await fetch(
    `${ZITADEL_CONFIG.issuer}/oidc/v1/userinfo`,
    {
      headers: { Authorization: `Bearer ${tokens.access_token}` },
    },
  );

  if (!userResponse.ok) {
    throw new Error("Failed to fetch user info");
  }

  const userInfo = await userResponse.json();

  const session: Session = {
    user: {
      sub: userInfo.sub,
      email: userInfo.email,
      name: userInfo.name,
      picture: userInfo.picture,
    },
    tenant: null,
    tokens: {
      access_token: tokens.access_token,
      id_token: tokens.id_token,
    },
  };

  localStorage.setItem("harpia_session", JSON.stringify(session));

  document.cookie = `harpia_session=${encodeURIComponent(
    JSON.stringify({
      sub: session.user.sub,
      email: session.user.email,
      name: session.user.name,
    }),
  )}; path=/; SameSite=Lax`;

  sessionStorage.removeItem("code_verifier");
  sessionStorage.removeItem("oauth_state");

  return session;
}

export function logout(): void {
  localStorage.removeItem("harpia_session");
  document.cookie =
    "harpia_session=; path=/; expires=Thu, 01 Jan 1970 00:00:00 GMT; SameSite=Lax";
  window.location.href = "/login";
}

const DEV_PERSONAS: Record<
  string,
  { sub: string; email: string; name: string }
> = {
  Leader: {
    sub: "dev-leader",
    email: "leader@harpia.local",
    name: "Lena Leader",
  },
  Overseer: {
    sub: "dev-overseer",
    email: "overseer@harpia.local",
    name: "Owen Overseer",
  },
  Engineer: {
    sub: "dev-engineer",
    email: "engineer@harpia.local",
    name: "Eli Engineer",
  },
};

export function devLogin(role: string = "Leader"): Session {
  const persona = DEV_PERSONAS[role] ?? DEV_PERSONAS.Leader;
  const session: Session = {
    user: { ...persona, role },
    tenant: DEV_TENANT,
    tokens: { access_token: "dev-token", id_token: "dev-token" },
  };
  localStorage.setItem("harpia_session", JSON.stringify(session));
  document.cookie = `harpia_session=${encodeURIComponent(
    JSON.stringify({
      sub: session.user.sub,
      email: session.user.email,
      name: session.user.name,
      role,
      tenant_id: DEV_TENANT.id,
    }),
  )}; path=/; SameSite=Lax`;
  return session;
}

export function getSession(): Session | null {
  if (typeof localStorage === "undefined") return null;
  const stored = localStorage.getItem("harpia_session");
  if (!stored) return null;
  try {
    return JSON.parse(stored);
  } catch {
    return null;
  }
}

export function getTenant(): Tenant | null {
  const session = getSession();
  return session?.tenant ?? null;
}

export function requireTenantId(): string {
  const tenant = getTenant();
  if (!tenant?.id) {
    throw new Error("Select a tenant before continuing");
  }
  return tenant.id;
}

export function setTenant(tenant: Tenant): void {
  const session = getSession();
  if (!session) return;
  session.tenant = tenant;
  localStorage.setItem("harpia_session", JSON.stringify(session));
}
