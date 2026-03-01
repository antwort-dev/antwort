"""SDK compatibility tests for antwort using the official OpenAI Python SDK.

These tests run against a live antwort server backed by the mock-backend.
They validate that the official OpenAI SDK can communicate with antwort
without modification.

Environment variables:
    ANTWORT_BASE_URL: Base URL of the antwort server (default: http://localhost:8080/v1)
    ANTWORT_API_KEY:  API key for authentication (default: test)
    ANTWORT_MODEL:    Model identifier (default: mock-model)
"""

import json
import os

import pytest
from openai import OpenAI

BASE_URL = os.environ.get("ANTWORT_BASE_URL", "http://localhost:8080/v1")
API_KEY = os.environ.get("ANTWORT_API_KEY", "test")
MODEL = os.environ.get("ANTWORT_MODEL", "mock-model")


def _msg(text):
    """Build a structured input message (antwort requires array of Item, not plain string)."""
    return [{"role": "user", "content": text}]


@pytest.fixture
def client():
    return OpenAI(base_url=BASE_URL, api_key=API_KEY)


def test_basic_response(client):
    """Client can create a non-streaming response and read the output text."""
    response = client.responses.create(
        model=MODEL,
        input=_msg("What is 2+2?"),
    )
    assert response.id.startswith("resp_")
    assert response.status == "completed"
    assert len(response.output) > 0
    assert response.output_text is not None
    assert len(response.output_text) > 0


def test_streaming(client):
    """Client can stream a response and reconstruct the full text from deltas."""
    stream = client.responses.create(
        model=MODEL,
        input=_msg("Say hello."),
        stream=True,
    )

    text_deltas = []
    completed = False
    for event in stream:
        if event.type == "response.output_text.delta":
            text_deltas.append(event.delta)
        elif event.type == "response.completed":
            completed = True

    assert completed, "Stream did not produce a response.completed event"
    full_text = "".join(text_deltas)
    assert len(full_text) > 0, "No text deltas received"


def test_tool_calling(client):
    """Client can send a request with tools and receive a response."""
    response = client.responses.create(
        model=MODEL,
        input=_msg("Use the test tool."),
        tools=[
            {
                "type": "function",
                "name": "test_tool",
                "description": "A test tool",
                "parameters": {
                    "type": "object",
                    "properties": {
                        "input": {"type": "string"},
                    },
                },
            }
        ],
    )
    assert response.id.startswith("resp_")
    assert response.status == "completed"


def test_conversation_chaining(client):
    """Client can chain responses using previous_response_id."""
    first = client.responses.create(
        model=MODEL,
        input=_msg("Remember this: alpha."),
    )
    assert first.id.startswith("resp_")

    second = client.responses.create(
        model=MODEL,
        input=_msg("What did I say?"),
        previous_response_id=first.id,
    )
    assert second.id.startswith("resp_")
    assert second.status == "completed"
    assert second.id != first.id


def test_structured_output(client):
    """Client can request structured JSON output via text.format."""
    response = client.responses.create(
        model=MODEL,
        input=_msg("List three colors."),
        text={
            "format": {
                "type": "json_schema",
                "name": "colors",
                "schema": {
                    "type": "object",
                    "properties": {
                        "colors": {
                            "type": "array",
                            "items": {"type": "string"},
                        }
                    },
                    "required": ["colors"],
                },
            }
        },
    )
    assert response.status == "completed"
    assert response.output_text is not None


def test_model_listing(client):
    """Client can list available models."""
    try:
        models = client.models.list()
        model_ids = [m.id for m in models]
        assert len(model_ids) > 0, "No models returned"
    except Exception as e:
        if "404" in str(e):
            pytest.skip("GET /v1/models not implemented yet")
        raise
