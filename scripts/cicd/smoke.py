"""Exercise the deployed contract, default-deny auth, login and authenticated health."""
import json
import os
from urllib.error import HTTPError
from urllib.parse import urlsplit
from urllib.request import Request, urlopen


def validate_url(value):
    parsed = urlsplit(value)
    if (parsed.scheme != 'https' or not parsed.hostname or parsed.username or parsed.password
            or parsed.query or parsed.fragment or parsed.path not in ('', '/')
            or any(character.isspace() for character in value)):
        raise ValueError('Smoke URL must be an HTTPS service origin without credentials, query or path')
    return value.rstrip('/')


def main():
    base = validate_url(os.environ["SMOKE_URL"])

    def request(path, expected, body=None, token=None):
        headers = {"Content-Type": "application/json"}
        if token:
            headers["Authorization"] = "Bearer " + token
        call = Request(base + "/dikasa/v1" + path, headers=headers,
                       data=json.dumps(body).encode() if body is not None else None)
        try:
            with urlopen(call, timeout=30) as response:
                status, payload = response.status, json.load(response)
        except HTTPError as error:
            status, payload = error.code, None
        if status != expected:
            # Never include response bodies, passwords or tokens in CI logs.
            raise RuntimeError(f"Smoke {path}: expected HTTP {expected}, received {status}")
        return payload

    request("/openapi.json", 200)
    request("/health", 401)
    request("/health", 401, token="invalid-smoke-token")
    login = request("/login", 200, {"identifier": os.environ["SMOKE_USERNAME"],
                                    "password": os.environ["SMOKE_PASSWORD"]})
    health = request("/health", 200, token=login["data"]["access_token"])
    if health.get("status") != "ok" or health.get("service") != "be-dikasa":
        raise RuntimeError("Unexpected authenticated health response")
    print("Smoke passed: OpenAPI, missing/invalid auth, login and authenticated health")


if __name__ == "__main__":
    main()
