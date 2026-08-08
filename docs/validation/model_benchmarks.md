# Riptide Model & Thinking Config Benchmark Report

**Date:** 2026-08-08 17:50:08  
**Task Prompt:** "Go to https://www.lotr.com/games/maze and start the game by focusing the canvas and pressing enter"  
**Max Turns:** 2  

## Matrix Summary

| Model | Thinking Config | Outcome | Turns | Wall (s) | Model (s) | Avg Turn (s) | Thought Tokens | Total Tokens |
|---|---|---|---|---|---|---|---|---|
| `gemini-3.5-flash` | `LOW` | `max_turns` | 2 | 12.00 | 7.57 | 6.00 | 123 | 9286 |
| `gemini-3.5-flash` | `HIGH` | `max_turns` | 2 | 14.00 | 8.46 | 7.00 | 163 | 9346 |
| `gemini-3.5-flash-lite` | `LOW` | `max_turns` | 2 | 9.00 | 4.76 | 4.50 | 0 | 9187 |
| `gemini-3.5-flash-lite` | `HIGH` | `max_turns` | 2 | 13.00 | 7.66 | 6.50 | 155 | 9392 |

## Model Averages

| Model | Runs | Avg Wall (s) | Avg Model (s) | Avg Turn Latency (s) | Avg Thought Tokens | Avg Total Tokens |
|---|---|---|---|---|---|---|
| `gemini-3.5-flash` | 2 | 13.00 | 8.02 | 6.50 | 143 | 9316 |
| `gemini-3.5-flash-lite` | 2 | 11.00 | 6.21 | 5.50 | 77 | 9289 |

## Recommendations

- **Latency-Sensitive Real-Time / Step-Locked Tasks:** Use `gemini-3.5-flash-lite` or `gemini-3.5-flash` with `ThinkingLevel=LOW` or `ThinkingBudget=2048` to minimize turn latency (~2–4s/turn).
- **Complex Multi-Step Reasoning / Audit Tasks:** Use `gemini-3.6-flash` or `gemini-3.5-flash` with `ThinkingLevel=HIGH` or `ThinkingBudget=8192` to maximize solution accuracy.
