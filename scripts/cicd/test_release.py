"""Regression tests for immutable releases and ordered environment promotion."""
import unittest
from urllib.parse import parse_qs, urlparse

from release import validate_manifest, validate_run, require_previous_success


SHA = "a" * 40
IMAGE = "ghcr.io/dikasa-company/be-dikasa"
MANIFEST = {"schema": 1, "repository": "DIKASA-COMPANY/be-dikasa",
            "source_sha": SHA, "run_id": 42, "image": IMAGE + "@sha256:" + "b" * 64}
RUN = {"id": 42, "path": ".github/workflows/be-release.yml", "event": "push",
       "head_branch": "main", "head_sha": SHA, "status": "completed",
       "conclusion": "success", "repository": {"full_name": MANIFEST["repository"]}}


class ReleaseTests(unittest.TestCase):
    def test_only_successful_main_release_is_promotable(self):
        validate_run(RUN, MANIFEST["repository"], 42)
        for field, value in [("path", ".github/workflows/be-ci.yml"), ("event", "pull_request"),
                             ("head_branch", "feature/test"), ("conclusion", "failure"),
                             ("status", "in_progress"), ("id", 43)]:
            with self.subTest(field=field), self.assertRaises(ValueError):
                validate_run(dict(RUN, **{field: value}), MANIFEST["repository"], 42)

    def test_dev_can_use_current_running_release_only(self):
        validate_run(dict(RUN, status="in_progress", conclusion=None), MANIFEST["repository"],
                     42, current_release=True)
        with self.assertRaises(ValueError):
            validate_run(dict(RUN, conclusion="failure"), MANIFEST["repository"],
                         42, current_release=True)

    def test_manifest_must_match_release_and_registry(self):
        validate_manifest(MANIFEST, RUN, IMAGE)
        for field, value in [("schema", 2), ("run_id", 43), ("source_sha", "c" * 40),
                             ("repository", "other/repo"), ("image", IMAGE + ":latest"),
                             ("image", "evil.example/api@sha256:" + "b" * 64)]:
            with self.subTest(field=field), self.assertRaises(ValueError):
                validate_manifest(dict(MANIFEST, **{field: value}), RUN, IMAGE)

    def test_each_environment_requires_previous_success_of_same_release(self):
        for target, previous in [("sit", "dev"), ("uat", "sit"), ("stg", "uat"), ("prd", "stg")]:
            requested = []

            def get(path):
                requested.append(path)
                if "/statuses" in path:
                    return [{"state": "success"}]
                return [{"id": 9, "payload": dict(MANIFEST)}]

            require_previous_success(get, MANIFEST["repository"], MANIFEST, target)
            self.assertIn("environment=" + previous, requested[0])

    def test_missing_failed_or_different_release_cannot_skip_gate(self):
        for deployments, statuses in [([], []), ([{"id": 9, "payload": MANIFEST}], [{"state": "failure"}]),
                                      ([{"id": 9, "payload": dict(MANIFEST, run_id=99)}], [{"state": "success"}]),
                                      ([{"id": 9, "payload": dict(MANIFEST, image=IMAGE + ":latest")}], [{"state": "success"}])]:
            def get(path):
                return statuses if "/statuses" in path else deployments
            with self.subTest(deployments=deployments, statuses=statuses), self.assertRaises(ValueError):
                require_previous_success(get, MANIFEST["repository"], MANIFEST, "prd")

    def test_history_is_paginated_and_malformed_payload_is_ignored(self):
        def get(path):
            if "/statuses" in path:
                return [{"state": "success"}]
            if parse_qs(urlparse(path).query)["page"] == ["1"]:
                return [{"id": 1, "payload": "not our record"}] * 100
            return [{"id": 2, "payload": MANIFEST}]
        require_previous_success(get, MANIFEST["repository"], MANIFEST, "sit")

    def test_rollback_requires_success_in_target_environment(self):
        requested = []
        def get(path):
            requested.append(path)
            return [{"state": "success"}] if "/statuses" in path else [{"id": 9, "payload": MANIFEST}]
        require_previous_success(get, MANIFEST["repository"], MANIFEST, "prd", rollback=True)
        self.assertIn("environment=prd", requested[0])

    def test_invalid_target_is_rejected(self):
        with self.assertRaises(ValueError):
            require_previous_success(lambda _: [], MANIFEST["repository"], MANIFEST, "production")


if __name__ == "__main__":
    unittest.main()
