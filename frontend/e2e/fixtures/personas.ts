import type { Page } from "@playwright/test";

type PersonaRole = "Leader" | "Overseer" | "Engineer";

type Persona = {
  sub: string;
  email: string;
  name: string;
  role: PersonaRole;
};

const devTenant = { id: "dev", name: "Dev Workspace" };

const personas: Record<PersonaRole, Persona> = {
  Leader: {
    sub: "dev-leader",
    email: "leader@harpia.local",
    name: "Lena Leader",
    role: "Leader",
  },
  Overseer: {
    sub: "dev-overseer",
    email: "overseer@harpia.local",
    name: "Owen Overseer",
    role: "Overseer",
  },
  Engineer: {
    sub: "dev-engineer",
    email: "engineer@harpia.local",
    name: "Eli Engineer",
    role: "Engineer",
  },
};

function sessionFor(role: PersonaRole) {
  return {
    user: personas[role],
    tenant: devTenant,
    tokens: { access_token: "dev-token", id_token: "dev-token" },
  };
}

function cookieFor(role: PersonaRole) {
  const persona = personas[role];
  return {
    sub: persona.sub,
    email: persona.email,
    name: persona.name,
    role,
    tenant_id: devTenant.id,
  };
}

export async function loginAsPersona(
  page: Page,
  role: PersonaRole,
  baseURL = "http://127.0.0.1:5173",
): Promise<void> {
  const session = sessionFor(role);

  await page.context().addCookies([
    {
      name: "harpia_session",
      value: encodeURIComponent(JSON.stringify(cookieFor(role))),
      url: baseURL,
      sameSite: "Lax",
    },
  ]);

  await page.addInitScript((storedSession) => {
    localStorage.setItem("harpia_session", JSON.stringify(storedSession));
    localStorage.setItem("harpia-theme", "dark");
  }, session);
}

export async function loginAsLeader(
  page: Page,
  baseURL?: string,
): Promise<void> {
  await loginAsPersona(page, "Leader", baseURL);
}

export async function loginAsOverseer(
  page: Page,
  baseURL?: string,
): Promise<void> {
  await loginAsPersona(page, "Overseer", baseURL);
}

export async function loginAsEngineer(
  page: Page,
  baseURL?: string,
): Promise<void> {
  await loginAsPersona(page, "Engineer", baseURL);
}
