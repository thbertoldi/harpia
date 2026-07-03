import { describe, expect, it } from "vitest";
import { renderMarkdown } from "$lib/artifacts/markdown";

describe("renderMarkdown", () => {
  it("escapes raw HTML for safe injection", () => {
    expect(renderMarkdown("<script>alert(1)</script>")).toContain(
      "&lt;script&gt;",
    );
    expect(renderMarkdown("<b>x</b>")).not.toContain("<b>");
  });

  it("renders headings and paragraphs", () => {
    const out = renderMarkdown("# Title\n\nA paragraph.");
    expect(out).toContain("<h1>Title</h1>");
    expect(out).toContain("<p>A paragraph.</p>");
  });

  it("renders bold and italic", () => {
    expect(renderMarkdown("**bold**")).toContain("<strong>bold</strong>");
    expect(renderMarkdown("*italic*")).toContain("<em>italic</em>");
  });

  it("renders inline code and fenced code blocks", () => {
    expect(renderMarkdown("use `x`")).toContain("<code>x</code>");
    const out = renderMarkdown("```\ncode line\n```");
    expect(out).toContain("<pre><code>code line</code></pre>");
  });

  it("renders unordered and ordered lists", () => {
    expect(renderMarkdown("- a\n- b")).toContain("<ul>");
    expect(renderMarkdown("- a\n- b")).toContain("<li>a</li>");
    expect(renderMarkdown("1. a\n2. b")).toContain("<ol>");
  });

  it("renders blockquotes and hr", () => {
    // '>' is escaped to &gt; before the blockquote rule runs
    expect(renderMarkdown("> quoted")).toContain(
      "<blockquote>quoted</blockquote>",
    );
    expect(renderMarkdown("---")).toContain("<hr />");
  });

  it("links safe schemes and drops unsafe ones", () => {
    const out = renderMarkdown("[g](https://example.com)");
    expect(out).toContain('href="https://example.com"');
    const bad = renderMarkdown("[x](javascript:alert(1))");
    expect(bad).not.toContain("javascript:");
    expect(bad).toContain("x");
  });
});
