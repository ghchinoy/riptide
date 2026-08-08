# Image Generation Prompts

A log of prompts used to generate illustrations for Riptide documentation and blog posts. Reuse or adapt these to maintain visual consistency across the series.

## Style Notes

The header images use a consistent aesthetic: **minimal flat design, soft blues and greens, white background, tech blog scale (16:9), no text labels in the image itself**. Alt text and captions carry the descriptive load; the image handles the visual mood.

Model: `gemini-3.1-flash-image-preview` (NanoBanana) via MCP, converted PNG → WebP with `cwebp -q 90`.

---

## Post 4: Sandboxing Browser Agents

**Output:** `docs/sandboxing_header.webp`  
**Aspect ratio:** 16:9  
**Generated:** 2026-06-26

**Prompt:**
> A minimal, modern technical illustration for a blog post about sandboxing browser automation agents. Show a stylized Go gopher sitting inside a transparent protective container or vault, with a Chrome browser icon visible through the container walls. Outside the container: a macOS window, a Docker whale, a Google Cloud logo. Clean flat design, soft blues and greens, white background, tech blog aesthetic. No text labels.

**Notes:** The gopher-in-container metaphor landed well. The transparent vault with visible contents is a good visual pattern for any "isolation" or "containment" topic. Reuse the vault framing for future safety or deployment posts.

---

## Prompt Templates for Future Posts

### Post 1: Migration (if a header is wanted)

> A minimal, modern technical illustration for a blog post about migrating a Go browser automation agent to a new AI model. Show a stylized Go gopher holding a wrench, standing between two versions of a code block or API config. Subtle arrow indicating upgrade direction. Clean flat design, soft blues and greens, white background, tech blog aesthetic. No text labels.

### Post 2: The Harness (Go vs Python comparison)

> A minimal, modern technical illustration for a blog post comparing a Go agent harness with a Python reference implementation. Show two stylized figures: a Go gopher on the left operating a well-organized control panel with clean dials and indicators; a Python snake on the right with a simpler console. Clean flat design, soft blues and greens on the Go side, warmer tones on the Python side, white background, tech blog aesthetic. No text labels.

### Post 3: Safety Best Practices

> A minimal, modern technical illustration for a blog post about computer use agent safety. Show a stylized Go gopher at a checkpoint gate reviewing a browser window before it passes through. A warning shield icon on the gate. Clean flat design, yellows and blues, white background, tech blog aesthetic. No text labels.

### Post 5: Benchmarking

> A minimal, modern technical illustration for a blog post about benchmarking AI browser agents. Show a stylized Go gopher at a finish line with a stopwatch, a browser window showing a task completion, and a simple bar chart floating nearby. Clean flat design, soft greens and blues, white background, tech blog aesthetic. No text labels.

### Post 6: BDD Testing Agent

> A minimal, modern technical illustration for a blog post about using a browser automation agent to write and run BDD tests. Show a stylized Go gopher at a clipboard writing Gherkin Given/When/Then steps, while a browser window with a green checkmark floats above it. Clean flat design, soft greens, white background, tech blog aesthetic. No text labels.
