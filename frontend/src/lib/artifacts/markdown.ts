/**
 * Minimal, dependency-free Markdown -> HTML renderer for artifact previews.
 *
 * Safety: the input is HTML-escaped BEFORE any transformation, so the result is
 * safe to inject via {@html}. Only a conservative inline subset is supported
 * (headings, paragraphs, lists, blockquotes, code, bold/italic, links, hr) —
 * enough for newsletter / LinkedIn draft content. Anything unrecognised is
 * treated as paragraph text.
 */

function escapeHtml(s: string): string {
  return s
    .replace(/&/g, "&amp;")
    .replace(/</g, "&lt;")
    .replace(/>/g, "&gt;")
    .replace(/"/g, "&quot;")
    .replace(/'/g, "&#39;");
}

function inlineFormat(text: string): string {
  // inline code first (content protected from further transforms)
  let out = text.replace(/`([^`]+)`/g, "<code>$1</code>");
  // links [text](url) — only http/https/mailto URLs are linked
  out = out.replace(
    /\[([^\]]+)\]\(([^)\s]+)\)/g,
    (_m, label: string, url: string) => {
      const safe = /^(https?:\/\/|mailto:)/i.test(url);
      return safe
        ? `<a href="${url}" target="_blank" rel="noopener noreferrer">${label}</a>`
        : label;
    },
  );
  // bold, then italic
  out = out.replace(/\*\*([^*]+)\*\*/g, "<strong>$1</strong>");
  out = out.replace(/\b__([^_]+)__\b/g, "<strong>$1</strong>");
  out = out.replace(/(^|[^*])\*([^*]+)\*/g, "$1<em>$2</em>");
  out = out.replace(/(^|[^_])_([^_]+)_/g, "$1<em>$2</em>");
  return out;
}

export function renderMarkdown(src: string): string {
  if (!src) return "";
  const lines = escapeHtml(src).split(/\r?\n/);
  const html: string[] = [];
  let i = 0;

  const flushParagraph = (buf: string[]) => {
    if (buf.length > 0) {
      html.push(`<p>${inlineFormat(buf.join(" "))}</p>`);
      buf.length = 0;
    }
  };

  const para: string[] = [];

  while (i < lines.length) {
    const line = lines[i];

    // fenced code block
    if (/^```/.test(line)) {
      flushParagraph(para);
      const lang = line.replace(/^```/, "").trim();
      const code: string[] = [];
      i++;
      while (i < lines.length && !/^```/.test(lines[i])) {
        code.push(lines[i]);
        i++;
      }
      i++; // skip closing fence (or EOF)
      const langAttr = lang ? ` class="language-${lang}"` : "";
      html.push(`<pre><code${langAttr}>${code.join("\n")}</code></pre>`);
      continue;
    }

    // heading
    const heading = line.match(/^(#{1,6})\s+(.*)$/);
    if (heading) {
      flushParagraph(para);
      const level = heading[1].length;
      html.push(`<h${level}>${inlineFormat(heading[2])}</h${level}>`);
      i++;
      continue;
    }

    // horizontal rule
    if (/^\s*(-{3,}|\*{3,}|_{3,})\s*$/.test(line)) {
      flushParagraph(para);
      html.push("<hr />");
      i++;
      continue;
    }

    // blockquote (group consecutive)
    if (/^&gt;\s?/.test(line)) {
      flushParagraph(para);
      const quote: string[] = [];
      while (i < lines.length && /^&gt;\s?/.test(lines[i])) {
        quote.push(lines[i].replace(/^&gt;\s?/, ""));
        i++;
      }
      html.push(`<blockquote>${inlineFormat(quote.join(" "))}</blockquote>`);
      continue;
    }

    // unordered list (group consecutive)
    if (/^\s*[-*+]\s+/.test(line)) {
      flushParagraph(para);
      const items: string[] = [];
      while (i < lines.length && /^\s*[-*+]\s+/.test(lines[i])) {
        items.push(
          `<li>${inlineFormat(lines[i].replace(/^\s*[-*+]\s+/, ""))}</li>`,
        );
        i++;
      }
      html.push(`<ul>${items.join("")}</ul>`);
      continue;
    }

    // ordered list (group consecutive)
    if (/^\s*\d+\.\s+/.test(line)) {
      flushParagraph(para);
      const items: string[] = [];
      while (i < lines.length && /^\s*\d+\.\s+/.test(lines[i])) {
        items.push(
          `<li>${inlineFormat(lines[i].replace(/^\s*\d+\.\s+/, ""))}</li>`,
        );
        i++;
      }
      html.push(`<ol>${items.join("")}</ol>`);
      continue;
    }

    // blank line → paragraph break
    if (line.trim() === "") {
      flushParagraph(para);
      i++;
      continue;
    }

    // default: accumulate paragraph text
    para.push(line);
    i++;
  }

  flushParagraph(para);
  return html.join("\n");
}
