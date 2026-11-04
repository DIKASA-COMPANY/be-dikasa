"""Validate build provenance and track successful digest promotions in GitHub Deployments."""
import json
import os
from pathlib import Path
import re
import sys
from urllib.parse import urlencode
from urllib.request import Request, urlopen


ENVIRONMENTS = ("dev", "sit", "uat", "stg", "prd")


def validate_run(run, repository, run_id, current_release=False):
    """Only a main push through the release workflow can supply deployable images."""
    valid = (run["id"] == int(run_id) and run["repository"]["full_name"] == repository
             and run["path"] == ".github/workflows/be-release.yml"
             and run["event"] == "push" and run["head_branch"] == "main"
             and re.fullmatch(r"[0-9a-f]{40}", run["head_sha"]))
    finished = run["status"] == "completed" and run["conclusion"] == "success"
    if not valid or not (finished or (current_release and run["status"] == "in_progress")):
        raise ValueError("Expected a successful main push from be-release.yml")


def validate_manifest(manifest, run, image_repository):
    """Reject mutable tags and manifests detached from the selected build run."""
    if (manifest.get("schema") != 1 or manifest.get("repository") != run["repository"]["full_name"]
            or manifest.get("run_id") != run["id"] or manifest.get("source_sha") != run["head_sha"]
            or not re.fullmatch(re.escape(image_repository) + r"@sha256:[0-9a-f]{64}", manifest.get("image", ""))):
        raise ValueError("Manifest does not match the release run or repository GHCR digest")


def require_previous_success(get, repository, manifest, target, rollback=False):
    """Require a successful record for this exact release in the preceding environment."""
    if target not in ENVIRONMENTS:
        raise ValueError("Unknown environment")
    if target == "dev" and not rollback:
        return
    previous = target if rollback else ENVIRONMENTS[ENVIRONMENTS.index(target) - 1]
    page = 1
    while True:
        query = urlencode({"environment": previous, "sha": manifest["source_sha"],
                           "task": "dikasa-release", "per_page": 100, "page": page})
        deployments = get(f"/repos/{repository}/deployments?{query}")
        for deployment in deployments:
            if deployment.get("payload") != manifest:
                continue
            statuses = get(f"/repos/{repository}/deployments/{deployment['id']}/statuses?per_page=1")
            if statuses and statuses[0]["state"] == "success":
                return
        if len(deployments) < 100:
            break
        page += 1
    raise ValueError(f"Release has no successful deployment in {previous}")


def api(path, data=None):
    """Use the job token without printing credentials or sensitive response bodies."""
    request = Request(os.environ.get("GITHUB_API_URL", "https://api.github.com") + path,
                      data=json.dumps(data).encode() if data is not None else None,
                      headers={"Authorization": "Bearer " + os.environ["GH_TOKEN"],
                               "Accept": "application/vnd.github+json",
                               "X-GitHub-Api-Version": "2022-11-28"})
    with urlopen(request, timeout=30) as response:
        return json.load(response)


def output(**values):
    with open(os.environ["GITHUB_OUTPUT"], "a", encoding="utf-8") as destination:
        for key, value in values.items():
            destination.write(f"{key}={value}\n")


def main():
    command = sys.argv[1]
    repository = os.environ["GITHUB_REPOSITORY"]
    image_repository = "ghcr.io/" + repository.lower()
    run_id = int(os.environ["RELEASE_RUN_ID"])
    if command == "manifest":
        manifest = {"schema": 1, "repository": repository, "source_sha": os.environ["GITHUB_SHA"],
                    "run_id": run_id, "image": image_repository + "@" + os.environ["IMAGE_DIGEST"]}
        Path("release.json").write_text(json.dumps(manifest, indent=2) + "\n", encoding="utf-8")
        return
    run = api(f"/repos/{repository}/actions/runs/{run_id}")
    current = (os.environ.get("TARGET_ENV") == "dev" and str(run_id) == os.environ["GITHUB_RUN_ID"]
               and os.environ.get("GITHUB_WORKFLOW_REF") == repository + "/.github/workflows/be-release.yml@refs/heads/main")
    validate_run(run, repository, run_id, current_release=current)
    if command == "check-run":
        output(source_sha=run["head_sha"])
        return
    manifest = json.loads(Path("release.json").read_text(encoding="utf-8"))
    validate_manifest(manifest, run, image_repository)
    target = os.environ["TARGET_ENV"]
    if command == "gate":
        if target == "dev" and not current and os.environ.get("ROLLBACK") != "true":
            raise ValueError("DEV forward deployment is reserved for the release workflow")
        require_previous_success(api, repository, manifest, target, os.environ.get("ROLLBACK") == "true")
        output(image=manifest["image"], source_sha=manifest["source_sha"])
    elif command == "record":
        if target not in ENVIRONMENTS:
            raise ValueError("Unknown environment")
        deployment = api(f"/repos/{repository}/deployments", {
            "ref": manifest["source_sha"], "environment": target, "task": "dikasa-release",
            "auto_merge": False, "required_contexts": [], "payload": manifest,
            "production_environment": target == "prd", "description": "Tested immutable backend release"})
        api(f"/repos/{repository}/deployments/{deployment['id']}/statuses", {
            "state": "success", "auto_inactive": False,
            "log_url": f"{os.environ['GITHUB_SERVER_URL']}/{repository}/actions/runs/{os.environ['GITHUB_RUN_ID']}",
            "environment_url": os.environ["SERVICE_URL"]})
    else:
        raise ValueError("Unknown command")


if __name__ == "__main__":
    main()
