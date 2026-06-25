import { cubicOut, cubicInOut } from "svelte/easing";
import { prefersReducedMotion } from "./reducedMotion";

type TransitionConfig = {
  delay?: number;
  duration: number;
  easing?: (t: number) => number;
  css?: (t: number, u: number) => string;
};

function opacityOnly(_: HTMLElement): TransitionConfig {
  return { duration: 0, css: (t) => `opacity: ${t};` };
}

/** Fade + 8px slide-up; 180ms cubic-out. Used for new assistant prompts. */
export function chatEnter(node: HTMLElement): TransitionConfig {
  if (prefersReducedMotion()) return opacityOnly(node);
  return {
    duration: 180,
    easing: cubicOut,
    css: (t, u) => `opacity: ${t}; transform: translateY(${u * 8}px);`,
  };
}

/** Hover lift: translateY(-1px) + shadow. 120ms cubic-out. */
export function cardLift(node: HTMLElement): TransitionConfig {
  if (prefersReducedMotion()) return opacityOnly(node);
  return {
    duration: 120,
    easing: cubicOut,
    css: (t) =>
      `transform: translateY(${(1 - t) * 0 + t * -1}px); box-shadow: 0 ${t * 4}px ${t * 12}px rgba(0,0,0,${t * 0.18});`,
  };
}

/** 50ms gold wash + 80ms decay; total 130ms. Used on chip + button activation. */
export function chipFlash(node: HTMLElement): TransitionConfig {
  if (prefersReducedMotion()) return opacityOnly(node);
  return {
    duration: 130,
    easing: cubicOut,
    css: (t) => {
      const wash = t < 50 / 130 ? 1 : Math.max(0, 1 - (t - 50 / 130) * (130 / 80));
      return `background-color: rgba(200, 146, 15, ${wash * 0.35});`;
    },
  };
}

/** 60ms red shake. Used when an optimistic action is rejected. */
export function errorShake(node: HTMLElement): TransitionConfig {
  if (prefersReducedMotion()) return opacityOnly(node);
  return {
    duration: 60,
    css: (t) => {
      const offset = Math.sin(t * Math.PI * 4) * 3;
      return `transform: translateX(${offset}px); background-color: rgba(217, 83, 79, ${0.18 * (1 - t)});`;
    },
  };
}

/** Save celebration: gold breathe (2 cycles) + staggered reveal. 1400ms cubic-in-out. */
export function saveCelebration(node: HTMLElement): TransitionConfig {
  if (prefersReducedMotion()) return opacityOnly(node);
  return {
    duration: 1400,
    easing: cubicInOut,
    css: (t) => {
      const breathe = 0.5 + 0.5 * Math.sin(t * Math.PI * 4);
      const fadeIn = Math.min(1, t * 2);
      return `opacity: ${fadeIn}; box-shadow: 0 0 ${breathe * 32}px rgba(200, 146, 15, ${breathe * 0.45});`;
    },
  };
}
