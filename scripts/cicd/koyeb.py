"""Deploy an immutable image through Koyeb's API and restore failed updates."""
import copy
import json
import os
import re
import subprocess
import time
from urllib.error import HTTPError, URLError
from urllib.request import Request, urlopen
from uuid import UUID

import smoke


def pinned(image):
    return isinstance(image, str) and re.fullmatch(r'[^\s]+@sha256:[0-9a-f]{64}', image)


def wait_healthy(api, service_id, deployment_id, sleep, attempts):
    """Require the requested deployment to be healthy and active, not an older revision."""
    failed = {'ERROR', 'DEGRADED', 'UNHEALTHY', 'CANCELED', 'STOPPED', 'ERRORING'}
    for _ in range(attempts):
        deployment = api('GET', f'/v1/deployments/{deployment_id}')['deployment']
        if deployment.get('status') in failed:
            raise RuntimeError('Koyeb deployment ended unhealthy')
        service = api('GET', f'/v1/services/{service_id}')['service']
        if service.get('latest_deployment_id') != deployment_id:
            raise RuntimeError('Service changed outside this deployment')
        if (deployment.get('status') == 'HEALTHY'
                and service.get('active_deployment_id') == deployment_id
                and service.get('status') == 'HEALTHY'):
            return
        sleep(5)
    raise RuntimeError('Timed out waiting for the requested Koyeb deployment')


def deploy(api, service_id, image, migrate, smoke_test, rollback=False,
           sleep=time.sleep, attempts=120):
    """Preserve all existing settings and recover the previous definition on known failures."""
    if not pinned(image):
        raise ValueError('Deployment image must use a sha256 digest')
    path = f'/v1/services/{service_id}'
    previous_service = api('GET', path)['service']
    previous_id = previous_service.get('active_deployment_id')
    if (not previous_id or previous_service.get('latest_deployment_id') != previous_id
            or previous_service.get('status') != 'HEALTHY'):
        raise RuntimeError('Provision a healthy service with no pending or saved changes first')
    previous = api('GET', f'/v1/deployments/{previous_id}')['deployment']['definition']
    if (not pinned(previous.get('docker', {}).get('image'))
            or any(previous.get(source) for source in ['git', 'archive', 'database'])):
        raise ValueError('Existing service must use a pinned Docker image, without Git autodeploy')
    if any(previous['docker'].get(key) for key in ['entrypoint', 'command', 'args']):
        raise ValueError('Remove Docker entrypoint/command/args overrides before deployment')
    definition = copy.deepcopy(previous)
    definition['docker']['image'] = image
    if not rollback:
        migrate()
    # Detect changes made in the console while migrations were running.
    if api('GET', path)['service'].get('latest_deployment_id') != previous_id:
        raise RuntimeError('Service changed during migration; deployment aborted')
    updated_id = None
    try:
        updated = api('PUT', path, {'definition': definition})['service']
        updated_id = updated['latest_deployment_id']
        if not updated_id or updated_id == previous_id:
            raise RuntimeError('Koyeb did not return a new deployment ID')
        wait_healthy(api, service_id, updated_id, sleep, attempts)
        smoke_test()
    except Exception:
        if updated_id and updated_id != previous_id:
            try:
                current = api('GET', path)['service']
                if current.get('latest_deployment_id') != updated_id:
                    raise RuntimeError('A newer deployment exists; refusing to overwrite it')
                restored = api('PUT', path, {'definition': previous})['service']['latest_deployment_id']
                wait_healthy(api, service_id, restored, sleep, attempts)
                print('Previous Koyeb service definition restored')
            except Exception as recovery:
                raise RuntimeError('Deployment failed; recovery requires manual attention') from recovery
        # An update request without a response has an unknown outcome; never blindly retry it.
        raise


def api(method, path, body=None):
    """Do not expose API tokens, service definitions or provider response bodies in logs."""
    request = Request('https://app.koyeb.com' + path, method=method,
                      data=json.dumps(body).encode() if body is not None else None,
                      headers={'Authorization': 'Bearer ' + os.environ['KOYEB_API_TOKEN'],
                               'Content-Type': 'application/json', 'Accept': 'application/json'})
    try:
        with urlopen(request, timeout=30) as response:
            return json.load(response)
    except HTTPError as error:
        raise RuntimeError(f'Koyeb API request failed (HTTP {error.code})') from None
    except URLError:
        raise RuntimeError('Koyeb API connection failed; inspect deployment state before retrying') from None


def migrate_image(image):
    """Run the migration shipped in the same digest; pass only the database secret to Docker."""
    environment = dict(os.environ, DATABASE_URL=os.environ['MIGRATION_DATABASE_URL'])
    subprocess.run(['docker', 'pull', image], check=True, timeout=300)
    subprocess.run(['docker', 'run', '--rm', '--env', 'DATABASE_URL', '--entrypoint', '/bin/sh',
                    image, '/app/deploy/migrate.sh'], env=environment, check=True, timeout=600)


def main():
    service_id = str(UUID(os.environ['KOYEB_SERVICE_ID']))
    image = os.environ['RELEASE_IMAGE']
    rollback = os.environ.get('ROLLBACK') == 'true'
    for key in ['KOYEB_API_TOKEN', 'SMOKE_URL', 'SMOKE_USERNAME', 'SMOKE_PASSWORD']:
        if not os.environ.get(key):
            raise ValueError(f'Missing {key}')
    if not rollback and not os.environ.get('MIGRATION_DATABASE_URL'):
        raise ValueError('Missing MIGRATION_DATABASE_URL')
    smoke.validate_url(os.environ['SMOKE_URL'])
    deploy(api, service_id, image, lambda: migrate_image(image), smoke.main, rollback=rollback)
    with open(os.environ['GITHUB_OUTPUT'], 'a', encoding='utf-8') as destination:
        destination.write('service_url=' + os.environ['SMOKE_URL'].rstrip('/') + '\n')


if __name__ == '__main__':
    main()
