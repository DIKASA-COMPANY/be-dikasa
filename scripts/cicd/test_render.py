"""Render deployment tests use in-memory API, migration and smoke fakes."""
import copy
import unittest

from render import deploy
from smoke import validate_url


REPOSITORY = 'ghcr.io/dikasa-company/be-dikasa'
OLD = REPOSITORY + '@sha256:' + 'a' * 64
NEW = REPOSITORY + '@sha256:' + 'b' * 64
SERVICE = {'type': 'web_service', 'suspended': 'not_suspended', 'imagePath': OLD,
           'serviceDetails': {'runtime': 'image', 'preDeployCommand': '',
                              'envSpecificDetails': {'dockerCommand': ''}}}


class FakeRender:
    def __init__(self, scenario='success'):
        self.scenario = scenario
        self.service = copy.deepcopy(SERVICE)
        self.latest = 'dep-old'
        self.images = {'dep-old': OLD}
        self.updates = []
        self.canceled = []

    def details(self, deployment):
        state = 'live'
        if deployment == 'dep-new':
            state = {'unhealthy': 'update_failed', 'timeout': 'update_in_progress'}.get(self.scenario, state)
        image = self.images[deployment]
        if self.scenario == 'wrong-digest' and deployment == 'dep-new':
            image = OLD
        return {'id': deployment, 'status': state,
                'image': {'ref': image, 'sha': image.split('@')[1]}}

    def __call__(self, method, path, body=None):
        if method == 'POST' and path.endswith('/cancel'):
            self.canceled.append(path)
            return {}
        if method == 'POST':
            self.updates.append(copy.deepcopy(body))
            self.latest = 'dep-new' if len(self.updates) == 1 else 'dep-restored'
            self.images[self.latest] = body['imageUrl']
            return {'id': self.latest}
        if path.endswith('/deploys?status=live&limit=1'):
            return [{'deploy': self.details('dep-old')}]
        if path.endswith('/deploys?limit=1'):
            if self.scenario == 'concurrent' and self.latest == 'dep-new':
                return [{'deploy': {'id': 'dep-external', 'status': 'live'}}]
            return [{'deploy': self.details(self.latest)}]
        if '/deploys/' in path:
            return self.details(path.rsplit('/', 1)[-1])
        return copy.deepcopy(self.service)


class RenderTests(unittest.TestCase):
    def run_deploy(self, api, migrate=lambda: None, smoke=lambda: None, rollback=False):
        deploy(api, 'srv-test', NEW, migrate, smoke, rollback=rollback,
               sleep=lambda _: None, attempts=2)

    def test_promotes_exact_digest_after_migration_without_changing_settings(self):
        api, events = FakeRender(), []
        self.run_deploy(api, lambda: events.append('migration'), lambda: events.append('smoke'))
        self.assertEqual(events, ['migration', 'smoke'])
        self.assertEqual(api.updates, [{'imageUrl': NEW}])
        self.assertEqual(api.service, SERVICE)

    def test_migration_failure_does_not_deploy(self):
        api = FakeRender()
        def fail(): raise RuntimeError('migration failed')
        with self.assertRaisesRegex(RuntimeError, 'migration failed'):
            self.run_deploy(api, migrate=fail)
        self.assertEqual(api.updates, [])

    def test_failed_timed_out_or_wrong_digest_deploy_restores_previous_image(self):
        for scenario in ['unhealthy', 'timeout', 'wrong-digest']:
            api = FakeRender(scenario)
            with self.subTest(scenario=scenario), self.assertRaises(RuntimeError):
                self.run_deploy(api)
            self.assertEqual(api.updates[-1], {'imageUrl': OLD})
            self.assertEqual(bool(api.canceled), scenario == 'timeout')

    def test_smoke_failure_restores_previous_image(self):
        api = FakeRender()
        def fail(): raise RuntimeError('smoke failed')
        with self.assertRaisesRegex(RuntimeError, 'smoke failed'):
            self.run_deploy(api, smoke=fail)
        self.assertEqual(api.updates, [{'imageUrl': NEW}, {'imageUrl': OLD}])

    def test_manual_rollback_skips_migrations(self):
        def fail(): self.fail('rollback must not migrate')
        self.run_deploy(FakeRender(), migrate=fail, rollback=True)

    def test_external_deployment_is_not_overwritten_by_recovery(self):
        api = FakeRender('concurrent')
        with self.assertRaisesRegex(RuntimeError, 'recovery requires manual attention'):
            self.run_deploy(api)
        self.assertEqual(len(api.updates), 1)

    def test_pending_deployment_is_rejected_before_migration(self):
        api = FakeRender('timeout')
        api.latest, api.images['dep-new'] = 'dep-new', NEW
        with self.assertRaises(RuntimeError):
            self.run_deploy(api)
        self.assertEqual(api.updates, [])

    def test_source_type_suspension_and_command_overrides_are_rejected(self):
        services = [dict(SERVICE, imagePath=None), dict(SERVICE, suspended='suspended'),
                    dict(SERVICE, type='background_worker')]
        for field, value in [('runtime', 'go'), ('preDeployCommand', 'migrate'),
                             ('envSpecificDetails', {'dockerCommand': 'other-app'})]:
            service = copy.deepcopy(SERVICE)
            service['serviceDetails'][field] = value
            services.append(service)
        for service in services:
            api = FakeRender()
            api.service = service
            with self.subTest(service=service), self.assertRaises(ValueError):
                self.run_deploy(api)
            self.assertEqual(api.updates, [])

    def test_mutable_or_different_repository_image_is_rejected(self):
        for image in ['example:latest', 'ghcr.io/other/api@sha256:' + 'b' * 64]:
            api = FakeRender()
            with self.subTest(image=image), self.assertRaises(ValueError):
                deploy(api, 'srv-test', image, lambda: None, lambda: None)
            self.assertEqual(api.updates, [])

    def test_service_change_during_migration_is_rejected(self):
        api = FakeRender()
        def change():
            api.latest = 'dep-external'
            api.images[api.latest] = OLD
        with self.assertRaisesRegex(RuntimeError, 'during migration'):
            self.run_deploy(api, migrate=change)
        self.assertEqual(api.updates, [])

    def test_previous_live_digest_is_required(self):
        api = FakeRender()
        def no_digest(method, path, body=None):
            result = api(method, path, body)
            if 'status=live' in path:
                result[0]['deploy']['image'] = {'ref': REPOSITORY + ':latest'}
            return result
        with self.assertRaises(ValueError):
            self.run_deploy(no_digest)
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
