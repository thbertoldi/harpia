// See https://svelte.dev/docs/kit/types#app.d.ts
import type { Session } from "$lib/auth";

declare global {
  namespace App {
    interface Locals {
      user: Session | null;
    }
    interface PageData {
      user: Session | null;
    }
  }
}

export {};
