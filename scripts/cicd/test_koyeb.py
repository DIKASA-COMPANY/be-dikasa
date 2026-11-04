"""Koyeb orchestration tests use only in-memory API, migration and smoke fakes."""
import copy
import unittest

from koyeb import deploy
from smoke import validate_url


OLD = 'ghcr.io/dikasa-company/be-dikasa@sha256:' + 'a' * 64
NEW = 'ghcr.io/dikasa-company/be-dikasa@sha256:' + 'b' * 64
DEFINITION = {'name': 'api', 'docker': {'image': OLD, 'image_registry_secret': 'ghcr-reader'},
              'env': [{'key': 'DATABASE_URL', 'secret': 'database-dev'}],
              'regions': ['sin'], 'ports': [{'port': 8080, 'protocol': 'http'}]}


class FakeKoyeb:
    def __init__(self, scenario='success'):
        self.scenario = scenario
        self.latest = 'old'
        self.definitions = {'old': copy.deepcopy(DEFINITION)}
        self.updates = []

    def __call__(self, method, path, body=None):
        if method == 'PUT':
            self.updates.append(copy.deepcopy(body))
            self.latest = 'new' if len(self.updates) == 1 else 'restored'
            self.definitions[self.latest] = copy.deepcopy(body['definition'])
            return {'service': {'latest_deployment_id': self.latest}}
        if '/deployments/' in path:
            deployment = path.rsplit('/', 1)[-1]
            status = 'ERROR' if self.scenario == 'unhealthy' and deployment == 'new' else 'HEALTHY'
            if self.scenario == 'timeout' and deployment == 'new':
                status = 'PROVISIONING'
            return {'deployment': {'status': status, 'definition': self.definitions[deployment]}}
        latest = 'external' if self.scenario == 'concurrent' and self.latest == 'new' else self.latest
        return {'service': {'active_deployment_id': self.latest, 'latest_deployment_id': latest,
                            'status': 'HEALTHY'}}


class KoyebTests(unittest.TestCase):
    def run_deploy(self, api, migrate=lambda: None, smoke=lambda: None, rollback=False):
        deploy(api, 'service-id', NEW, migrate, smoke, rollback=rollback,
               sleep=lambda _: None, attempts=2)

    def test_promotes_digest_after_migration_and_preserves_service_settings(self):
        api, events = FakeKoyeb(), []
        self.run_deploy(api, lambda: events.append('migration'), lambda: events.append('smoke'))
        self.assertEqual(events, ['migration', 'smoke'])
        expected = copy.deepcopy(DEFINITION)
        expected['docker']['image'] = NEW
        self.assertEqual(api.updates, [{'definition': expected}])

    def test_migration_failure_does_not_update_service(self):
        api = FakeKoyeb()
        def fail(): raise RuntimeError('migration failed')
        with self.assertRaisesRegex(RuntimeError, 'migration failed'):
            self.run_deploy(api, migrate=fail)
        self.assertEqual(api.updates, [])

    def test_unhealthy_or_timed_out_deployment_restores_previous_definition(self):
        for scenario in ['unhealthy', 'timeout']:
            api = FakeKoyeb(scenario)
            with self.subTest(scenario=scenario), self.assertRaises(RuntimeError):
                self.run_deploy(api)
            self.assertEqual(api.updates[-1], {'definition': DEFINITION})

    def test_smoke_failure_restores_previous_definition(self):
        api = FakeKoyeb()
        def fail(): raise RuntimeError('smoke failed')
        with self.assertRaisesRegex(RuntimeError, 'smoke failed'):
            self.run_deploy(api, smoke=fail)
        self.assertEqual(len(api.updates), 2)
        self.assertEqual(api.updates[-1], {'definition': DEFINITION})

    def test_manual_rollback_skips_migrations(self):
        api = FakeKoyeb()
        def fail(): self.fail('rollback must not migrate')
        self.run_deploy(api, migrate=fail, rollback=True)
        self.assertEqual(len(api.updates), 1)

    def test_recovery_does_not_overwrite_external_deployment(self):
        api = FakeKoyeb('concurrent')
        with self.assertRaisesRegex(RuntimeError, 'recovery requires manual attention'):
            self.run_deploy(api)
        self.assertEqual(len(api.updates), 1)

    def test_mutable_previous_image_is_rejected_before_migration(self):
        api = FakeKoyeb()
        api.definitions['old']['docker']['image'] = 'example:latest'
        with self.assertRaises(ValueError):
            self.run_deploy(api)
        self.assertEqual(api.updates, [])

    def test_mutable_new_image_is_rejected(self):
        with self.assertRaises(ValueError):
            deploy(FakeKoyeb(), 'service-id', 'example:latest', lambda: None, lambda: None)

    def test_pending_service_change_is_rejected(self):
        api = FakeKoyeb()
        def pending(method, path, body=None):
            response = api(method, path, body)
            if path.startswith('/v1/services/') and method == 'GET':
                response['service']['latest_deployment_id'] = 'pending'
            return response
        with self.assertRaises(RuntimeError):
            self.run_deploy(pending)
        self.assertEqual(api.updates, [])

    def test_source_overrides_are_rejected_before_mutation(self):
        for key, value in [('command', '/other/app'), ('args', ['--different']), ('entrypoint', ['/bin/sh'])]:
            api = FakeKoyeb()
            api.definitions['old']['docker'][key] = value
            with self.subTest(key=key), self.assertRaises(ValueError):
                self.run_deploy(api)
            self.assertEqual(api.updates, [])

    def test_smoke_credentials_only_go_to_an_https_origin(self):
        self.assertEqual(validate_url('https://api.example/'), 'https://api.example')
        for value in ['http://api.example', 'https://user:password@api.example',
                      'https://api.example?token=x', 'https://api.example/path',
                      'https://api.example\nkey=value', 'https://api.example#fragment']:
            with self.subTest(value=value), self.assertRaises(ValueError):
                validate_url(value)


if __name__ == '__main__':
    unittest.main()
