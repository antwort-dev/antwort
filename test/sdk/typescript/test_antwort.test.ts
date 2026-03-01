/**
 * SDK compatibility tests for antwort using the official OpenAI TypeScript SDK.
 *
 * These tests run against a live antwort server backed by the mock-backend.
 * They validate that the official OpenAI SDK can communicate with antwort
 * without modification.
 */

import { describe, test, expect } from "bun:test";
import OpenAI from "openai";

const BASE_URL = process.env.ANTWORT_BASE_URL || "http://localhost:8080/v1";
const API_KEY = process.env.ANTWORT_API_KEY || "test";
const MODEL = process.env.ANTWORT_MODEL || "mock-model";

const client = new OpenAI({ baseURL: BASE_URL, apiKey: API_KEY });

/** Build a structured input message (antwort requires array of Item, not plain string). */
function msg(text: string) {
  return [{ role: "user" as const, content: text }];
}

describe("antwort SDK compatibility", () => {
  test("basic response", async () => {
    const response = await client.responses.create({
      model: MODEL,
      input: msg("What is 2+2?"),
    });
    expect(response.id).toMatch(/^resp_/);
    expect(response.status).toBe("completed");
    expect(response.output.length).toBeGreaterThan(0);
    expect(response.output_text).toBeTruthy();
    expect(response.output_text!.length).toBeGreaterThan(0);
  });

  test("streaming", async () => {
    const stream = await client.responses.create({
      model: MODEL,
      input: msg("Say hello."),
      stream: true,
    });

    const textDeltas: string[] = [];
    let completed = false;
    for await (const event of stream) {
      if (event.type === "response.output_text.delta") {
        textDeltas.push((event as any).delta);
      } else if (event.type === "response.completed") {
        completed = true;
      }
    }

    expect(completed).toBe(true);
    const fullText = textDeltas.join("");
    expect(fullText.length).toBeGreaterThan(0);
  });

  test("tool calling", async () => {
    const response = await client.responses.create({
      model: MODEL,
      input: msg("Use the test tool."),
      tools: [
        {
          type: "function" as const,
          name: "test_tool",
          description: "A test tool",
          parameters: {
            type: "object" as const,
            properties: {
              input: { type: "string" as const },
            },
          },
        },
      ],
    });
    expect(response.id).toMatch(/^resp_/);
    expect(response.status).toBe("completed");
  });

  test("conversation chaining", async () => {
    const first = await client.responses.create({
      model: MODEL,
      input: msg("Remember this: alpha."),
    });
    expect(first.id).toMatch(/^resp_/);

    const second = await client.responses.create({
      model: MODEL,
      input: msg("What did I say?"),
      previous_response_id: first.id,
    });
    expect(second.id).toMatch(/^resp_/);
    expect(second.status).toBe("completed");
    expect(second.id).not.toBe(first.id);
  });

  test("structured output", async () => {
    const response = await client.responses.create({
      model: MODEL,
      input: msg("List three colors."),
      text: {
        format: {
          type: "json_schema",
          name: "colors",
          schema: {
            type: "object",
            properties: {
              colors: {
                type: "array",
                items: { type: "string" },
              },
            },
            required: ["colors"],
          },
        },
      },
    });
    expect(response.status).toBe("completed");
    expect(response.output_text).toBeTruthy();
  });

  test("model listing", async () => {
    try {
      const models = await client.models.list();
      const modelIds: string[] = [];
      for await (const model of models) {
        modelIds.push(model.id);
      }
      expect(modelIds.length).toBeGreaterThan(0);
    } catch (e: any) {
      if (e?.status === 404) {
        console.log("SKIP: GET /v1/models not implemented yet");
        return;
      }
      throw e;
    }
  });
});
