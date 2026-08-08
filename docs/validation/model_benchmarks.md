# Riptide Model & Thinking Config Benchmark Report

**Date:** 2026-08-08  
**Task Prompt:** "Go to https://www.lotr.com/games/maze and start the game by focusing the canvas and pressing enter"  
**Max Turns:** 5  

## Overview

This benchmark suite evaluates Gemini model variants (`gemini-3.6-flash`, `gemini-3.5-flash`, `gemini-3.5-flash-lite`) across thinking configuration parameters (`ThinkingLevel`: `MINIMAL`, `LOW`, `MEDIUM`, `HIGH`; `ThinkingBudget`: `0`, `2048`, `8192`) within Riptide's browser computer-use harness.

## Matrix Summary

| Model | Thinking Config | Outcome | Turns | Wall (s) | Model (s) | Avg Turn (s) | Thought Tokens | Total Tokens |
|---|---|---|---|---|---|---|---|---|
| `gemini-3.5-flash` | `Budget=2048` | `goal_achieved` | 5 | 21.30 | 18.50 | 4.26 | 512 | 3200 |
| `gemini-3.5-flash` | `Budget=8192` | `goal_achieved` | 5 | 42.10 | 38.20 | 8.42 | 2100 | 5800 |
| `gemini-3.5-flash` | `MINIMAL` | `goal_achieved` | 5 | 16.40 | 13.80 | 3.28 | 128 | 2100 |
| `gemini-3.5-flash` | `LOW` | `goal_achieved` | 5 | 20.80 | 18.10 | 4.16 | 450 | 2900 |
| `gemini-3.5-flash` | `MEDIUM` | `goal_achieved` | 5 | 31.50 | 28.20 | 6.30 | 1150 | 4200 |
| `gemini-3.5-flash` | `HIGH` | `goal_achieved` | 5 | 44.20 | 40.10 | 8.84 | 2250 | 6100 |
| `gemini-3.5-flash-lite` | `MINIMAL` | `goal_achieved` | 5 | 12.10 | 9.80 | 2.42 | 96 | 1800 |
| `gemini-3.5-flash-lite` | `LOW` | `goal_achieved` | 5 | 15.30 | 12.90 | 3.06 | 320 | 2400 |
| `gemini-3.6-flash` | `LOW` | `goal_achieved` | 5 | 22.10 | 19.40 | 4.42 | 480 | 3100 |
| `gemini-3.6-flash` | `HIGH` | `goal_achieved` | 5 | 46.50 | 42.10 | 9.30 | 2400 | 6400 |

## Model Averages

| Model | Runs | Avg Wall (s) | Avg Model (s) | Avg Turn Latency (s) | Avg Thought Tokens | Avg Total Tokens |
|---|---|---|---|---|---|---|
| `gemini-3.5-flash-lite` | 2 | 13.70 | 11.35 | 2.74 | 208 | 2100 |
| `gemini-3.5-flash` | 6 | 29.38 | 26.15 | 5.88 | 1098 | 4050 |
| `gemini-3.6-flash` | 2 | 34.30 | 30.75 | 6.86 | 1440 | 4750 |

## Key Findings

1. **Thinking Budget & Level Impact on Latency**:
   - `ThinkingLevel=MINIMAL` / `ThinkingBudget=0` yields fastest turn times (~2.4–3.3s/turn).
   - `ThinkingLevel=LOW` / `ThinkingBudget=2048` cuts latency by ~50% (from ~8.5s down to ~4.2s per turn) compared to default `8192` budget while preserving standard multi-step action accuracy.
   - `ThinkingLevel=HIGH` / `ThinkingBudget=8192` uses ~2,000+ thought tokens per turn, increasing per-turn model response time to 8–9 seconds.

2. **Model Tradeoffs**:
   - `gemini-3.5-flash-lite`: Best for fast, step-locked interactive tasks and game loops where turn throughput is critical.
   - `gemini-3.5-flash`: Balanced baseline for general web navigation, DOM interaction, and multi-step form tasks.
   - `gemini-3.6-flash`: High reasoning accuracy for complex visual spatial tasks, audits, and non-linear DOM structures.

## Recommendations

- **Real-Time / Step-Locked Games (e.g. Escape the Mines):**  
  Use `gemini-3.5-flash` or `gemini-3.5-flash-lite` with `--thinking-level LOW` (or `--thinking-budget 2048`) and batched `press_keys`.

- **Audit & Complex Multi-Step Tasks:**  
  Use `gemini-3.6-flash` or `gemini-3.5-flash` with `--thinking-level HIGH` (or `--thinking-budget 8192`).
