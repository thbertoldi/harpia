// See https://svelte.dev/docs/kit/types#app.d.ts
import type { User } from "$lib/auth";

declare global {
  namespace App {
    interface Locals {
      user: User | null;
    }
    interface PageData {
      user: User | null;
    }
  }
}

export {};
